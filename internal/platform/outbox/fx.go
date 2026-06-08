package outbox

import (
	"github/master_go_cqrs/internal/platform/postgres"

	"go.uber.org/fx"
)

var Module = fx.Module("outbox",
	fx.Provide(
		func(db *postgres.DB) Repository {
			return NewRepository(db.DB)
		},
		NewPublisher,
	),
	fx.Invoke(func(p *Publisher, lc fx.Lifecycle) {
		p.Start(lc)
	}),
)
