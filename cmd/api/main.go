/* package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"quantlo/internal/config"
	"quantlo/internal/repository"
	transportHTTP "quantlo/internal/transport/http"
	transportNATS "quantlo/internal/transport/nats"
	"quantlo/internal/worker"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	config, err := config.New()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbPool, err := pgxpool.New(ctx, config.DSN())
	if err != nil {
		slog.Error("Failed to connect to Postgres", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: config.RedisAddr(),
	})
	defer rdb.Close()

	nc, err := nats.Connect(config.NatsAddr())
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer nc.Close()
	
	bus := transportNATS.NewNatsBus(nc)

	ledgerRepo := repository.NewLedgerRepo(rdb, dbPool, bus)

	httpHandler := transportHTTP.NewHandler(ledgerRepo)
	natsCommandHandler := transportNATS.NewCommandHandler(ledgerRepo, nc)
	
	dbWorker := worker.NewTransactionWorker(ledgerRepo, nc)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		slog.Info("NATS Command Handler starting...")
		return natsCommandHandler.Subscribe(ctx)
	})

	g.Go(func() error {
		slog.Info("DB Sync Worker starting...")
		return dbWorker.Run(ctx)
	})

	if addr, err := config.ApiAddr(); err == nil {
		mux := http.NewServeMux()
		httpHandler.Register(mux)

		srv := &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		g.Go(func() error {
			slog.Info("HTTP Server starting", "addr", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		})
		
		g.Go(func() error {
			<-ctx.Done()
			slog.Info("Shutting down gracefully...")

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := srv.Shutdown(shutdownCtx); err != nil {
				slog.Error("Server forced to shutdown", "error", err)
				return err
			}
			slog.Info("Server stopped clean")
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		slog.Error("Application finished with error", "error", err)
		os.Exit(1)
	}

	slog.Info("Application exited successfully")
} */
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"quantlo/internal/infrastructure"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, cleanup, err := infrastructure.Bootstrap(ctx)
	if err != nil {
		slog.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	if err := app.Run(ctx); err != nil {
		slog.Error("app execution failed", "error", err)
		os.Exit(1)
	}
}