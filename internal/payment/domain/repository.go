package domain

import (
	"context"

	"gorm.io/gorm"
)

type TransactionRepository interface {
	Save(ctx context.Context, db *gorm.DB, tx *Transaction) error
	GetByID(ctx context.Context, id string) (*Transaction, error)
	GetByIDWithDB(ctx context.Context, db *gorm.DB, id string) (*Transaction, error)
	UpdateStatus(ctx context.Context, db *gorm.DB, id string, status TransactionState) error
}
