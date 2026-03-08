package mongo

import (
	"context"
	"errors"
	"testing"
)

type fakePingClient struct {
	called bool
	err    error
}

func (c *fakePingClient) Ping(context.Context) error {
	c.called = true
	return c.err
}

func TestHealthChecker_NameAndCheck(t *testing.T) {
	client := &fakePingClient{}
	checker := &HealthChecker{client: client}

	if checker.Name() != "mongodb" {
		t.Fatalf("expected checker name mongodb, got %q", checker.Name())
	}

	if err := checker.Check(context.Background()); err != nil {
		t.Fatalf("expected successful check, got %v", err)
	}

	if !client.called {
		t.Fatal("expected checker to call client ping")
	}
}

func TestHealthChecker_CheckReturnsPingError(t *testing.T) {
	wantErr := errors.New("ping failed")
	checker := &HealthChecker{client: &fakePingClient{err: wantErr}}

	err := checker.Check(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected ping error %v, got %v", wantErr, err)
	}
}

func TestHealthChecker_CheckReturnsErrorWhenClientIsNil(t *testing.T) {
	checker := &HealthChecker{}

	err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("expected nil-client error")
	}
}

func TestNewHealthChecker(t *testing.T) {
	// NewHealthChecker with a nil *Client: the interface wraps a typed nil so
	// h.client != nil (interface), but Ping returns an error for nil receiver.
	checker := NewHealthChecker(nil)
	if checker == nil {
		t.Fatal("expected non-nil HealthChecker")
	}
	if checker.Name() != "mongodb" {
		t.Fatalf("expected name mongodb, got %q", checker.Name())
	}
	err := checker.Check(context.Background())
	if err == nil {
		t.Fatal("expected error when underlying *Client is nil")
	}
}
