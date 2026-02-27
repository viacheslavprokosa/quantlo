package infrastructure

import (
	"context"
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
		_ = rdb.Close()
	})

	// ── Infrastructure wiring ──────────────────────────────────────────────────
	var bus repository.MessageBus
	var servers []Server

	// 1. Bus setup
	switch cfg.BusProvider {
	case "nats":
		nc, err := connectNats(cfg.NatsAddr())
		if err != nil {
			return nil, runCleanup(cleanupFns), err
		}
		bus = transportNATS.NewBus(nc)
		cleanupFns = append(cleanupFns, nc.Close)

		// NATS needs handlers to process commands/syncs
		repo := repository.NewLedgerRepo(rdb, db, bus)
		var svc service.LedgerService = repo

		// If worker is NATS, add the worker
		if cfg.WorkerProvider == "nats" {
			servers = append(servers, worker.NewTransactionWorker(svc, nc))
		}
		// NATS can also handle commands
		servers = append(servers, transportNATS.NewHandler(svc, nc))

		// Other transports
		servers = append(servers, transportGRPC.NewServer(":50051", svc))
		if addr, apiErr := cfg.ApiAddr(); apiErr == nil {
			servers = append(servers, transportHTTP.NewServer(addr, svc))
		}

	case "grpc":
		grpcBus, cleanup, err := transportGRPC.NewGrpcBusFromAddr(cfg.GRPCAddr(), cfg.BusBufferSize)
		if err != nil {
			return nil, runCleanup(cleanupFns), err
		}
		bus = grpcBus
		cleanupFns = append(cleanupFns, cleanup)

		repo := repository.NewLedgerRepo(rdb, db, bus)
		var svc service.LedgerService = repo

		// gRPC server acts as worker if WorkerProvider is "grpc" (handled in Server.Publish)
		servers = append(servers, transportGRPC.NewServer(":50051", svc))

		if addr, apiErr := cfg.ApiAddr(); apiErr == nil {
			servers = append(servers, transportHTTP.NewServer(addr, svc))
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
