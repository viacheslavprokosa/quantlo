package repository

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
)

//go:embed spend.lua
var spendLuaScript string

// LedgerRepo is a structure that contains connections to our storages
type LedgerRepo struct {
	redisClient *redis.Client
}

// NewLedgerRepo is a constructor for our repository
func NewLedgerRepo(rdb *redis.Client) *LedgerRepo {
	return &LedgerRepo{
		redisClient: rdb,
	}
}

// SpendResult is a structure for convenient result returning
type SpendResult struct {
	NewBalance int64
	Status     string
}

// Common business logic errors
var (
	ErrAlreadyProcessed = errors.New("request already processed (idempotency)")
	ErrCacheMiss        = errors.New("balance not found in cache")
	ErrInsufficient     = errors.New("insufficient funds")
)

// Spend calls our Lua script in Redis
func (r *LedgerRepo) Spend(ctx context.Context, accountID, resourceType, idempotencyKey string, amount int64) (*SpendResult, error) {
	// 1. Form keys for Redis
	balanceKey := fmt.Sprintf("balance:%s:%s", accountID, resourceType)
	idemKey := fmt.Sprintf("idem:%s", idempotencyKey)

	keys := []string{balanceKey, idemKey}

	// 2. Arguments (deduction amount)
	args := []interface{}{amount}

	// 3. Execute Lua script
	result, err := r.redisClient.Eval(ctx, spendLuaScript, keys, args...).Result()
	if err != nil {
		return nil, fmt.Errorf("error executing Lua script: %w", err)
	}

	// 4. Parse response (our array [status_code, payload])
	resArray, ok := result.([]interface{})
	if !ok || len(resArray) < 2 {
		return nil, errors.New("unexpected response format from Redis")
	}

	statusCode := resArray[0].(int64)

	// 5. Handle business logic based on statuses from Lua
	switch statusCode {
	case 1:
		// Success! The second argument is the new balance
		newBalance := resArray[1].(int64)
		return &SpendResult{NewBalance: newBalance, Status: "SUCCESS"}, nil

	case 0:
		// Request with this Idempotency-Key already exists
		return nil, ErrAlreadyProcessed

	case -1:
		// Cache is empty (Cold start). We'll have to go to PostgreSQL
		return nil, ErrCacheMiss

	case -2:
		// Not enough money
		return nil, ErrInsufficient

	default:
		return nil, fmt.Errorf("unknown status from Lua: %d", statusCode)
	}
}
