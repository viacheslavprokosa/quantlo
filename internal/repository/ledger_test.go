package repository

import (
	"testing"
)

type mockBus struct {
	published bool
}

func (m *mockBus) Publish(topic string, data []byte) error {
	m.published = true
	return nil
}

func TestSpend_InsufficientFunds(t *testing.T) {
	// Normally we would mock Redis/DB here.
	// This is a placeholder to show where the tests would go.
	// In a real project, we'd use testify/mock or similar.
}

func TestRecharge_NotFound(t *testing.T) {
	// ...
}
