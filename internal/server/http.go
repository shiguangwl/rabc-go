package server

import (
	nethttp "net/http"
	"regexp"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"rabc-go/api/apiv1"
	docs "rabc-go/docs/swagger"
	iamapi "rabc-go/internal/admin/iam/api"
	"rabc-go/internal/admin/iam/menu"
	"rabc-go/internal/admin/iam/permission"
	"rabc-go/internal/admin/iam/role"
	"rabc-go/internal/admin/iam/user"
	"rabc-go/internal/auth"
	"rabc-go/internal/middleware"
	"rabc-go/pkg/config"
	"rabc-go/pkg/jwt"
	"rabc-go/pkg/log"
	"rabc-go/pkg/server/http"
	"rabc-go/web"
)

// devOriginRe 限定 dev CORS 仅放通本机环回（localhost / 127.0.0.1 / [::1]）任意端口。
// 不再使用 AllowOriginFunc 一律 return true，避免开发机上其他本地服务（Storybook、
// 内部工具页面）借同主机域信任带 cookie 调本服务（CSRF 风险）。
// 加入 [::1] 是为了覆盖 OS 默认监听 IPv6 loopback 时浏览器走 [::1]:port 的来源。
var devOriginRe = regexp.MustCompile(`^https?://(?:localhost|127\.0\.0\.1|\[::1\])(?::\d+)?$`)

func NewHTTPServer(
	logger *log.Logger,
	conf *viper.Viper,
	jwtUtil *jwt.JWT,
	e *casbin.SyncedEnforcer,
	authHandler *auth.Handler,
	userHandler *user.Handler,
	roleHandler *role.Handler,
	menuHandler *menu.Handler,
	apiHandler *iamapi.Handler,
	permHandler *permission.Handler,
) *http.Server {
	if config.IsProd(conf) {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	s := http.NewServer(
		gin.New(),
		logger,
		http.WithServerHost(conf.GetString("http.host")),
		http.WithServerPort(conf.GetInt("http.port")),
	)

	// 非 prod 启用宽松 CORS，便于本地前端开发联调；
	// prod 由 Nginx/Ingress 反代统一处理 CORS，应用层不下发任何 CORS 头。
	//
	// AllowOriginFunc + AllowCredentials 组合：浏览器禁止 Access-Control-Allow-Origin=*
	// 与 Allow-Credentials=true 共存，因此用 OriginFunc 反射回请求 Origin，
	// 既保留宽松联调体验，又支持前端 withCredentials=true（cookie/JWT 透传）。
	if !config.IsProd(conf) {
		s.Use(cors.New(cors.Config{
			AllowOriginFunc:  devOriginRe.MatchString,
			AllowCredentials: true,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders: []string{
				"Origin", "Content-Type", "Accept",
				"Authorization", "X-Requested-With",
			},
			ExposeHeaders: []string{"Content-Length", "Content-Disposition"},
			MaxAge:        12 * time.Hour,
		}))
	}

	logRequestHeaders := conf.GetBool("log.request.headers.enabled")
	logRequestBody := conf.GetBool("log.request.body.enabled")
	logResponseBody := conf.GetBool("log.response.body.enabled")
	maxBodyBytes := conf.GetInt("log.body.max_bytes")
	s.Use(
		middleware.RequestLogMiddleware(logger, logRequestHeaders, logRequestBody, maxBodyBytes),
		middleware.ResponseLogMiddleware(logger, logResponseBody, maxBodyBytes),
		middleware.Recovery(),
	)

	frontendFS := static.EmbedFolder(web.Assets(), "dist")
	if frontendFS != nil {
		s.Use(static.Serve("/", frontendFS))
	}
	s.NoRoute(func(c *gin.Context) {
		if !isSPAFallback(c.Request) {
			apiv1.WriteResponse(c, apiv1.ErrNotFound, nil)
			return
		}
		indexPageData, err := web.Assets().ReadFile("dist/index.html")
		if err != nil {
			c.String(nethttp.StatusNotFound, "404 page not found")
			return
		}
		c.Data(nethttp.StatusOK, "text/html; charset=utf-8", indexPageData)
	})
	docs.SwaggerInfo.BasePath = "/"
	s.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerfiles.Handler,
		ginSwagger.DefaultModelsExpandDepth(-1),
		ginSwagger.PersistAuthorization(true),
	))

	v1 := s.Group("/v1")
	{
		// Auth 子系统：login / refresh / logout 不走 StrictAuth。
		// refresh 自验证 refresh_token；logout 即便 access 过期用户也能登出。
		noAuth := v1.Group("/")
		{
			noAuth.POST("/login", authHandler.Login)
			noAuth.POST("/auth/refresh", authHandler.Refresh)
			noAuth.POST("/auth/logout", authHandler.Logout)
		}

		strict := v1.Group("/").Use(middleware.StrictAuth(jwtUtil, logger), middleware.AuthMiddleware(e))
		{
			// 当前用户菜单（按权限过滤）
			strict.GET("/menus", menuHandler.GetMenus)

			// 菜单管理
			strict.GET("/admin/menus", menuHandler.GetAdminMenus)
			strict.POST("/admin/menu", menuHandler.MenuCreate)
			strict.PUT("/admin/menu", menuHandler.MenuUpdate)
			strict.DELETE("/admin/menu", menuHandler.MenuDelete)

			// 管理员账户
			strict.GET("/admin/users", userHandler.GetAdminUsers)
			strict.GET("/admin/user", userHandler.GetAdminUser)
			strict.PUT("/admin/user", userHandler.AdminUserUpdate)
			strict.POST("/admin/user", userHandler.AdminUserCreate)
			strict.DELETE("/admin/user", userHandler.AdminUserDelete)

			// 用户权限 / 会话
			strict.GET("/admin/user/permissions", permHandler.GetUserPermissions)
			strict.GET("/admin/user/sessions", authHandler.GetUserSessions)
			strict.DELETE("/admin/user/sessions", authHandler.RevokeUserSessions)
			strict.DELETE("/admin/user/session", authHandler.KickUserSession)

			// 角色权限
			strict.GET("/admin/role/permissions", permHandler.GetRolePermissions)
			strict.PUT("/admin/role/permission", permHandler.UpdateRolePermission)

			// 角色管理
			strict.GET("/admin/roles", roleHandler.GetRoles)
			strict.POST("/admin/role", roleHandler.RoleCreate)
			strict.PUT("/admin/role", roleHandler.RoleUpdate)
			strict.DELETE("/admin/role", roleHandler.RoleDelete)

			// API 资源管理
			strict.GET("/admin/apis", apiHandler.GetApis)
			strict.POST("/admin/api", apiHandler.APICreate)
			strict.PUT("/admin/api", apiHandler.APIUpdate)
			strict.DELETE("/admin/api", apiHandler.APIDelete)
		}
	}
	return s
}

func isSPAFallback(r *nethttp.Request) bool {
	if r.Method != nethttp.MethodGet && r.Method != nethttp.MethodHead {
		return false
	}
	path := r.URL.Path
	return path != "/v1" &&
		!strings.HasPrefix(path, "/v1/") &&
		path != "/swagger" &&
		!strings.HasPrefix(path, "/swagger/")
}
