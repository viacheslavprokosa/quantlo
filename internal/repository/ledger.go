package repository

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

//go:embed spend.lua
var spendLuaScript string

type LedgerRepo struct {
	redisClient *redis.Client
	dbPool      *pgxpool.Pool
	natsConn    *nats.Conn
}

func NewLedgerRepo(rdb *redis.Client, db *pgxpool.Pool, natsConn *nats.Conn) *LedgerRepo {
	return &LedgerRepo{
		redisClient: rdb,
		dbPool:      db,
		natsConn:    natsConn,
	}
}

type SpendResult struct {
	NewBalance int64
	Status     string
}

var (
	ErrAlreadyProcessed = errors.New("request already processed (idempotency)")
	ErrCacheMiss        = errors.New("balance not found in cache")
	ErrInsufficient     = errors.New("insufficient funds")
	ErrNotFoundInDB     = errors.New("account not found in database")
)

// Spend calls the Lua script. If the cache is empty, it fetches data from the DB and retries.
func (r *LedgerRepo) Spend(ctx context.Context, accountID, resourceType, idempotencyKey string, amount int64) (*SpendResult, error) {
	result, err := r.executeLua(ctx, accountID, resourceType, idempotencyKey, amount)

	// If we got ErrCacheMiss (our status -1), go to the database!
	if errors.Is(err, ErrCacheMiss) {
		log.Printf("Cold start for %s:%s. Going to PostgreSQL...", accountID, resourceType)

		err = r.warmUpCache(ctx, accountID, resourceType)
		if err != nil {
			return nil, err
		}

		// Cache is "warmed up", repeat the Lua script call
		log.Println("Repeating deduction after cache warmup...")
		return r.executeLua(ctx, accountID, resourceType, idempotencyKey, amount)
	}

	return result, err
}

// executeLua is our old logic, extracted into a private function
func (r *LedgerRepo) executeLua(ctx context.Context, accountID, resourceType, idempotencyKey string, amount int64) (*SpendResult, error) {
	balanceKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)
	idemKey := fmt.Sprintf("idem:%s", idempotencyKey)

	keys := []string{balanceKey, idemKey}
	args := []interface{}{amount}

	result, err := r.redisClient.Eval(ctx, spendLuaScript, keys, args...).Result()
	if err != nil {
		return nil, fmt.Errorf("error executing Lua script: %w", err)
	}

	resArray, ok := result.([]interface{})
	if !ok || len(resArray) < 2 {
		return nil, errors.New("unexpected response format from Redis")
	}

	statusCode := resArray[0].(int64)

	switch statusCode {
	case 1:
		newBalance := resArray[1].(int64)
		event := SpendEvent{
			AccountID:      accountID,
			ResourceType:   resourceType,
			Amount:         amount,
			IdempotencyKey: idempotencyKey,
			CreatedAt:      time.Now(),
		}
		eventData, _ := json.Marshal(event)
		_ = r.natsConn.Publish("transactions.created", eventData)
		return &SpendResult{NewBalance: newBalance, Status: "SUCCESS"}, nil
	case 0:
		return nil, ErrAlreadyProcessed
	case -1:
		return nil, ErrCacheMiss
	case -2:
		return nil, ErrInsufficient
	default:
		return nil, fmt.Errorf("unknown status from Lua: %d", statusCode)
	}
}

// warmUpCache fetches the balance from Postgres and puts it into Redis
func (r *LedgerRepo) warmUpCache(ctx context.Context, accountID, resourceType string) error {
	var currentBalance int64

	// Perform SELECT from our balances table
	query := `SELECT amount FROM balances WHERE account_id = $1 AND resource_type = $2`
	err := r.dbPool.QueryRow(ctx, query, accountID, resourceType).Scan(&currentBalance)

	if err != nil {
		// If the record doesn't exist in the DB at all
		if err.Error() == "no rows in result set" {
			return ErrNotFoundInDB
		}
		return fmt.Errorf("database query error: %w", err)
	}

	// Write the found balance to Redis.
	// We don't set an expiration (TTL) because this is the primary cache.
	balanceKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)
	err = r.redisClient.Set(ctx, balanceKey, currentBalance, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to save balance to Redis: %w", err)
	}

	return nil
}
