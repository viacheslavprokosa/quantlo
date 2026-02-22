package repository

import "time"

type SpendEvent struct {
	AccountID      string    `json:"account_id"`
	ResourceType   string    `json:"resource_type"`
	Amount         int64     `json:"amount"`
	IdempotencyKey string    `json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
}