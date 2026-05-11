package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeServer struct {
	start func(context.Context) error
	stop  func(context.Context) error
}

func (s fakeServer) Start(ctx context.Context) error { return s.start(ctx) }
func (s fakeServer) Stop(ctx context.Context) error  { return s.stop(ctx) }

func TestRunReturnsStartErrorAndStopsServers(t *testing.T) {
	startErr := errors.New("listen failed")
	var stopped atomic.Bool
	a := NewApp(WithServer(fakeServer{
		start: func(context.Context) error {
			return startErr
		},
		stop: func(context.Context) error {
			stopped.Store(true)
			return nil
		},
	}))

	err := a.Run(context.Background())
	if !errors.Is(err, startErr) {
		t.Fatalf("Run() error = %v, want %v", err, startErr)
	}
	if !stopped.Load() {
		t.Fatal("Run() did not stop server after start error")
	}
}

func TestRunReturnsWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	var stopped atomic.Bool
	a := NewApp(WithServer(fakeServer{
		start: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
		stop: func(context.Context) error {
			stopped.Store(true)
			return nil
		},
	}))

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.Run(ctx)
	}()
	<-started
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
	if !stopped.Load() {
		t.Fatal("Run() did not stop server after context cancellation")
	}
}

func TestRunStopsBlockingServerWhenAnotherServerExits(t *testing.T) {
	var stopped atomic.Bool
	a := NewApp(WithServer(
		fakeServer{
			start: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
			stop: func(context.Context) error {
				stopped.Store(true)
				return nil
			},
		},
		fakeServer{
			start: func(context.Context) error {
				return nil
			},
			stop: func(context.Context) error {
				return nil
			},
		},
	))

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.Run(context.Background())
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after one server exited")
	}
	if !stopped.Load() {
		t.Fatal("Run() did not stop blocking server")
	}
}
