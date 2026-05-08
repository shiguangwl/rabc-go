//go:build wireinject
// +build wireinject

package wire

import (
	"github.com/google/wire"
	"github.com/spf13/viper"
	"nunu-layout-admin/internal/repository"
	"nunu-layout-admin/internal/server"
	"nunu-layout-admin/pkg/app"
	"nunu-layout-admin/pkg/log"
	"nunu-layout-admin/pkg/sid"
)

var repositorySet = wire.NewSet(
	repository.NewDB,
	//repository.NewRedis,
	repository.NewRepository,
	repository.NewCasbinEnforcer,
)
var serverSet = wire.NewSet(
	server.NewMigrateServer,
)

// build App
func newApp(
	migrateServer *server.MigrateServer,
) *app.App {
	return app.NewApp(
		app.WithServer(migrateServer),
		app.WithName("demo-migrate"),
	)
}

func NewWire(*viper.Viper, *log.Logger) (*app.App, func(), error) {
	panic(wire.Build(
		repositorySet,
		serverSet,
		sid.NewSid,
		newApp,
	))
}
