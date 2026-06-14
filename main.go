package main

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"go.sakib.dev/le/logger"
	"go.sakib.dev/le/pkg/cfg"
	"go.sakib.dev/le/pkg/server"
	"go.sakib.dev/le/pkg/tui"
)

func main() {

	slog.SetDefault(slog.New(logger.NewHandler()))

	config, err := cfg.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("Loaded", "config", config)

	srvr, err := server.NewServer(config)

	if err != nil {
		slog.Error("Failed to start server:", "error", err)
		os.Exit(1)
	}

	wg := sync.WaitGroup{}

	errCh := make(chan error, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(2)
	go func() {
		defer wg.Done()
		err = tui.Start(ctx, srvr)
		if err != nil {
			slog.Error("Failed to start TUI", "error", err)
			errCh <- err
		}
		cancel()
	}()

	go func() {
		defer wg.Done()
		if err := srvr.Start(); err != nil {
			slog.Error("Failed to start server", "error", err)
			errCh <- err
		}
		cancel()
	}()

	exitCode := 0

	select {
	case <-errCh:
		exitCode = 1
	case <-ctx.Done():
	}

	slog.Info("Exit code", "code", exitCode)

	wg.Wait()

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
