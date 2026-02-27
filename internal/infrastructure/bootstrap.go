package infrastructure

import (
	"context"
	"log/slog"
	"quantlo/internal/config"
	"quantlo/internal/repository"
	"quantlo/internal/service"
	transportGRPC "quantlo/internal/transport/grpc"
	transportHTTP "quantlo/internal/transport/http"
	transportNATS "quantlo/internal/transport/nats"
	"quantlo/internal/worker"
)

// Bootstrap initialises all dependencies from config and wires up the application.
// Returns the App, a cleanup function, or an error.
func Bootstrap(ctx context.Context) (*App, func(), error) {
	cfg, err := config.New()
	if err != nil {
		return nil, nil, err
	}

	db, err := connectPostgres(cfg.DSN())
	if err != nil {
		return nil, nil, err
	}

	rdb, err := connectRedis(cfg.RedisAddr())
	if err != nil {
		db.Close()
		return nil, nil, err
	}

	var cleanupFns []func()
	cleanupFns = append(cleanupFns, func() {
		db.Close()
		rdb.Close()
	})

	// ── Bus wiring ────────────────────────────────────────────────────────────
	// Bus is resolved first so it can be injected into the repository.
	var bus repository.MessageBus

	var servers []Server

	switch cfg.BusProvider {
	case "nats":
		nc, err := connectNats(cfg.NatsAddr())
		if err != nil {
			return nil, runCleanup(cleanupFns), err
		}
		bus = transportNATS.NewBus(nc)
		cleanupFns = append(cleanupFns, nc.Close)

		// Repository + service depend on bus, so build them here inside the case.
		repo := repository.NewLedgerRepo(rdb, db, bus)
		var svc service.LedgerService = repo

		// NATS command handler (subscribe to commands.spend / commands.recharge)
		servers = append(servers, transportNATS.NewHandler(svc, nc))
		// DB-sync worker (subscribe to transactions.created)
		servers = append(servers, worker.NewTransactionWorker(svc, nc))

		// gRPC server (available as a separate transport)
		servers = append(servers, transportGRPC.NewServer(":50051", svc))

		// HTTP server (optional)
		if addr, apiErr := cfg.ApiAddr(); apiErr == nil {
			servers = append(servers, transportHTTP.NewServer(addr, svc))
		} else {
			slog.Info("HTTP API disabled", "reason", apiErr)
		}

	case "grpc":
		grpcBus, cleanup, err := transportGRPC.NewGrpcBusFromAddr(cfg.GRPCAddr())
		if err != nil {
			return nil, runCleanup(cleanupFns), err
		}
		bus = grpcBus
		cleanupFns = append(cleanupFns, cleanup)

		repo := repository.NewLedgerRepo(rdb, db, bus)
		var svc service.LedgerService = repo

		servers = append(servers, transportGRPC.NewServer(":50051", svc))

		if addr, apiErr := cfg.ApiAddr(); apiErr == nil {
			servers = append(servers, transportHTTP.NewServer(addr, svc))
		} else {
			slog.Info("HTTP API disabled", "reason", apiErr)
		}
	}

	return NewApp(servers), runCleanup(cleanupFns), nil
}

// runCleanup returns a single function that calls all cleanup functions in reverse order.
func runCleanup(fns []func()) func() {
	return func() {
		for i := len(fns) - 1; i >= 0; i-- {
			fns[i]()
		}
	}
}
