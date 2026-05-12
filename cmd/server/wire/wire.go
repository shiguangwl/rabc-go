//go:build wireinject
// +build wireinject

package wire

import (
	"rabc-go/internal/auth"
	"rabc-go/internal/handler"
	"rabc-go/internal/repository"
	"rabc-go/internal/server"
	"rabc-go/internal/service"
	"rabc-go/pkg/app"
	"rabc-go/pkg/jwt"
	"rabc-go/pkg/log"
	"rabc-go/pkg/server/http"

	"github.com/google/wire"
	"github.com/spf13/viper"
)

var repositorySet = wire.NewSet(
	repository.NewDB,
	repository.NewRepository,
	repository.NewTransaction,
	repository.NewCasbinEnforcer,
	repository.NewAdminRepository,
	repository.NewRedis,
	repository.NewAuthRepository,
)

var serviceSet = wire.NewSet(
	service.NewService,
	service.NewAdminService,
	service.NewAuthService,
	auth.LoadAuthConfig,
)

var handlerSet = wire.NewSet(
	handler.NewHandler,
	handler.NewAdminHandler,
	handler.NewAuthHandler,
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
		repositorySet,
		serviceSet,
		handlerSet,
		serverSet,
		jwt.NewJwt,
		newApp,
	))
}
