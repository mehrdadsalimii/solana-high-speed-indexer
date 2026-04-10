package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Transaction is a normalized unit of work produced by the Solana listener.
type Transaction struct {
	Signature string
	Slot      uint64
	Timestamp time.Time
}

// Handler processes a single transaction.
type Handler func(ctx context.Context, tx Transaction) error

// Config defines worker pool settings.
type Config struct {
	Workers int
	Input   <-chan Transaction
	Handler Handler
}

// Pool consumes transactions from a channel and processes them concurrently.
type Pool struct {
	cfg    Config
	logger *zap.Logger
	wg     sync.WaitGroup
}

func NewPool(cfg Config, logger *zap.Logger) (*Pool, error) {
	if cfg.Workers <= 0 {
		return nil, errors.New("workers must be greater than zero")
	}
	if cfg.Input == nil {
		return nil, errors.New("input channel is required")
	}
	if cfg.Handler == nil {
		return nil, errors.New("handler is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}

	return &Pool{cfg: cfg, logger: logger}, nil
}

// Run starts all workers.
func (p *Pool) Run(ctx context.Context) {
	for i := 0; i < p.cfg.Workers; i++ {
		workerID := i + 1
		p.wg.Add(1)
		go p.workerLoop(ctx, workerID)
	}
	p.logger.Info("worker pool started", zap.Int("workers", p.cfg.Workers))
}

func (p *Pool) workerLoop(ctx context.Context, workerID int) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			p.logger.Debug("worker stopped by context", zap.Int("worker_id", workerID))
			return
		case tx, ok := <-p.cfg.Input:
			if !ok {
				p.logger.Debug("worker stopped because input channel closed", zap.Int("worker_id", workerID))
				return
			}

			if err := p.cfg.Handler(ctx, tx); err != nil {
				p.logger.Error("failed to process transaction",
					zap.Int("worker_id", workerID),
					zap.String("signature", tx.Signature),
					zap.Uint64("slot", tx.Slot),
					zap.Error(err),
				)
			}
		}
	}
}

// Wait waits for all workers to exit.
func (p *Pool) Wait() {
	p.wg.Wait()
}

// WaitWithContext waits for all workers to exit or the context to expire.
func (p *Pool) WaitWithContext(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.wg.Wait()
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("waiting workers: %w", ctx.Err())
	case <-done:
		return nil
	}
}
