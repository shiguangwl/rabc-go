package app

import (
	"context"
	"errors"
	"fmt"
	stdlog "log"
	"os/signal"
	"rabc-go/pkg/server"
	"syscall"
	"time"
)

type App struct {
	name    string
	servers []server.Server
}

type Option func(a *App)

func NewApp(opts ...Option) *App {
	a := &App{}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func WithServer(servers ...server.Server) Option {
	return func(a *App) {
		a.servers = servers
	}
}

func WithName(name string) Option {
	return func(a *App) {
		a.name = name
	}
}

func (a *App) Run(ctx context.Context) error {
	if len(a.servers) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	signalCtx, stopSignals := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	runCtx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	errCh := make(chan error, len(a.servers))
	doneCh := make(chan struct{}, len(a.servers))
	for _, srv := range a.servers {
		go func(srv server.Server) {
			defer func() { doneCh <- struct{}{} }()
			if err := srv.Start(runCtx); err != nil {
				errCh <- fmt.Errorf("start %T: %w", srv, err)
				return
			}
			errCh <- nil
		}(srv)
	}

	var runErr error
	select {
	case err := <-errCh:
		runErr = err
		if runErr == nil {
			stdlog.Println("Server exited")
		}
	case <-runCtx.Done():
		if ctx.Err() != nil {
			runErr = ctx.Err()
		} else {
			stdlog.Println("Received termination signal")
		}
	}
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	var stopErr error
	for _, srv := range a.servers {
		if err := srv.Stop(shutdownCtx); err != nil {
			stopErr = errors.Join(stopErr, fmt.Errorf("stop %T: %w", srv, err))
		}
	}

	for range a.servers {
		select {
		case <-doneCh:
		case <-shutdownCtx.Done():
			stopErr = errors.Join(stopErr, fmt.Errorf("wait servers stopped: %w", shutdownCtx.Err()))
			return errors.Join(runErr, stopErr)
		}
	}

	return errors.Join(runErr, stopErr)
}
