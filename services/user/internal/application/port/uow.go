package port

import "context"

// UnitOfWork provides transactional boundaries without leaking
// infrastructure details (e.g. *gorm.DB) into the domain or application layers.
type UnitOfWork interface {
	// Do executes fn within a transaction. If fn returns an error the
	// transaction is rolled back; otherwise it is committed.
	// The txCtx passed to fn carries an opaque transaction handle that
	// adapter-layer repositories can extract internally.
	Do(ctx context.Context, fn func(txCtx context.Context) error) error
}
