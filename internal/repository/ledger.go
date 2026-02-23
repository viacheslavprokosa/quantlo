package repository

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

//go:embed spend.lua
var spendLuaScript string

type SpendResult struct {
	NewBalance int64  `json:"new_balance"`
	Status     string `json:"status"`
}

var (
	ErrAlreadyProcessed = errors.New("request already processed (idempotency)")
	ErrCacheMiss        = errors.New("balance not found in cache")
	ErrInsufficient     = errors.New("insufficient funds")
	ErrNotFoundInDB     = errors.New("account not found in database")
)

type LedgerRepo struct {
	rdb      *redis.Client
	db       *pgxpool.Pool
	natsConn *nats.Conn
}

func NewLedgerRepo(rdb *redis.Client, db *pgxpool.Pool, natsConn *nats.Conn) *LedgerRepo {
	return &LedgerRepo{
		rdb:      rdb,
		db:       db,
		natsConn: natsConn,
	}
}

func (r *LedgerRepo) Spend(ctx context.Context, accountID, resourceType, idempotencyKey string, amount int64) (*SpendResult, error) {
	result, err := r.executeLua(ctx, accountID, resourceType, idempotencyKey, amount)

	if errors.Is(err, ErrCacheMiss) {
		slog.Info("cold start, warming up cache", "account_id", accountID, "resource", resourceType)

		if err := r.warmUpCache(ctx, accountID, resourceType); err != nil {
			return nil, err
		}

		return r.executeLua(ctx, accountID, resourceType, idempotencyKey, amount)
	}

	return result, err
}

func (r *LedgerRepo) CreateAccount(ctx context.Context, accountID, resourceType string, initialAmount int64) error {
    query := `
        INSERT INTO balances (account_id, resource_type, amount, created_at, updated_at)
        VALUES ($1, $2, $3, NOW(), NOW())
        ON CONFLICT (account_id, resource_type) DO NOTHING`

    res, err := r.db.Exec(ctx, query, accountID, resourceType, initialAmount)
    if err != nil {
        return fmt.Errorf("failed to create account: %w", err)
    }

    if res.RowsAffected() == 0 {
        return errors.New("account already exists")
    }

    cacheKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)
    _ = r.rdb.Set(ctx, cacheKey, initialAmount, 0).Err()

    return nil
}

func (r *LedgerRepo) DeleteAccount(ctx context.Context, accountID, resourceType string) error {
	query := `
		UPDATE balances 
		SET deleted_at = NOW(), updated_at = NOW() 
		WHERE account_id = $1 AND resource_type = $2 AND deleted_at IS NULL`

	res, err := r.db.Exec(ctx, query, accountID, resourceType)
	
	if err != nil {
		return fmt.Errorf("failed to soft delete account: %w", err)
	}

	if res.RowsAffected() == 0 {
		return errors.New("account not found or already deleted")
	}

	// Clear Redis
	balanceKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)
	
	// Use Pipeline for speed
	pipe := r.rdb.Pipeline()
	pipe.Del(ctx, balanceKey)
	// Lock account in Redis for a short period (Tombstone), 
	// so even Cold Start doesn't try to raise it 5-10 seconds.
	pipe.Set(ctx, fmt.Sprintf("deleted:%s:%s", accountID, resourceType), "1", 30*time.Second)
	
	_, err = pipe.Exec(ctx)
	if err != nil {
		slog.Error("redis cleanup failed during deletion", "error", err)
	}

	slog.Info("account marked as deleted", "account_id", accountID, "type", resourceType)

	return nil
}

func (r *LedgerRepo) Recharge(ctx context.Context, accountID, resourceType string, amount int64) error {
	query := `
		UPDATE balances 
		SET amount = amount + $1, updated_at = NOW() 
		WHERE account_id = $2 AND resource_type = $3`

	res, err := r.db.Exec(ctx, query, amount, accountID, resourceType)
	if err != nil {
		return fmt.Errorf("db recharge error: %w", err)
	}

	if res.RowsAffected() == 0 {
		return ErrNotFoundInDB
	}

	cacheKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)
	if err := r.rdb.Del(ctx, cacheKey).Err(); err != nil {
		slog.Error("failed to invalidate cache", "key", cacheKey, "error", err)
	}

	slog.Info("balance recharged successfully", "account_id", accountID, "amount", amount)
	return nil
}

func (r *LedgerRepo) SyncTransactionWithBalance(ctx context.Context, event SpendEvent) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
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
		return fmt.Errorf("transaction insert failed: %w", err)
	}

	if insertedKey == "" {
		slog.Debug("skipping duplicate transaction", "key", event.IdempotencyKey)
		return nil
	}

	queryUpdate := `
		UPDATE balances 
		SET amount = amount - $1, updated_at = NOW() 
		WHERE account_id = $2 AND resource_type = $3`

	res, err := tx.Exec(ctx, queryUpdate, event.Amount, event.AccountID, event.ResourceType)
	if err != nil {
		return fmt.Errorf("balance update failed: %w", err)
	}

	if res.RowsAffected() == 0 {
		return fmt.Errorf("no balance record to update for %s", event.AccountID)
	}

	return tx.Commit(ctx)
}

func (r *LedgerRepo) GetBalance(ctx context.Context, accountID, resourceType string) (int64, error) {
	balanceKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)

	val, err := r.rdb.Get(ctx, balanceKey).Int64()
	if err == nil {
		return val, nil
	}

	if errors.Is(err, redis.Nil) {
		slog.Info("balance cache miss, fetching from db", "account_id", accountID)
		
		if err := r.warmUpCache(ctx, accountID, resourceType); err != nil {
			return 0, err
		}

		return r.rdb.Get(ctx, balanceKey).Int64()
	}

	return 0, fmt.Errorf("redis error: %w", err)
}

func (r *LedgerRepo) executeLua(ctx context.Context, accountID, resourceType, idempotencyKey string, amount int64) (*SpendResult, error) {
	balanceKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)
	idemKey := fmt.Sprintf("idem:%s", idempotencyKey)

	result, err := r.rdb.Eval(ctx, spendLuaScript, []string{balanceKey, idemKey}, amount).Result()
	if err != nil {
		return nil, fmt.Errorf("lua error: %w", err)
	}

	resArray, ok := result.([]interface{})
	if !ok || len(resArray) < 2 {
		return nil, errors.New("invalid redis response format")
	}

	status := resArray[0].(int64)
	switch status {
	case 1:
		newBalance := resArray[1].(int64)
		r.publishEvent(accountID, resourceType, idempotencyKey, amount)
		return &SpendResult{NewBalance: newBalance, Status: "SUCCESS"}, nil
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

	balanceKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)
	return r.rdb.Set(ctx, balanceKey, currentBalance, 0).Err()
}

func (r *LedgerRepo) publishEvent(accID, resType, idemKey string, amount int64) {
	event := SpendEvent{
		AccountID:      accID,
		ResourceType:   resType,
		Amount:         amount,
		IdempotencyKey: idemKey,
		CreatedAt:      time.Now(),
	}
	data, _ := json.Marshal(event)
	if err := r.natsConn.Publish("transactions.created", data); err != nil {
		slog.Error("nats publish failed", "error", err, "key", idemKey)
	}
}