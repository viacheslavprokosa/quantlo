package worker

import (
	"context"
	"encoding/json"
	"log"
	
	"quantlo/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type TransactionWorker struct {
	dbPool   *pgxpool.Pool
	natsConn *nats.Conn
}

func NewTransactionWorker(db *pgxpool.Pool, nc *nats.Conn) *TransactionWorker {
	return &TransactionWorker{
		dbPool:   db,
		natsConn: nc,
	}
}

func (w *TransactionWorker) Start(ctx context.Context) error {
	// Using QueueSubscribe. 
	// This is important: if we run 10 copies of our API, 
	// only ONE worker will receive the message (to avoid duplicates).
	_, err := w.natsConn.QueueSubscribe("transactions.created", "worker_group", func(m *nats.Msg) {
		var event repository.SpendEvent
		if err := json.Unmarshal(m.Data, &event); err != nil {
			log.Printf("Error deserializing: %v", err)
			return
		}
		query := `
			INSERT INTO transactions (account_id, resource_type, amount, idempotency_key, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (idempotency_key) DO NOTHING`

		_, err := w.dbPool.Exec(ctx, query, 
			event.AccountID, 
			event.ResourceType, 
			event.Amount, 
			event.IdempotencyKey, 
			event.CreatedAt,
		)

		if err != nil {
			log.Printf("Error saving transaction to DB: %v", err)
			return
		}

		log.Printf("Transaction saved: %s (id: %s)", event.AccountID, event.IdempotencyKey)
	})

	return err
}