package postgres

import (
	"context"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/payment/internal/application/port"

	"gorm.io/gorm"
)

type GormUnitOfWork struct{ db *gorm.DB }

func NewUnitOfWork(db *platformpostgres.DB) port.UnitOfWork { return &GormUnitOfWork{db: db.DB} }

func (u *GormUnitOfWork) Do(ctx context.Context, fn func(context.Context) error) error {
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(platformpostgres.WithTx(ctx, tx))
	})
}
