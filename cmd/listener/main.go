package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"solana-high-speed-indexer/internal/solana"

	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	listener, err := solana.NewLogListener(solana.Config{
		WSURL: "wss://api.devnet.solana.com/",
	}, logger)
	if err != nil {
		logger.Fatal("failed to create listener", zap.Error(err))
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- listener.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("listener exited with error", zap.Error(err))
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := listener.Close(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	} else {
		logger.Info("listener stopped gracefully")
	}
}
