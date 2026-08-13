// File: internal/platform/postgres/postgres.go
// Description: GORM PostgreSQL database connection setup, configuration, and Fx lifecycle hooks.

package postgres

import (
	"context"
	"fmt"
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		cfg.Host,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.Port,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	// Set connection pool settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}
	// Adjust these values according to your needs
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Ping the database to ensure the connection is established
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	logger.Bg().Info("Connected to Postgres database")

	return &DB{
		DB:     db,
		Logger: logger,
	}, nil
}

func (p *DB) Close() error {
	p.Logger.Bg().Info("Closing Postgres connection")
	if p.DB != nil {
		sqlDB, err := p.DB.DB()
		if err != nil {
			p.Logger.Bg().Error("Failed to get database connection", zap.Error(err))
			return nil // Return nil to avoid blocking the shutdown process
		}
		err = sqlDB.Close()
		if err != nil {
			p.Logger.Bg().Error("Failed to close database connection", zap.Error(err))
			return nil // Return nil to avoid blocking the shutdown process
		}
	}
	return nil
}
