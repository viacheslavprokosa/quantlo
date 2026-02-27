package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"quantlo/internal/model"
	"quantlo/internal/service"

	"github.com/nats-io/nats.go"
)

// TransactionWorker listens on the "transactions.created" NATS topic
// and syncs spend events to the PostgreSQL transactions table.
type TransactionWorker struct {
	svc      service.LedgerService
	natsConn *nats.Conn
}

func NewTransactionWorker(svc service.LedgerService, nc *nats.Conn) *TransactionWorker {
	return &TransactionWorker{
		svc:      svc,
		natsConn: nc,
	}
}

// Run subscribes to "transactions.created" and blocks until ctx is cancelled.
func (w *TransactionWorker) Run(ctx context.Context) error {
	// QueueSubscribe ensures that messages are processed in parallel,
	// but each message will be received by only one worker in the group.
	sub, err := w.natsConn.QueueSubscribe("transactions.created", "worker_group", func(m *nats.Msg) {
		var event model.SpendEvent
		if err := json.Unmarshal(m.Data, &event); err != nil {
			slog.Error("worker: failed to unmarshal nats message", "error", err)
			return
		}

		// Handle the transaction: idempotency check + balance update in Postgres.
		if err := w.svc.SyncTransactionWithBalance(ctx, event); err != nil {
			slog.Error("worker: failed to sync transaction with postgres",
				"account_id", event.AccountID,
				"key", event.IdempotencyKey,
				"error", err,
			)
			return
		}

		slog.Info("worker: transaction synced successfully",
			"account_id", event.AccountID,
			"key", event.IdempotencyKey,
		)
	})

	if err != nil {
		return fmt.Errorf("worker: failed to subscribe to NATS: %w", err)
	}

	slog.Info("Transaction worker is running")

	// Wait for shutdown signal.
	<-ctx.Done()

	slog.Info("Worker received shutdown signal, draining subscription...")
	// Close subscription gracefully, waiting for current processing to complete.
	return sub.Drain()
}

// Start implements the infrastructure.Server interface.
func (w *TransactionWorker) Start(ctx context.Context) error {
	return w.Run(ctx)
}

// Stop implements the infrastructure.Server interface (no-op, shutdown is via ctx).
func (w *TransactionWorker) Stop(ctx context.Context) error {
	return nil
}
