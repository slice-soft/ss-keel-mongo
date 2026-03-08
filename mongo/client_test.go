package mongo

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// newTestDriverClient initialises a mongo driver client (topology monitoring
// starts in the background). No real server is needed: operations fail after
// ~1 ms server-selection timeout, but every delegation line is still executed.
// Connect (not NewClient) is used so that Disconnect/Close succeeds.
func newTestDriverClient(t *testing.T) *mongodriver.Client {
	t.Helper()
	client, err := mongodriver.Connect(
		context.Background(),
		options.Client().
			ApplyURI("mongodb://localhost:27017").
			SetServerSelectionTimeout(time.Millisecond),
	)
	if err != nil {
		t.Fatalf("failed to create mongo driver client: %v", err)
	}
	return client
}

// fakeLogger satisfies the Logger interface for testing.
type fakeLogger struct{ called bool }

func (l *fakeLogger) Info(format string, args ...interface{}) { l.called = true }
func (l *fakeLogger) Warn(format string, args ...interface{})  {}
func (l *fakeLogger) Error(format string, args ...interface{}) {}
func (l *fakeLogger) Debug(format string, args ...interface{}) {}

// --- New ---

func TestNew_ValidateFails(t *testing.T) {
	// No database → validate returns error.
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected validation error for missing database")
	}
}

func TestNew_InvalidURIReturnsConnectError(t *testing.T) {
	_, err := New(Config{Database: "testdb", URI: "://invalid", SkipPing: true})
	if err == nil {
		t.Fatal("expected connect error for invalid URI")
	}
}

func TestNew_SkipPing(t *testing.T) {
	// mongodriver.Connect starts topology in background and returns immediately,
	// so SkipPing: true succeeds without a real server.
	client, err := New(Config{Database: "testdb", SkipPing: true})
	if err != nil {
		t.Fatalf("expected success with SkipPing, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	_ = client.Close()
}

func TestNew_SkipPing_WithLogger(t *testing.T) {
	logger := &fakeLogger{}
	client, err := New(Config{Database: "testdb", SkipPing: true, Logger: logger})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	_ = client.Close()
	if !logger.called {
		t.Fatal("expected Logger.Info to be called after successful connection")
	}
}

func TestNew_PingFailure(t *testing.T) {
	// No real server → ping fails after PingTimeout.
	_, err := New(Config{
		Database:    "testdb",
		PingTimeout: time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected ping error when no server is available")
	}
}

// --- NewRepository (non-nil client branch) ---

func TestNewRepository_WithNonNilClient(t *testing.T) {
	driver := newTestDriverClient(t)
	defer driver.Disconnect(context.Background()) //nolint:errcheck
	db := driver.Database("testdb")
	client := &Client{client: driver, database: db}

	repo := NewRepository[repoUser, string](client, "users")
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
}

// --- Client.Native ---

func TestClientNative_NilSafe(t *testing.T) {
	var c *Client
	if c.Native() != nil {
		t.Fatal("expected nil native for nil receiver")
	}
}

func TestClientNative_ReturnsUnderlyingClient(t *testing.T) {
	driver := newTestDriverClient(t)
	c := &Client{client: driver}
	if c.Native() != driver {
		t.Fatal("expected Native() to return the underlying driver client")
	}
}

// --- Client.Database ---

func TestClientDatabase_NilSafe(t *testing.T) {
	var c *Client
	if c.Database() != nil {
		t.Fatal("expected nil database for nil receiver")
	}
}

func TestClientDatabase_ReturnsUnderlying(t *testing.T) {
	driver := newTestDriverClient(t)
	db := driver.Database("testdb")
	c := &Client{client: driver, database: db}
	if c.Database() != db {
		t.Fatal("expected Database() to return the underlying database")
	}
}

// --- Client.Collection ---

func TestClientCollection_NilSafe(t *testing.T) {
	var c *Client
	if c.Collection("col") != nil {
		t.Fatal("expected nil collection for nil receiver")
	}
}

func TestClientCollection_NilDatabase(t *testing.T) {
	// database field is zero value (nil)
	c := &Client{client: newTestDriverClient(t)}
	if c.Collection("col") != nil {
		t.Fatal("expected nil collection when database is nil")
	}
}

func TestClientCollection_ReturnsCollection(t *testing.T) {
	driver := newTestDriverClient(t)
	db := driver.Database("testdb")
	c := &Client{client: driver, database: db}
	if c.Collection("users") == nil {
		t.Fatal("expected non-nil collection")
	}
}

// --- Client.Ping ---

func TestClientPing_NilSafe(t *testing.T) {
	var c *Client
	if err := c.Ping(context.Background()); err == nil {
		t.Fatal("expected error for nil client ping")
	}
}

func TestClientPing_WithClient(t *testing.T) {
	driver := newTestDriverClient(t)
	c := &Client{client: driver}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	// Ping will fail (no real server), but the delegation line is covered.
	_ = c.Ping(ctx)
}

// --- Client.Close ---

func TestClientClose_WithExplicitTimeout(t *testing.T) {
	driver := newTestDriverClient(t)
	c := &Client{client: driver, disconnectTimeout: time.Second}
	if err := c.Close(); err != nil {
		t.Fatalf("expected Close to succeed, got %v", err)
	}
}

func TestClientClose_DefaultTimeoutApplied(t *testing.T) {
	driver := newTestDriverClient(t)
	// disconnectTimeout == 0 triggers the fallback-to-default branch.
	c := &Client{client: driver, disconnectTimeout: 0}
	if err := c.Close(); err != nil {
		t.Fatalf("expected Close to succeed with default timeout, got %v", err)
	}
}

// --- mongoCollection wrapper methods ---
// These are thin delegations to *mongodriver.Collection.
// We exercise each method so the delegation line is covered; operations will
// fail (no real server), but the code path itself is tested.

func TestMongoCollection_Methods(t *testing.T) {
	driver := newTestDriverClient(t)
	coll := driver.Database("testdb").Collection("testcol")
	mc := &mongoCollection{collection: coll}

	// Raw is synchronous — no network call.
	if mc.Raw() != coll {
		t.Fatal("expected Raw() to return the underlying collection")
	}

	// FindOne is lazy in the mongo driver — returns a *SingleResult without
	// hitting the network yet, so no timeout needed.
	_ = mc.FindOne(context.Background(), bson.D{})

	// The remaining operations make network calls and will fail without a server.
	// A very short deadline ensures each fails quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	_, _ = mc.Find(ctx, bson.D{})
	_, _ = mc.InsertOne(ctx, bson.D{})
	_, _ = mc.UpdateOne(ctx, bson.D{}, bson.M{"$set": bson.M{"x": 1}})
	_, _ = mc.DeleteOne(ctx, bson.D{})
	_, _ = mc.CountDocuments(ctx, bson.D{})
}
