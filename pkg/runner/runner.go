package runner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type Server interface {
	Serve(listener net.Listener) error
	Shutdown(ctx context.Context) error
}

func RunServer(
	ctx context.Context,
	server Server,
	port string,
	errChan chan<- error,
	wgr *sync.WaitGroup,
) error {
	return runServer(ctx, server, port, errChan, wgr, net.Listen)
}

func runServer(
	ctx context.Context,
	server Server,
	port string,
	errChan chan<- error,
	wgr *sync.WaitGroup,
	listen func(string, string) (net.Listener, error),
) error {
	listener, err := listen("tcp4", port)
	if err != nil {
		return fmt.Errorf("can't listen tcp port %s: %w", port, err)
	}

	wgr.Add(1)

	go func() {
		defer wgr.Done()

		err := server.Serve(listener)
		// http.ErrServerClosed - это нормальная ситуация при graceful shutdown
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("can't start http server: %w", err)
		}
	}()

	wgr.Add(1)

	go func() {
		defer wgr.Done()

		<-ctx.Done()

		const timeout = time.Second * 5

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			errChan <- fmt.Errorf("can't shutdown http server: %w", err)
		}
	}()

	return nil
}
