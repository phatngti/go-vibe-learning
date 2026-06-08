package command

import (
	"context"

	"github/master_go_cqrs/internal/agent/application/dto"
	"github/master_go_cqrs/internal/agent/domain"
)

type RecordAuditLogHandler struct {
	repo domain.AgentRepository
}

func NewRecordAuditLogHandler(repo domain.AgentRepository) *RecordAuditLogHandler {
	return &RecordAuditLogHandler{repo: repo}
}

func (h *RecordAuditLogHandler) Handle(ctx context.Context, cmd *dto.RecordAuditLog) error {
	log := &domain.AuditLog{
		AgentID: cmd.AgentID,
		Action:  cmd.Action,
		Details: cmd.Details,
	}
	return h.repo.CreateAuditLog(ctx, nil, log)
}
