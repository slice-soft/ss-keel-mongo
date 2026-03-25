package mongo

import (
	"errors"
	"strings"
	"time"

	"github.com/slice-soft/ss-keel-core/contracts"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	defaultURI                    = "mongodb://localhost:27017"
	defaultConnectTimeout         = 10 * time.Second
	defaultPingTimeout            = 2 * time.Second
	defaultDisconnectTimeout      = 5 * time.Second
	defaultServerSelectionTimeout = 5 * time.Second
	defaultMaxPoolSize            = uint64(25)
	defaultMaxConnIdleTime        = 15 * time.Minute
)

// Config controls connection and pool behavior for MongoDB.
type Config struct {
	URI                    string `keel:"mongo.uri,required"`
	Database               string `keel:"mongo.database,required"`
	Production             bool
	ConnectTimeout         time.Duration
	PingTimeout            time.Duration
	DisconnectTimeout      time.Duration
	ServerSelectionTimeout time.Duration
	MaxPoolSize            uint64
	MinPoolSize            uint64
	MaxConnIdleTime        time.Duration
	AppName                string `keel:"app.name"`
	SkipPing               bool
	ClientOptions          *options.ClientOptions
	Logger                 contracts.Logger
}

func (cfg *Config) withDefaults() {
	if strings.TrimSpace(cfg.URI) == "" {
		cfg.URI = defaultURI
	}

	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = defaultConnectTimeout
	}

	if cfg.PingTimeout <= 0 {
		cfg.PingTimeout = defaultPingTimeout
	}

	if cfg.DisconnectTimeout <= 0 {
		cfg.DisconnectTimeout = defaultDisconnectTimeout
	}

	if cfg.ServerSelectionTimeout <= 0 {
		cfg.ServerSelectionTimeout = defaultServerSelectionTimeout
	}

	if cfg.MaxPoolSize == 0 {
		cfg.MaxPoolSize = defaultMaxPoolSize
	}

	if cfg.MinPoolSize > cfg.MaxPoolSize {
		cfg.MinPoolSize = cfg.MaxPoolSize
	}

	if cfg.MaxConnIdleTime <= 0 {
		cfg.MaxConnIdleTime = defaultMaxConnIdleTime
	}
}

func (cfg Config) validate() error {
	if strings.TrimSpace(cfg.Database) == "" {
		return errors.New("database is required")
	}

	return nil
}

func buildClientOptions(cfg Config) *options.ClientOptions {
	base := options.Client().
		ApplyURI(cfg.URI).
		SetServerSelectionTimeout(cfg.ServerSelectionTimeout).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetMinPoolSize(cfg.MinPoolSize).
		SetMaxConnIdleTime(cfg.MaxConnIdleTime)

	if appName := strings.TrimSpace(cfg.AppName); appName != "" {
		base.SetAppName(appName)
	}

	if cfg.ClientOptions == nil {
		return base
	}

	return options.MergeClientOptions(base, cfg.ClientOptions)
}
