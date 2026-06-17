// File: internal/platform/eventstore/fx.go
// Description: Fx dependency injection module configuration for the eventstore package.

package eventstore

import (
	"github/master_go_cqrs/internal/platform/outbox"
	"github/master_go_cqrs/internal/platform/postgres"

	"go.uber.org/fx"
)

// Module provides a plain eventstore.Repository without outbox relay.
// Services that need relay should use ModuleWithRelay instead.
var Module = fx.Module("eventstore",
	fx.Provide(
		func(db *postgres.DB) Repository {
			return NewRepository(db.DB)
		},
	),
)

// ModuleWithRelay provides an eventstore.Repository that automatically
// forwards matching events to the transactional outbox.
// The caller must supply a RelayConfig via fx.Supply or fx.Provide.
var ModuleWithRelay = fx.Module("eventstore",
	fx.Provide(
		func(db *postgres.DB, outboxRepo outbox.Repository, config RelayConfig) Repository {
			base := NewRepository(db.DB)
			return NewRelayRepository(base, outboxRepo, config)
		},
	),
)
