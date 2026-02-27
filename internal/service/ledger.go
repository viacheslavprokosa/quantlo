package service

import (
	"context"

	"quantlo/internal/model"
)

// LedgerService defines the business operations for the ledger.
// All transport layers (HTTP, gRPC, NATS) depend on this interface, not on the concrete repo.
type LedgerService interface {
	Spend(ctx context.Context, req model.SpendRequest) (*model.SpendResult, error)
	Recharge(ctx context.Context, req model.RechargeRequest) error
	GetBalance(ctx context.Context, accountID, resourceType string) (int64, error)
	CreateAccount(ctx context.Context, accountID, resourceType string, initialAmount int64) error
	DeleteAccount(ctx context.Context, accountID, resourceType string) error
	SyncTransactionWithBalance(ctx context.Context, event model.SpendEvent) error
}
