package main

import (
	"context"
	"flag"
	"os"
	"rabc-go/cmd/task/wire"
	"rabc-go/pkg/config"
	"rabc-go/pkg/log"

	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	var envConf = flag.String("conf", "config/local.yml", "config path, eg: -conf ./config/local.yml")
	flag.Parse()
	conf := config.NewConfig(*envConf)

	logger := log.NewLog(conf)
	logger.Info("任务服务已启动")
	app, cleanup, err := wire.NewWire(conf, logger)
	if err != nil {
		logger.Error("依赖装配失败", zap.Error(err))
		return err
	}
	defer cleanup()
	if err = app.Run(context.Background()); err != nil {
		logger.Error("任务服务异常退出", zap.Error(err))
		return err
	}
	return nil
}
