package solana

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"solana-high-speed-indexer/internal/worker"

	"github.com/gagliardetto/solana-go/rpc/ws"
	"go.uber.org/zap"
)

// Config stores Solana websocket connection settings.
type Config struct {
	WSURL        string
	Transactions chan<- worker.Transaction
}

// LogListener handles websocket connection and log subscriptions.
type LogListener struct {
	cfg      Config
	logger   *zap.Logger
	client   *ws.Client
	sub      *ws.LogSubscription
	closeMux sync.Mutex
	closed   bool
}

func NewLogListener(cfg Config, logger *zap.Logger) (*LogListener, error) {
	if cfg.WSURL == "" {
		return nil, errors.New("ws url is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}

	return &LogListener{
		cfg:    cfg,
		logger: logger,
	}, nil
}

func (l *LogListener) Run(ctx context.Context) error {
	client, err := ws.Connect(ctx, l.cfg.WSURL)
	if err != nil {
		return fmt.Errorf("connect websocket: %w", err)
	}
	l.client = client

	sub, err := client.LogsSubscribe(
		ws.LogsSubscribeFilterAll,
		"",
	)
	if err != nil {
		return fmt.Errorf("subscribe logs: %w", err)
	}
	l.sub = sub

	l.logger.Info("connected to solana websocket and subscribed to logs", zap.String("url", l.cfg.WSURL))

	for {
		msg, err := sub.Recv(ctx)
		if err != nil {
			return fmt.Errorf("receive log: %w", err)
		}

		l.logger.Info("new transaction log",
			zap.Uint64("slot", msg.Context.Slot),
			zap.String("signature", msg.Value.Signature.String()),
			zap.Int("log_count", len(msg.Value.Logs)),
			zap.String("err", fmt.Sprint(msg.Value.Err)),
		)

		if l.cfg.Transactions != nil {
			tx := worker.Transaction{
				Signature: msg.Value.Signature.String(),
				Slot:      msg.Context.Slot,
				Timestamp: time.Now().UTC(),
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case l.cfg.Transactions <- tx:
			}
		}
	}
}

func (l *LogListener) Close(ctx context.Context) error {
	l.closeMux.Lock()
	defer l.closeMux.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true

	done := make(chan struct{})
	var closeErr error
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				closeErr = fmt.Errorf("panic during close: %v", r)
			}
		}()

		if l.sub != nil {
			l.sub.Unsubscribe()
		}
		if l.client != nil {
			l.client.Close()
		}
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("shutdown timeout: %w", ctx.Err())
	case <-done:
		return closeErr
	}
}
