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

	v1 "rabc-go/api/v1"
	swagger "rabc-go/docs/swagger"
	"rabc-go/internal/handler"
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
	jwt *jwt.JWT,
	e *casbin.SyncedEnforcer,
	adminHandler *handler.AdminHandler,
	authHandler *handler.AuthHandler,
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
		gin.Recovery(),
	)

	s.Use(static.Serve("/", static.EmbedFolder(web.Assets(), "dist")))
	s.NoRoute(func(c *gin.Context) {
		if !isSPAFallback(c.Request) {
			v1.WriteResponse(c, v1.ErrNotFound, nil)
			return
		}
		indexPageData, err := web.Assets().ReadFile("dist/index.html")
		if err != nil {
			c.String(nethttp.StatusNotFound, "404 page not found")
			return
		}
		c.Data(nethttp.StatusOK, "text/html; charset=utf-8", indexPageData)
	})
	swagger.SwaggerInfo.BasePath = "/"
	s.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerfiles.Handler,
		ginSwagger.DefaultModelsExpandDepth(-1),
		ginSwagger.PersistAuthorization(true),
	))

	v1Group := s.Group("/v1")
	{
		noAuthRouter := v1Group.Group("/")
		{
			noAuthRouter.POST("/login", adminHandler.Login)
			// Auth 子系统：refresh / logout 不走 StrictAuth——
			// refresh 自验证 refresh_token；logout 即便 access 过期用户也能登出。
			noAuthRouter.POST("/auth/refresh", authHandler.Refresh)
			noAuthRouter.POST("/auth/logout", authHandler.Logout)
		}

		strictAuthRouter := v1Group.Group("/").Use(middleware.StrictAuth(jwt, logger), middleware.AuthMiddleware(e))
		{
			strictAuthRouter.GET("/menus", adminHandler.GetMenus)
			strictAuthRouter.GET("/admin/menus", adminHandler.GetAdminMenus)
			strictAuthRouter.POST("/admin/menu", adminHandler.MenuCreate)
			strictAuthRouter.PUT("/admin/menu", adminHandler.MenuUpdate)
			strictAuthRouter.DELETE("/admin/menu", adminHandler.MenuDelete)

			strictAuthRouter.GET("/admin/users", adminHandler.GetAdminUsers)
			strictAuthRouter.GET("/admin/user", adminHandler.GetAdminUser)
			strictAuthRouter.PUT("/admin/user", adminHandler.AdminUserUpdate)
			strictAuthRouter.POST("/admin/user", adminHandler.AdminUserCreate)
			strictAuthRouter.DELETE("/admin/user", adminHandler.AdminUserDelete)
			strictAuthRouter.GET("/admin/user/permissions", adminHandler.GetUserPermissions)
			strictAuthRouter.GET("/admin/user/sessions", adminHandler.GetUserSessions)
			strictAuthRouter.DELETE("/admin/user/sessions", adminHandler.RevokeUserSessions)
			strictAuthRouter.DELETE("/admin/user/session", adminHandler.KickUserSession)
			strictAuthRouter.GET("/admin/role/permissions", adminHandler.GetRolePermissions)
			strictAuthRouter.PUT("/admin/role/permission", adminHandler.UpdateRolePermission)
			strictAuthRouter.GET("/admin/roles", adminHandler.GetRoles)
			strictAuthRouter.POST("/admin/role", adminHandler.RoleCreate)
			strictAuthRouter.PUT("/admin/role", adminHandler.RoleUpdate)
			strictAuthRouter.DELETE("/admin/role", adminHandler.RoleDelete)

			strictAuthRouter.GET("/admin/apis", adminHandler.GetApis)
			strictAuthRouter.POST("/admin/api", adminHandler.ApiCreate)
			strictAuthRouter.PUT("/admin/api", adminHandler.ApiUpdate)
			strictAuthRouter.DELETE("/admin/api", adminHandler.ApiDelete)

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
