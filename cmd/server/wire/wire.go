//go:build wireinject
// +build wireinject

package wire

import (
	"rabc-go/internal/admin/config"
	iamapi "rabc-go/internal/admin/iam/api"
	"rabc-go/internal/admin/iam/casbinkit"
	"rabc-go/internal/admin/iam/menu"
	"rabc-go/internal/admin/iam/permission"
	"rabc-go/internal/admin/iam/role"
	"rabc-go/internal/admin/iam/user"
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

var iamSet = wire.NewSet(
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
	iamapi.NewRepo,
	iamapi.NewService,
	iamapi.NewHandler,
	permission.NewRepo,
	permission.NewService,
	permission.NewHandler,

	// 跨子域依赖反转：消费者侧接口绑定到实现者，避免下游子域反向依赖上游具体类型。
	wire.Bind(new(menu.PermissionReader), new(*permission.Repo)),
	wire.Bind(new(auth.UserLookup), new(*user.Repo)),
)

var configSet = wire.NewSet(
	config.NewRepo,
	config.NewService,
	config.NewHandler,
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
		iamSet,
		configSet,
		authSet,
		serverSet,
		jwt.NewJwt,
		newApp,
	))
}
