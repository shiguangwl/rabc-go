package main

import (
	"context"
	"flag"
	"os"

	"go.uber.org/zap"

	"rabc-go/cmd/seed/wire"
	"rabc-go/pkg/config"
	"rabc-go/pkg/log"
)

// 调用前必须先完成 schema migration（atlas migrate apply）；默认要求业务表为空，
// -reset 仅允许在 local 环境清空业务表后重新写入。
func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	envConf := flag.String("conf", "config/local.yml", "config path, eg: -conf ./config/local.yml")
	reset := flag.Bool("reset", false,
		"truncate RBAC tables before seeding (forbidden in prod)")
	flag.Parse()

	conf := config.NewConfig(*envConf)
	conf.Set("seed.reset", *reset)

	logger := log.NewLog(conf)
	seedServer, cleanup, err := wire.NewWire(conf, logger)
	if err != nil {
		logger.Error("依赖装配失败", zap.Error(err))
		return err
	}
	defer cleanup()

	ctx := context.Background()
	if err := seedServer.Start(ctx); err != nil {
		logger.Error("种子数据写入失败", zap.Error(err), zap.Bool("reset", *reset))
		_ = seedServer.Stop(ctx)
		return err
	}
	if err := seedServer.Stop(ctx); err != nil {
		logger.Warn("种子任务停止失败", zap.Error(err))
	}
	logger.Info("种子数据写入完成", zap.Bool("reset", *reset))
	return nil
}
