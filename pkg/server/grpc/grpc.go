package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"

	"rabc-go/pkg/log"
)

const defaultShutdownTimeout = 5 * time.Second

type Server struct {
	*grpc.Server
	host   string
	port   int
	logger *log.Logger
}

type Option func(s *Server)

func NewServer(logger *log.Logger, opts ...Option) *Server {
	s := &Server{
		Server: grpc.NewServer(),
		logger: logger,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithServerHost(host string) Option {
	return func(s *Server) {
		s.host = host
	}
}

func WithServerPort(port int) Option {
	return func(s *Server) {
		s.port = port
	}
}

func (s *Server) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 gRPC 地址 %s 失败: %w", addr, err)
	}
	if err = s.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("启动 gRPC 服务 %s 失败: %w", addr, err)
	}
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultShutdownTimeout)
		defer cancel()
	}

	stopped := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-ctx.Done():
		s.Server.Stop()
		<-stopped
		return fmt.Errorf("优雅停止 gRPC 服务超时: %w", ctx.Err())
	}

	s.logger.Info("gRPC 服务已退出")

	return nil
}
