package utiltx_test

import (
	"context"
	"shared/utiltx"
	"testing"
)

var _ utiltx.Transactor = utiltx.NewTransactor(nil)

func TestTx_returnsNilOnPlainContext(t *testing.T) {
	if tx := utiltx.Tx(context.Background()); tx != nil {
		t.Errorf("Tx() = %v, want nil", tx)
	}
}

func TestTx_returnsNilOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if tx := utiltx.Tx(ctx); tx != nil {
		t.Errorf("Tx() on cancelled ctx = %v, want nil", tx)
	}
}

func TestTx_returnsNilOnValueContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "some-key", "some-val")
	if tx := utiltx.Tx(ctx); tx != nil {
		t.Errorf("Tx() with unrelated value = %v, want nil", tx)
	}
}

func TestNewTransactor_acceptsNilDB(t *testing.T) {
	tx := utiltx.NewTransactor(nil)
	if tx == nil {
		t.Fatal("NewTransactor(nil) returned nil")
	}
}
