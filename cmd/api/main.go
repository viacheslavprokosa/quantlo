package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"quantlo/internal/config"
	"quantlo/internal/repository"
	"quantlo/internal/worker"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

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

func main() {
	//Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load config
	cfg, err := config.New()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Database connection
	dbPool, err := pgxpool.New(ctx, cfg.DSN())
	if err != nil {
		slog.Error("Database connection failed", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// Redis connection
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr()})
	defer rdb.Close()

	// NATS connection
	nc, err := nats.Connect(cfg.NatsAddr())
	if err != nil {
		slog.Error("NATS connection failed", "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	// Ledger repository
	ledgerRepo := repository.NewLedgerRepo(rdb, dbPool, nc)
	txWorker := worker.NewTransactionWorker(ledgerRepo, nc)

	// Group goroutines (Worker + Server)
	g, ctx := errgroup.WithContext(ctx)

	// --- GOROUTINE 1: WORKER ---
	g.Go(func() error {
		slog.Info("Worker starting...")
		return txWorker.Run(ctx)
	})

	mux := http.NewServeMux()
	setupRoutes(mux, ledgerRepo)

	server := &http.Server{
		Addr:         cfg.ApiAddr(),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// --- GOROUTINE 2: HTTP SERVER ---
	g.Go(func() error {
		slog.Info("HTTP Server starting", "addr", cfg.ApiAddr())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	})

	// --- GOROUTINE 3: GRACEFUL SHUTDOWN ---
	g.Go(func() error {
		<-ctx.Done()
		slog.Info("Shutting down gracefully...")

		// Create a separate context for 10 seconds to complete tasks
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Server forced shutdown", "error", err)
			return err
		}
		slog.Info("Server stopped clean")
		return nil
	})

	// Wait for all processes in the group to complete
	if err := g.Wait(); err != nil {
		slog.Error("Application finished with error", "error", err)
	} else {
		slog.Info("Application exited successfully")
	}
}



func setupRoutes(mux *http.ServeMux, repo *repository.LedgerRepo) {
	// --- Health ---
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// --- Spend ---
	mux.HandleFunc("POST /spend", func(w http.ResponseWriter, r *http.Request) {
		var req SpendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid json")
			return
		}
		res, err := repo.Spend(r.Context(), req.AccountID, req.ResourceType, req.IdempotencyKey, req.Amount)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondWithJSON(w, http.StatusOK, res)
	})

	// --- Recharge ---
	mux.HandleFunc("POST /recharge", func(w http.ResponseWriter, r *http.Request) {
		var req RechargeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if err := repo.Recharge(r.Context(), req.AccountID, req.ResourceType, req.Amount); err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondWithJSON(w, http.StatusOK, map[string]string{"status": "success"})
	})

	// --- Balance ---
	mux.HandleFunc("GET /balance", func(w http.ResponseWriter, r *http.Request) {
		accountID := r.URL.Query().Get("account_id")
		resType := r.URL.Query().Get("resource_type")

		if accountID == "" || resType == "" {
			respondWithError(w, http.StatusBadRequest, "missing account_id or resource_type")
			return
		}

		balance, err := repo.GetBalance(r.Context(), accountID, resType)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, repository.ErrNotFoundInDB) {
				status = http.StatusNotFound
			}
			respondWithError(w, status, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"account_id":    accountID,
			"resource_type": resType,
			"balance":       balance,
		})
	})

	// --- Create Account ---
	mux.HandleFunc("POST /accounts", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			AccountID    string `json:"account_id"`
			ResourceType string `json:"resource_type"`
			Amount       int64  `json:"initial_amount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "invalid json")
			return
		}

		if err := repo.CreateAccount(r.Context(), req.AccountID, req.ResourceType, req.Amount); err != nil {
			respondWithError(w, http.StatusConflict, err.Error())
			return
		}
		respondWithJSON(w, http.StatusCreated, map[string]string{"status": "created"})
	})

	// --- Delete Account ---
	mux.HandleFunc("DELETE /accounts", func(w http.ResponseWriter, r *http.Request) {
		accountID := r.URL.Query().Get("account_id")
		resType := r.URL.Query().Get("resource_type")

		if accountID == "" || resType == "" {
			respondWithError(w, http.StatusBadRequest, "missing account_id or resource_type")
			return
		}

		if err := repo.DeleteAccount(r.Context(), accountID, resType); err != nil {
			respondWithError(w, http.StatusNotFound, err.Error())
			return
		}
		respondWithJSON(w, http.StatusNoContent, nil)
	})
}

func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		json.NewEncoder(w).Encode(payload)
	}
}

func respondWithError(w http.ResponseWriter, status int, message string) {
	respondWithJSON(w, status, map[string]string{"error": message})
}
