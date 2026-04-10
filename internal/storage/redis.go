package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"solana-high-speed-indexer/internal/worker"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Addr       string
	Password   string
	DB         int
	ListKey    string
	MaxEntries int64
}

type RedisStore struct {
	client     *redis.Client
	listKey    string
	maxEntries int64
}

func NewRedisStore(ctx context.Context, cfg RedisConfig) (*RedisStore, error) {
	if cfg.Addr == "" {
		return nil, errors.New("redis address is required")
	}
	if cfg.ListKey == "" {
		cfg.ListKey = "transactions:latest"
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 500
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &RedisStore{
		client:     client,
		listKey:    cfg.ListKey,
		maxEntries: cfg.MaxEntries,
	}, nil
}

func (s *RedisStore) SaveTransaction(ctx context.Context, tx worker.Transaction) error {
	payload, err := json.Marshal(struct {
		Signature string    `json:"signature"`
		Slot      uint64    `json:"slot"`
		Timestamp time.Time `json:"timestamp"`
	}{
		Signature: tx.Signature,
		Slot:      tx.Slot,
		Timestamp: tx.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("marshal transaction payload: %w", err)
	}

	pipe := s.client.TxPipeline()
	pipe.LPush(ctx, s.listKey, payload)
	pipe.LTrim(ctx, s.listKey, 0, s.maxEntries-1)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache transaction: %w", err)
	}

	return nil
}

func (s *RedisStore) Close(_ context.Context) error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
