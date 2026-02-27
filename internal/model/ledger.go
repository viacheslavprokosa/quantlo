package model

import "time"

type SpendRequest struct {
	AccountID      string `json:"account_id"`
	ResourceType   string `json:"resource_type"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

type RechargeRequest struct {
	AccountID    string `json:"account_id"`
	ResourceType string `json:"resource_type"`
	Amount       int64  `json:"amount"`
}

type SpendResult struct {
	NewBalance int64  `json:"new_balance"`
	Status     string `json:"status"`
}

type SpendEvent struct {
	AccountID      string    `json:"account_id"`
	ResourceType   string    `json:"resource_type"`
	Amount         int64     `json:"amount"`
	IdempotencyKey string    `json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
}