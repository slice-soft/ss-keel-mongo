package mongo

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestConfigWithDefaults_AppliesExpectedDefaults(t *testing.T) {
	cfg := Config{Database: "app"}

	cfg.withDefaults()

	if cfg.URI != defaultURI {
		t.Fatalf("expected default URI %q, got %q", defaultURI, cfg.URI)
	}
	if cfg.ConnectTimeout != defaultConnectTimeout {
		t.Fatalf("expected connect timeout %s, got %s", defaultConnectTimeout, cfg.ConnectTimeout)
	}
	if cfg.PingTimeout != defaultPingTimeout {
		t.Fatalf("expected ping timeout %s, got %s", defaultPingTimeout, cfg.PingTimeout)
	}
	if cfg.DisconnectTimeout != defaultDisconnectTimeout {
		t.Fatalf("expected disconnect timeout %s, got %s", defaultDisconnectTimeout, cfg.DisconnectTimeout)
	}
	if cfg.ServerSelectionTimeout != defaultServerSelectionTimeout {
		t.Fatalf("expected server selection timeout %s, got %s", defaultServerSelectionTimeout, cfg.ServerSelectionTimeout)
	}
	if cfg.MaxPoolSize != defaultMaxPoolSize {
		t.Fatalf("expected max pool size %d, got %d", defaultMaxPoolSize, cfg.MaxPoolSize)
	}
	if cfg.MinPoolSize != 0 {
		t.Fatalf("expected min pool size 0, got %d", cfg.MinPoolSize)
	}
	if cfg.MaxConnIdleTime != defaultMaxConnIdleTime {
		t.Fatalf("expected max conn idle time %s, got %s", defaultMaxConnIdleTime, cfg.MaxConnIdleTime)
	}
}

func TestConfigWithDefaults_PreservesConfiguredValues(t *testing.T) {
	cfg := Config{
		URI:                    "mongodb://db:27017",
		Database:               "custom",
		ConnectTimeout:         3 * time.Second,
		PingTimeout:            4 * time.Second,
		DisconnectTimeout:      6 * time.Second,
		ServerSelectionTimeout: 2 * time.Second,
		MaxPoolSize:            50,
		MinPoolSize:            10,
		MaxConnIdleTime:        20 * time.Minute,
		AppName:                "keel-mongo-test",
	}

	cfg.withDefaults()

	if cfg.URI != "mongodb://db:27017" {
		t.Fatalf("expected URI to be preserved, got %q", cfg.URI)
	}
	if cfg.ConnectTimeout != 3*time.Second {
		t.Fatalf("expected connect timeout to be preserved, got %s", cfg.ConnectTimeout)
	}
	if cfg.PingTimeout != 4*time.Second {
		t.Fatalf("expected ping timeout to be preserved, got %s", cfg.PingTimeout)
	}
	if cfg.DisconnectTimeout != 6*time.Second {
		t.Fatalf("expected disconnect timeout to be preserved, got %s", cfg.DisconnectTimeout)
	}
	if cfg.ServerSelectionTimeout != 2*time.Second {
		t.Fatalf("expected server selection timeout to be preserved, got %s", cfg.ServerSelectionTimeout)
	}
	if cfg.MaxPoolSize != 50 {
		t.Fatalf("expected max pool size to be preserved, got %d", cfg.MaxPoolSize)
	}
	if cfg.MinPoolSize != 10 {
		t.Fatalf("expected min pool size to be preserved, got %d", cfg.MinPoolSize)
	}
	if cfg.MaxConnIdleTime != 20*time.Minute {
		t.Fatalf("expected max conn idle time to be preserved, got %s", cfg.MaxConnIdleTime)
	}
}

func TestConfigWithDefaults_ClampsMinPoolToMaxPool(t *testing.T) {
	cfg := Config{Database: "app", MaxPoolSize: 5, MinPoolSize: 10}

	cfg.withDefaults()

	if cfg.MinPoolSize != 5 {
		t.Fatalf("expected min pool size clamped to 5, got %d", cfg.MinPoolSize)
	}
}

func TestConfigValidate_RequiresDatabase(t *testing.T) {
	cfg := Config{}
	cfg.withDefaults()

	err := cfg.validate()
	if err == nil {
		t.Fatal("expected validation error for missing database")
	}
}

func TestBuildClientOptions_MergesConfiguredOptions(t *testing.T) {
	cfg := Config{
		URI:                    "mongodb://localhost:27017",
		Database:               "app",
		ServerSelectionTimeout: 3 * time.Second,
		MaxPoolSize:            80,
		MinPoolSize:            10,
		MaxConnIdleTime:        10 * time.Minute,
		AppName:                "ss-keel-mongo-tests",
		ClientOptions:          options.Client().SetRetryWrites(false),
	}

	opts := buildClientOptions(cfg)

	if opts.MaxPoolSize == nil || *opts.MaxPoolSize != 80 {
		t.Fatalf("unexpected max pool size option: %#v", opts.MaxPoolSize)
	}
	if opts.MinPoolSize == nil || *opts.MinPoolSize != 10 {
		t.Fatalf("unexpected min pool size option: %#v", opts.MinPoolSize)
	}
	if opts.ServerSelectionTimeout == nil || *opts.ServerSelectionTimeout != 3*time.Second {
		t.Fatalf("unexpected server selection timeout option: %#v", opts.ServerSelectionTimeout)
	}
	if opts.MaxConnIdleTime == nil || *opts.MaxConnIdleTime != 10*time.Minute {
		t.Fatalf("unexpected max conn idle time option: %#v", opts.MaxConnIdleTime)
	}
	if opts.AppName == nil || *opts.AppName != "ss-keel-mongo-tests" {
		t.Fatalf("unexpected app name option: %#v", opts.AppName)
	}
	if opts.RetryWrites == nil || *opts.RetryWrites {
		t.Fatalf("expected retry writes false from merged options, got %#v", opts.RetryWrites)
	}
}

func TestClientClose_NilSafe(t *testing.T) {
	var client *Client
	if err := client.Close(); err != nil {
		t.Fatalf("expected nil client close to succeed, got %v", err)
	}
}

func TestConfigValidate_ValidDatabaseReturnsNil(t *testing.T) {
	cfg := Config{Database: "myapp"}
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected nil error for valid database, got %v", err)
	}
}

func TestBuildClientOptions_WithoutAppName(t *testing.T) {
	cfg := Config{
		URI:                    "mongodb://localhost:27017",
		Database:               "app",
		ServerSelectionTimeout: 3 * time.Second,
		MaxPoolSize:            10,
		MaxConnIdleTime:        5 * time.Minute,
		// AppName intentionally empty
	}

	opts := buildClientOptions(cfg)

	if opts.AppName != nil {
		t.Fatalf("expected AppName to be nil when not configured, got %q", *opts.AppName)
	}
}
