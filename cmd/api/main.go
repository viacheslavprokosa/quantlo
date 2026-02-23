package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
	"quantlo/internal/config"
	"quantlo/internal/repository"
	"quantlo/internal/worker"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

type SpendRequest struct {
	AccountID      string `json:"account_id"`
	ResourceType   string `json:"resource_type"`
	Amount         int64  `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
}

func main() {
	_ = godotenv.Load()

	ctx := context.Background()

	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Database connection
	dsn := cfg.DSN()
	dbPool, err := pgxpool.New(ctx, dsn)

	if err != nil {
		log.Fatalf("Failed to create DB connection pool: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		log.Fatalf("Database is not responding: %v", err)
	}

	// Redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr(),
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Error connecting to Redis: %v", err)
	}

	// NATS connection
	nc, err := nats.Connect(cfg.NatsAddr())
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}
	defer nc.Close()

	// Ledger repository
	ledgerRepo := repository.NewLedgerRepo(rdb, dbPool, nc)

	// Transaction worker
	txWorker := worker.NewTransactionWorker(dbPool, nc)
	if err := txWorker.Start(ctx); err != nil {
		log.Fatalf("Error starting transaction worker: %v", err)
	}

	// HTTP server
	mux := http.NewServeMux()

	// Health-check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("Qantlo Engine is running!"))
		if err != nil {
			log.Printf("Error writing response: %v", err)
		}
	})

	// Main token deduction method
	mux.HandleFunc("POST /spend", func(w http.ResponseWriter, r *http.Request) {
		var req SpendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Call our business logic (including "cold start")
		result, err := ledgerRepo.Spend(r.Context(), req.AccountID, req.ResourceType, req.IdempotencyKey, req.Amount)

		w.Header().Set("Content-Type", "application/json")

		// Handle expected business logic errors
		if err != nil {
			switch err {
			case repository.ErrInsufficient:
				w.WriteHeader(http.StatusPaymentRequired)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Insufficient funds"})
			case repository.ErrAlreadyProcessed:
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Request already processed"})
			case repository.ErrNotFoundInDB:
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Account not found in the system"})
			default:
				log.Printf("Internal error: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Something went wrong"})
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(result)
	})

	server := &http.Server{
		Addr:         cfg.ApiAddr(),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
