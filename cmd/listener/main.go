package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"solana-high-speed-indexer/internal/solana"
	"solana-high-speed-indexer/internal/storage"
	"solana-high-speed-indexer/internal/worker"

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

	postgresDSN := getEnv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/solana_indexer?sslmode=disable")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPassword := os.Getenv("REDIS_PASSWORD")

	postgresStore, err := storage.NewPostgresStore(ctx, postgresDSN)
	if err != nil {
		logger.Fatal("failed to create postgres store", zap.Error(err))
	}

	redisStore, err := storage.NewRedisStore(ctx, storage.RedisConfig{
		Addr:       redisAddr,
		Password:   redisPassword,
		ListKey:    "transactions:latest",
		MaxEntries: 1000,
	})
	if err != nil {
		_ = postgresStore.Close(context.Background())
		logger.Fatal("failed to create redis store", zap.Error(err))
	}

	store, err := storage.NewMultiStore(postgresStore, redisStore)
	if err != nil {
		_ = redisStore.Close(context.Background())
		_ = postgresStore.Close(context.Background())
		logger.Fatal("failed to create storage layer", zap.Error(err))
	}

	txCh := make(chan worker.Transaction, 1024)
	const workerCount = 4

	pool, err := worker.NewPool(worker.Config{
		Workers: workerCount,
		Input:   txCh,
		Handler: func(ctx context.Context, tx worker.Transaction) error {
			logger.Info("processing transaction",
				zap.String("signature", tx.Signature),
				zap.Uint64("slot", tx.Slot),
				zap.Time("timestamp", tx.Timestamp),
			)

			if err := store.SaveTransaction(ctx, tx); err != nil {
				return err
			}
			return nil
		},
	}, logger)
	if err != nil {
		_ = store.Close(context.Background())
		logger.Fatal("failed to create worker pool", zap.Error(err))
	}
	pool.Run(ctx)

	listener, err := solana.NewLogListener(solana.Config{
		WSURL:        "wss://api.devnet.solana.com/",
		Transactions: txCh,
	}, logger)
	if err != nil {
		logger.Fatal("failed to create listener", zap.Error(err))
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- listener.Run(ctx)
	}()

	listenerDone := false
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		listenerDone = true
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("listener exited with error", zap.Error(err))
		}
	}
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := listener.Close(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	} else {
		logger.Info("listener stopped gracefully")
	}

	if !listenerDone {
		if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("listener exited with error", zap.Error(err))
		}
	}

	close(txCh)
	if err := pool.WaitWithContext(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("worker pool shutdown failed", zap.Error(err))
	} else {
		logger.Info("worker pool stopped gracefully")
	}

	if err := store.Close(shutdownCtx); err != nil {
		logger.Error("storage shutdown failed", zap.Error(err))
	} else {
		logger.Info("storage stopped gracefully")
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
