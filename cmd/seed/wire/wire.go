//go:build wireinject
// +build wireinject

package wire

import (
	"rabc-go/internal/repository"
	"rabc-go/internal/server"
	"rabc-go/pkg/log"
	"rabc-go/pkg/sid"

	"github.com/google/wire"
	"github.com/spf13/viper"
)

var repositorySet = wire.NewSet(
	repository.NewDB,
	repository.NewRepository,
	repository.NewCasbinEnforcer,
)
var serverSet = wire.NewSet(
	server.NewSeedServer,
)

// NewWire 直接返回 *SeedServer，不再包成 app.App。
// 原因：app.Run 是常驻服务模型（spawn goroutine + 阻塞等信号），与 cmd/seed
// "写完即退出"的一次性 CLI 语义不匹配，外面套 App 会让 make seed 永远挂起。
func NewWire(*viper.Viper, *log.Logger) (*server.SeedServer, func(), error) {
	panic(wire.Build(
		repositorySet,
		serverSet,
		sid.NewSid,
	))
}
