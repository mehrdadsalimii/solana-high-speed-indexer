package storage

import (
	"context"
	"errors"
	"fmt"

	"solana-high-speed-indexer/internal/worker"
)

// TransactionStore defines the storage contract for processed transactions.
type TransactionStore interface {
	SaveTransaction(ctx context.Context, tx worker.Transaction) error
}

// CloserStore is a store that can be closed gracefully.
type CloserStore interface {
	TransactionStore
	Close(ctx context.Context) error
}

// MultiStore writes transactions to multiple backends.
type MultiStore struct {
	stores []TransactionStore
}

func NewMultiStore(stores ...TransactionStore) (*MultiStore, error) {
	filtered := make([]TransactionStore, 0, len(stores))
	for _, store := range stores {
		if store != nil {
			filtered = append(filtered, store)
		}
	}
	if len(filtered) == 0 {
		return nil, errors.New("at least one store is required")
	}

	return &MultiStore{stores: filtered}, nil
}

func (m *MultiStore) SaveTransaction(ctx context.Context, tx worker.Transaction) error {
	for _, store := range m.stores {
		if err := store.SaveTransaction(ctx, tx); err != nil {
			return err
		}
	}
	return nil
}

func (m *MultiStore) Close(ctx context.Context) error {
	for _, store := range m.stores {
		closer, ok := store.(CloserStore)
		if !ok {
			continue
		}
		if err := closer.Close(ctx); err != nil {
			return fmt.Errorf("close store: %w", err)
		}
	}
	return nil
}
