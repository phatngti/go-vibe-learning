package agent

import (
	"github/master_go_cqrs/internal/agent/application/command"
	"github/master_go_cqrs/internal/agent/application/query"
	"github/master_go_cqrs/internal/agent/delivery/grpc"
	"github/master_go_cqrs/internal/agent/delivery/http"
	"github/master_go_cqrs/internal/agent/repository/postgres"
	"github/master_go_cqrs/internal/httpapi/route"

	"go.uber.org/fx"
)

var Module = fx.Module("agent",
	fx.Provide(
		postgres.NewAgentRepository,
		command.NewConfigureCallbackHandler,
		command.NewGenerateAPIKeyHandler,
		command.NewRecordAuditLogHandler,
		query.NewValidateAPIKeyHandler,
		route.AsRegistrar(http.NewHandler),
		grpc.NewServerHandler,
	),
	fx.Invoke(grpc.RequireServerHandler),
)
