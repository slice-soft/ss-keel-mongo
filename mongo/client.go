package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Client wraps mongo-driver primitives and keeps Keel-oriented defaults.
type Client struct {
	client            *mongodriver.Client
	database          *mongodriver.Database
	disconnectTimeout time.Duration
}

// New creates a MongoDB client, validates connectivity, and returns the selected database handle.
func New(cfg Config) (*Client, error) {
	cfg.withDefaults()

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancelConnect()

	client, err := mongodriver.Connect(connectCtx, buildClientOptions(cfg))
	if err != nil {
		return nil, fmt.Errorf("unable to connect to mongodb: %w", err)
	}

	if !cfg.SkipPing {
		pingCtx, cancelPing := context.WithTimeout(context.Background(), cfg.PingTimeout)
		defer cancelPing()

		if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
			_ = client.Disconnect(context.Background())
			return nil, fmt.Errorf("unable to ping mongodb: %w", err)
		}
	}

	if cfg.Logger != nil {
		cfg.Logger.Info("mongodb connected [database=%s]", cfg.Database)
	}

	return &Client{
		client:            client,
		database:          client.Database(cfg.Database),
		disconnectTimeout: cfg.DisconnectTimeout,
	}, nil
}

// Native returns the underlying *mongo.Client for advanced driver-specific usage.
func (c *Client) Native() *mongodriver.Client {
	if c == nil {
		return nil
	}
	return c.client
}

// Database returns the selected *mongo.Database.
func (c *Client) Database() *mongodriver.Database {
	if c == nil {
		return nil
	}
	return c.database
}

// Collection returns a named collection from the selected database.
func (c *Client) Collection(name string) *mongodriver.Collection {
	if c == nil || c.database == nil {
		return nil
	}
	return c.database.Collection(name)
}

// Ping checks client health against the primary node.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return errors.New("mongodb client is nil")
	}
	return c.client.Ping(ctx, readpref.Primary())
}

// Close disconnects the MongoDB client using a bounded timeout.
func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}

	timeout := c.disconnectTimeout
	if timeout <= 0 {
		timeout = defaultDisconnectTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return c.client.Disconnect(ctx)
}
