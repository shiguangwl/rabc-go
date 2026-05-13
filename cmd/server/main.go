package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"rabc-go/pkg/log"

	"rabc-go/cmd/server/wire"
	"rabc-go/pkg/config"

	"go.uber.org/zap"
)

// @title           RABC-Go Admin API
// @version         1.0.0
// @description     RBAC 管理后台接口，覆盖登录认证、菜单/API 权限、角色授权与管理员管理。
// @host      localhost:8000
// @securityDefinitions.apiKey Bearer
// @in header
// @name Authorization
func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	envConf := flag.String("conf", "config/local.yml", "config path, eg: -conf ./config/local.yml")
	flag.Parse()
	conf := config.NewConfig(*envConf)

	logger := log.NewLog(conf)

	app, cleanup, err := wire.NewWire(conf, logger)
	if err != nil {
		logger.Error("依赖装配失败", zap.Error(err))
		return err
	}
	defer cleanup()
	logger.Info("服务已启动", zap.String("host", fmt.Sprintf("http://%s:%d", conf.GetString("http.host"), conf.GetInt("http.port"))))
	logger.Info("接口文档已就绪", zap.String("addr", fmt.Sprintf("http://%s:%d/swagger/index.html", conf.GetString("http.host"), conf.GetInt("http.port"))))
	if err = app.Run(context.Background()); err != nil {
		logger.Error("服务异常退出", zap.Error(err))
		return err
	}
	return nil
}
