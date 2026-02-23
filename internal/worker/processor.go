package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"quantlo/internal/repository"
	"github.com/nats-io/nats.go"
)

type TransactionWorker struct {
	repo     *repository.LedgerRepo
	natsConn *nats.Conn
}

func NewTransactionWorker(repo *repository.LedgerRepo, nc *nats.Conn) *TransactionWorker {
	return &TransactionWorker{
		repo:     repo,
		natsConn: nc,
	}
}

func (w *TransactionWorker) Run(ctx context.Context) error {
	// QueueSubscribe ensures that messages are processed in parallel, 
	// but each message will be received by only one worker in the group.
	sub, err := w.natsConn.QueueSubscribe("transactions.created", "worker_group", func(m *nats.Msg) {
		var event repository.SpendEvent
		if err := json.Unmarshal(m.Data, &event); err != nil {
			slog.Error("failed to unmarshal nats message", "error", err)
			return
		}

		// It will handle the transaction, idempotency check, and balance update.
		if err := w.repo.SyncTransactionWithBalance(ctx, event); err != nil {
			slog.Error("failed to sync transaction with postgres", 
				"account_id", event.AccountID, 
				"key", event.IdempotencyKey, 
				"error", err,
			)
			return
		}

		slog.Info("transaction synced successfully", 
			"account_id", event.AccountID, 
			"key", event.IdempotencyKey,
		)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to NATS: %w", err)
	}

	slog.Info("Transaction worker is running")

	// Wait for shutdown signal from main.go
	<-ctx.Done()

	slog.Info("Worker received shutdown signal, draining subscription...")
	
	// Close subscription gracefully, waiting for current processing to complete
	return sub.Drain()
}