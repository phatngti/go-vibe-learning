package command

import (
	"context"

	"github/master_go_cqrs/services/agent/internal/application/dto"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/domain"
)

type RecordAuditLogHandler struct {
	repo port.AgentRepository
}

func NewRecordAuditLogHandler(repo port.AgentRepository) *RecordAuditLogHandler {
	return &RecordAuditLogHandler{repo: repo}
}

func (h *RecordAuditLogHandler) Handle(ctx context.Context, cmd *dto.RecordAuditLog) error {
	log := &domain.AuditLog{
		AgentID: cmd.AgentID,
		Action:  cmd.Action,
		Details: cmd.Details,
	}
	return h.repo.CreateAuditLog(ctx, log)
}
