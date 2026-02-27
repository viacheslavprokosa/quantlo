package nats

import (
	"context"
	"encoding/json"
	"log/slog"
	"quantlo/internal/model"
	"quantlo/internal/service"

	"github.com/nats-io/nats.go"
)

// Handler subscribes to NATS command topics and delegates to the ledger service.
type Handler struct {
	svc  service.LedgerService
	nc   *nats.Conn
	subs []*nats.Subscription
}

func NewHandler(svc service.LedgerService, nc *nats.Conn) *Handler {
	return &Handler{svc: svc, nc: nc}
}

// Start subscribes to command topics and blocks until ctx is cancelled (graceful shutdown).
func (h *Handler) Start(ctx context.Context) error {
	s1, err := h.nc.QueueSubscribe("commands.spend", "ledger_group", func(m *nats.Msg) {
		var req model.SpendRequest
		if err := json.Unmarshal(m.Data, &req); err != nil {
			slog.Error("nats: failed to unmarshal spend command", "error", err)
			return
		}
		if _, err := h.svc.Spend(ctx, req); err != nil {
			slog.Error("nats: spend failed", "error", err, "account_id", req.AccountID)
		}
	})
	if err != nil {
		return err
	}
	h.subs = append(h.subs, s1)

	s2, err := h.nc.QueueSubscribe("commands.recharge", "ledger_group", func(m *nats.Msg) {
		var req model.RechargeRequest
		if err := json.Unmarshal(m.Data, &req); err != nil {
			slog.Error("nats: failed to unmarshal recharge command", "error", err)
			return
		}
		if err := h.svc.Recharge(ctx, req); err != nil {
			slog.Error("nats: recharge failed", "error", err, "account_id", req.AccountID)
		}
	})
	if err != nil {
		return err
	}
	h.subs = append(h.subs, s2)

	slog.Info("NATS command handler is running")

	// Block until context is cancelled.
	<-ctx.Done()
	slog.Info("NATS command handler shutting down, draining subscriptions...")

	for _, s := range h.subs {
		_ = s.Drain()
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	for _, s := range h.subs {
		_ = s.Unsubscribe()
	}
	return nil
}
