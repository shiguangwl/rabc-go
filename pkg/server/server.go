package server

import (
	"context"
	"net/url"
)

// Server 是 App 编排的最小生命周期单元：Start 接受根 ctx 启动，
// Stop 在收到关停信号时让实现自行完成 graceful shutdown。
type Server interface {
	Start(context.Context) error
	Stop(context.Context) error
}

// Endpointer 让具体 Server 实现可在 Start 之外按需上报自己的访问地址，
// 供服务发现 / 健康检查 / 单元测试在不依赖固定端口的前提下读取。
type Endpointer interface {
	Endpoint() (*url.URL, error)
}
