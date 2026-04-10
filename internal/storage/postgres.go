package storage

import (
	"context"
	"errors"
	"fmt"

	"solana-high-speed-indexer/internal/worker"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	if dsn == "" {
		return nil, errors.New("postgres dsn is required")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) SaveTransaction(ctx context.Context, tx worker.Transaction) error {
	const query = `
		INSERT INTO transactions (signature, slot, observed_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (signature) DO NOTHING
	`

	if _, err := s.pool.Exec(ctx, query, tx.Signature, tx.Slot, tx.Timestamp); err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}
	return nil
}

func (s *PostgresStore) Close(_ context.Context) error {
	if s.pool != nil {
		s.pool.Close()
	}
	return nil
}
