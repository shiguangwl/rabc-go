//go:build wireinject
// +build wireinject

package wire

import (
	"rabc-go/internal/platform"
	"rabc-go/internal/server"
	"rabc-go/pkg/log"
	"rabc-go/pkg/sid"

	"github.com/google/wire"
	"github.com/spf13/viper"
)

var platformSet = wire.NewSet(
	platform.NewDB,
	platform.NewCasbinEnforcer,
)

var serverSet = wire.NewSet(
	server.NewSeedServer,
)

// NewWire 返回一次性种子任务，调用方负责显式驱动 Start/Stop 生命周期。
func NewWire(*viper.Viper, *log.Logger) (*server.SeedServer, func(), error) {
	panic(wire.Build(
		platformSet,
		serverSet,
		sid.NewSid,
	))
}
