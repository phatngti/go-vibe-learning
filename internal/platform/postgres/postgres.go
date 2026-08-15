// File: internal/platform/postgres/postgres.go
// Description: GORM PostgreSQL database connection setup, configuration, and Fx lifecycle hooks.

package postgres

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/observability/logger"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DB struct {
	DB *gorm.DB

	Logger logger.Factory
}

type transactionContextKey struct{}

// WithTx binds a GORM transaction to a context without exposing a
// service-specific persistence adapter to transport or application packages.
func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, transactionContextKey{}, tx)
}

// FromContext returns the bound transaction, or fallback when the context is
// not transactional.
func FromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(transactionContextKey{}).(*gorm.DB); ok {
		return tx
	}
	return fallback
}

var Module = fx.Module("postgres",
	fx.Provide(New),
)

func New(lifecycle fx.Lifecycle, cfg *config.Postgres, logger logger.Factory) (*DB, error) {
	logger.Bg().Info("Creating Postgres connection")
	connectTimeout := cfg.ConnectTimeout
	if connectTimeout <= 0 {
		connectTimeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	db, err := connect(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres connection: %w", err)
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return db.Close()
		},
	})

	return db, nil
}

func connect(ctx context.Context, cfg *config.Postgres, logger logger.Factory) (*DB, error) {
	dsn, err := postgresDSN(cfg)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 50
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle < 0 {
		maxIdle = 0
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetMaxOpenConns(maxOpen)
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	logger.Bg().Info("Connected to Postgres database")

	return &DB{
		DB:     db,
		Logger: logger,
	}, nil
}

func postgresDSN(cfg *config.Postgres) (string, error) {
	sslMode := strings.ToLower(strings.TrimSpace(cfg.SSLMode))
	if sslMode == "" {
		sslMode = "disable"
	}
	switch sslMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return "", fmt.Errorf("unsupported postgres sslmode %q", cfg.SSLMode)
	}
	if (strings.TrimSpace(cfg.SSLCert) == "") != (strings.TrimSpace(cfg.SSLKey) == "") {
		return "", fmt.Errorf("postgres SSL certificate and key must be configured together")
	}

	port := strings.TrimSpace(cfg.Port)
	if port == "" {
		port = "5432"
	}
	dsnURL := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(strings.TrimSpace(cfg.Host), port),
		Path:   strings.TrimSpace(cfg.DBName),
	}
	if cfg.Password == "" {
		dsnURL.User = url.User(cfg.User)
	} else {
		dsnURL.User = url.UserPassword(cfg.User, cfg.Password)
	}
	query := dsnURL.Query()
	query.Set("sslmode", sslMode)
	query.Set("timezone", "UTC")
	if cfg.ApplicationName != "" {
		query.Set("application_name", cfg.ApplicationName)
	}
	if cfg.ConnectTimeout > 0 {
		seconds := int64((cfg.ConnectTimeout + time.Second - 1) / time.Second)
		query.Set("connect_timeout", strconv.FormatInt(seconds, 10))
	}
	if cfg.StatementTimeout > 0 {
		query.Set("statement_timeout", strconv.FormatInt(cfg.StatementTimeout.Milliseconds(), 10))
	}
	if cfg.SSLRootCert != "" {
		query.Set("sslrootcert", cfg.SSLRootCert)
	}
	if cfg.SSLCert != "" {
		query.Set("sslcert", cfg.SSLCert)
		query.Set("sslkey", cfg.SSLKey)
	}
	dsnURL.RawQuery = query.Encode()
	return dsnURL.String(), nil
}

func (p *DB) Close() error {
	p.Logger.Bg().Info("Closing Postgres connection")
	if p.DB != nil {
		sqlDB, err := p.DB.DB()
		if err != nil {
			p.Logger.Bg().Error("Failed to get database connection", zap.Error(err))
			return err
		}
		err = sqlDB.Close()
		if err != nil {
			p.Logger.Bg().Error("Failed to close database connection", zap.Error(err))
			return err
		}
	}
	return nil
}
