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

// cmd/seed 仅负责 RBAC 业务初始数据写入。
//
// schema 演进由 atlas migrate apply 处理，本命令前置依赖 schema 已存在。
//
// 行为：
//   - 默认：要求业务表为空，写入初始数据；表非空直接拒绝
//   - -reset=true：先清空业务表数据再写入（仅 dev/local，prod 永禁）
//
// 用 run() 包装让所有 defer 在 os.Exit 前必然执行。
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
	// 把 CLI flag 注入 viper，让 SeedServer 通过 conf 读取，
	// 与其他配置入口（yml / 环境变量）保持一致的取值通道。
	conf.Set("seed.reset", *reset)

	logger := log.NewLog(conf)
	seedServer, cleanup, err := wire.NewWire(conf, logger)
	if err != nil {
		logger.Error("依赖装配失败", zap.Error(err))
		return err
	}
	defer cleanup()

	// 直接驱动一次 Start/Stop，绕过 app.Run 的"常驻 + 等信号"语义。
	// 用 context.Background() 即可：种子任务无外部超时来源，调用方 Ctrl+C 直达 OS。
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
