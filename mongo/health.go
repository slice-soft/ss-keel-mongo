package mongo

import (
	"context"
	"errors"
)

// HealthChecker exposes MongoDB state through Keel's health endpoint.
type HealthChecker struct {
	client interface {
		Ping(ctx context.Context) error
	}
}

// NewHealthChecker returns a checker suitable for app.RegisterHealthChecker(...).
func NewHealthChecker(client *Client) *HealthChecker {
	return &HealthChecker{client: client}
}

// Name is the key exposed under GET /health checks.
func (h *HealthChecker) Name() string {
	return "mongodb"
}

// Check performs a ping against MongoDB.
func (h *HealthChecker) Check(ctx context.Context) error {
	if h == nil || h.client == nil {
		return errors.New("mongodb client is nil")
	}
	return h.client.Ping(ctx)
}
