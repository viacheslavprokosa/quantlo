package repository

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"quantlo/internal/model"
	"quantlo/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Compile-time assertion: LedgerRepo must implement service.LedgerService.
var _ service.LedgerService = (*LedgerRepo)(nil)

//go:embed spend.lua
var spendLuaScript string

var (
	ErrAlreadyProcessed = errors.New("request already processed (idempotency)")
	ErrCacheMiss        = errors.New("balance not found in cache")
	ErrInsufficient     = errors.New("insufficient funds")
	ErrNotFoundInDB     = errors.New("account not found in database")
)

type LedgerRepo struct {
	rdb *redis.Client
	db  *pgxpool.Pool
	bus MessageBus
}

func NewLedgerRepo(rdb *redis.Client, db *pgxpool.Pool, bus MessageBus) *LedgerRepo {
	return &LedgerRepo{
		rdb: rdb,
		db:  db,
		bus: bus,
	}
}

func (r *LedgerRepo) Spend(ctx context.Context, req model.SpendRequest) (*model.SpendResult, error) {
	result, err := r.executeLua(ctx, req)

	if errors.Is(err, ErrCacheMiss) {
		slog.Info("cold start, warming up cache", "account_id", req.AccountID)

		if err := r.warmUpCache(ctx, req.AccountID, req.ResourceType); err != nil {
			return nil, err
		}

		return r.executeLua(ctx, req)
	}

	return result, err
}

func (r *LedgerRepo) Recharge(ctx context.Context, req model.RechargeRequest) error {
	query := `
        UPDATE balances 
        SET amount = amount + $1, updated_at = NOW() 
        WHERE account_id = $2 AND resource_type = $3 AND deleted_at IS NULL`

	res, err := r.db.Exec(ctx, query, req.Amount, req.AccountID, req.ResourceType)
	if err != nil {
		return fmt.Errorf("db recharge error: %w", err)
	}

	if res.RowsAffected() == 0 {
		return ErrNotFoundInDB
	}

	cacheKey := fmt.Sprintf("balance:%s:%s", req.AccountID, req.ResourceType)
	return r.rdb.Del(ctx, cacheKey).Err()
}

func (r *LedgerRepo) GetBalance(ctx context.Context, accountID, resourceType string) (int64, error) {
	balanceKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)

	val, err := r.rdb.Get(ctx, balanceKey).Int64()
	if err == nil {
		return val, nil
	}

	if errors.Is(err, redis.Nil) {
		if err := r.warmUpCache(ctx, accountID, resourceType); err != nil {
			return 0, err
		}
		return r.rdb.Get(ctx, balanceKey).Int64()
	}

	return 0, err
}

func (r *LedgerRepo) CreateAccount(ctx context.Context, accountID, resourceType string, initialAmount int64) error {
	query := `
        INSERT INTO balances (account_id, resource_type, amount, created_at, updated_at)
        VALUES ($1, $2, $3, NOW(), NOW())
        ON CONFLICT (account_id, resource_type) DO NOTHING`

	res, err := r.db.Exec(ctx, query, accountID, resourceType, initialAmount)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return errors.New("account already exists")
	}

	cacheKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)
	return r.rdb.Set(ctx, cacheKey, initialAmount, 0).Err()
}

func (r *LedgerRepo) DeleteAccount(ctx context.Context, accountID, resourceType string) error {
	query := `
        UPDATE balances 
        SET deleted_at = NOW(), updated_at = NOW() 
        WHERE account_id = $1 AND resource_type = $2 AND deleted_at IS NULL`

	res, err := r.db.Exec(ctx, query, accountID, resourceType)
	if err != nil {
		return fmt.Errorf("db delete account: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFoundInDB
	}

	balanceKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)
	pipe := r.rdb.Pipeline()
	pipe.Del(ctx, balanceKey)
	pipe.Set(ctx, fmt.Sprintf("deleted:%s:%s", accountID, resourceType), "1", 30*time.Second)
	_, err = pipe.Exec(ctx)

	return err
}

func (r *LedgerRepo) SyncTransactionWithBalance(ctx context.Context, event model.SpendEvent) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var insertedKey string
	queryInsert := `
        INSERT INTO transactions (account_id, resource_type, amount, idempotency_key, created_at)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (idempotency_key) DO NOTHING
        RETURNING idempotency_key`

	err = tx.QueryRow(ctx, queryInsert,
		event.AccountID, event.ResourceType, event.Amount, event.IdempotencyKey, event.CreatedAt,
	).Scan(&insertedKey)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if insertedKey == "" {
		return nil
	}

	queryUpdate := `UPDATE balances SET amount = amount - $1 WHERE account_id = $2 AND resource_type = $3`
	_, err = tx.Exec(ctx, queryUpdate, event.Amount, event.AccountID, event.ResourceType)

	return tx.Commit(ctx)
}

func (r *LedgerRepo) executeLua(ctx context.Context, req model.SpendRequest) (*model.SpendResult, error) {
	balanceKey := fmt.Sprintf("balance:%s:%s", req.AccountID, req.ResourceType)
	idemKey := fmt.Sprintf("idem:%s", req.IdempotencyKey)

	result, err := r.rdb.Eval(ctx, spendLuaScript, []string{balanceKey, idemKey}, req.Amount).Result()
	if err != nil {
		return nil, err
	}

	resArray := result.([]interface{})
	status := resArray[0].(int64)

	switch status {
	case 1:
		newBalance := resArray[1].(int64)
		r.publishEvent(req)
		return &model.SpendResult{NewBalance: newBalance, Status: "SUCCESS"}, nil
	case 0:
		return nil, ErrAlreadyProcessed
	case -1:
		return nil, ErrCacheMiss
	case -2:
		return nil, ErrInsufficient
	default:
		return nil, fmt.Errorf("unknown lua status: %d", status)
	}
}

func (r *LedgerRepo) warmUpCache(ctx context.Context, accountID, resourceType string) error {
	var currentBalance int64
	var deletedAt *time.Time

	query := `SELECT amount, deleted_at FROM balances WHERE account_id = $1 AND resource_type = $2`
	err := r.db.QueryRow(ctx, query, accountID, resourceType).Scan(&currentBalance, &deletedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFoundInDB
		}
		return err
	}
	if deletedAt != nil {
		return errors.New("account is deleted")
	}

	return r.rdb.Set(ctx, fmt.Sprintf("balance:%s:%s", accountID, resourceType), currentBalance, 0).Err()
}

func (r *LedgerRepo) publishEvent(req model.SpendRequest) {
	event := model.SpendEvent{
		AccountID:      req.AccountID,
		ResourceType:   req.ResourceType,
		Amount:         req.Amount,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      time.Now(),
	}
	data, _ := json.Marshal(event)

	if err := r.bus.Publish("transactions.created", data); err != nil {
		slog.Error("event publish failed", "error", err, "topic", "transactions.created")
	}
}
