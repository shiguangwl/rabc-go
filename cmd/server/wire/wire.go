//go:build wireinject
// +build wireinject

package wire

import (
	rbacapi "rabc-go/internal/admin/rbac/api"
	"rabc-go/internal/admin/rbac/menu"
	"rabc-go/internal/admin/rbac/permission"
	"rabc-go/internal/admin/rbac/casbinkit"
	"rabc-go/internal/admin/rbac/role"
	"rabc-go/internal/admin/rbac/user"
	"rabc-go/internal/auth"
	"rabc-go/internal/platform"
	"rabc-go/internal/server"
	"rabc-go/pkg/app"
	"rabc-go/pkg/jwt"
	"rabc-go/pkg/log"
	"rabc-go/pkg/server/http"

	"github.com/google/wire"
	"github.com/spf13/viper"
)

var platformSet = wire.NewSet(
	platform.NewDB,
	platform.NewCasbinEnforcer,
	platform.NewRedis,
)

var rbacSet = wire.NewSet(
	casbinkit.NewRBACMu,

	user.NewRepo,
	user.NewService,
	user.NewHandler,
	role.NewRepo,
	role.NewService,
	role.NewHandler,
	menu.NewRepo,
	menu.NewService,
	menu.NewHandler,
	rbacapi.NewRepo,
	rbacapi.NewService,
	rbacapi.NewHandler,
	permission.NewRepo,
	permission.NewService,
	permission.NewHandler,

	// 跨子域依赖反转：消费者侧接口绑定到实现者，避免下游子域反向依赖上游具体类型。
	wire.Bind(new(menu.PermissionReader), new(*permission.Repo)),
	wire.Bind(new(auth.UserLookup), new(*user.Repo)),
)

var authSet = wire.NewSet(
	auth.LoadConfig,
	auth.NewRepository,
	auth.NewService,
	auth.NewHandler,
	wire.Bind(new(user.AuthRevoker), new(*auth.Service)),
)

var serverSet = wire.NewSet(
	server.NewHTTPServer,
)

func newApp(
	logger *log.Logger,
	httpServer *http.Server,
) *app.App {
	return app.NewApp(
		app.WithServer(httpServer),
		app.WithName("demo-server"),
		app.WithLogger(logger),
	)
}

func NewWire(*viper.Viper, *log.Logger) (*app.App, func(), error) {
	panic(wire.Build(
		platformSet,
		rbacSet,
		authSet,
		serverSet,
		jwt.NewJwt,
		newApp,
	))
}
