package grpc

import (
	"context"
	"encoding/json"
	"testing"

	"quantlo/internal/model"
	"quantlo/internal/proto"
)

type mockService struct {
	syncCalled bool
	syncErr    error
}

func (m *mockService) Spend(ctx context.Context, req model.SpendRequest) (*model.SpendResult, error) {
	return nil, nil
}
func (m *mockService) Recharge(ctx context.Context, req model.RechargeRequest) error { return nil }
func (m *mockService) GetBalance(ctx context.Context, accountID, resourceType string) (int64, error) {
	return 0, nil
}
func (m *mockService) CreateAccount(ctx context.Context, accountID, resourceType string, initialAmount int64) error {
	return nil
}
func (m *mockService) DeleteAccount(ctx context.Context, accountID, resourceType string) error {
	return nil
}
func (m *mockService) SyncTransactionWithBalance(ctx context.Context, event model.SpendEvent) error {
	m.syncCalled = true
	return m.syncErr
}

func TestServer_Publish(t *testing.T) {
	svc := &mockService{}
	server := &Server{svc: svc}

	event := model.SpendEvent{AccountID: "user123", Amount: 100}
	payload, _ := json.Marshal(event)

	req := &proto.EventRequest{
		Topic:   "transactions.created",
		Payload: payload,
	}

	res, err := server.Publish(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.Success {
		t.Error("expected success")
	}

	if !svc.syncCalled {
		t.Error("expected SyncTransactionWithBalance to be called")
	}
}
