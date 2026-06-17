// File: internal/platform/outbox/fx.go
// Description: Fx dependency injection module configuration for the transactional outbox package.

package outbox

import (
	"github/master_go_cqrs/internal/platform/postgres"

	"go.uber.org/fx"
)

var Module = fx.Module("outbox",
	fx.Provide(
		func(db *postgres.DB) Repository { return NewRepository(db.DB) },
		NewPublisher,
	),
	fx.Invoke(func(publisher *Publisher, lifecycle fx.Lifecycle) { publisher.Start(lifecycle) }),
)
