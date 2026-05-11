//go:build wireinject
// +build wireinject

package wire

import (
	"rabc-go/internal/repository"
	"rabc-go/internal/server"
	"rabc-go/internal/task"
	"rabc-go/pkg/app"
	"rabc-go/pkg/log"
	"rabc-go/pkg/sid"

	"github.com/google/wire"
	"github.com/spf13/viper"
)

var repositorySet = wire.NewSet(
	repository.NewDB,
	repository.NewRepository,
	repository.NewTransaction,
	repository.NewUserRepository,
	repository.NewCasbinEnforcer,
)

var taskSet = wire.NewSet(
	task.NewTask,
	task.NewUserTask,
)
var serverSet = wire.NewSet(
	server.NewTaskServer,
)

// build App
func newApp(
	task *server.TaskServer,
) *app.App {
	return app.NewApp(
		app.WithServer(task),
		app.WithName("demo-task"),
	)
}

func NewWire(*viper.Viper, *log.Logger) (*app.App, func(), error) {
	panic(wire.Build(
		repositorySet,
		taskSet,
		serverSet,
		newApp,
		sid.NewSid,
	))
}
