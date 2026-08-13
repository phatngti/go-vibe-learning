package postgres

import "github/master_go_cqrs/services/agent/internal/domain"

func configToDomain(r *AgentConfigRow) *domain.AgentConfig {
	return &domain.AgentConfig{AgentID: r.AgentID, CallbackURL: r.CallbackURL, ApiKey: r.ApiKey, AllowedIPs: r.AllowedIPs, UpdatedAt: r.UpdatedAt}
}
func configToRow(e *domain.AgentConfig) *AgentConfigRow {
	return &AgentConfigRow{AgentID: e.AgentID, CallbackURL: e.CallbackURL, ApiKey: e.ApiKey, AllowedIPs: e.AllowedIPs, UpdatedAt: e.UpdatedAt}
}
func auditLogToRow(e *domain.AuditLog) *AuditLogRow {
	return &AuditLogRow{ID: e.ID, AgentID: e.AgentID, Action: e.Action, Details: e.Details, CreatedAt: e.CreatedAt}
}
func callbackToDomain(r *CallbackDeliveryRow) *domain.CallbackDelivery {
	return &domain.CallbackDelivery{TransactionID: r.TransactionID, AgentID: r.AgentID, CallbackURL: r.CallbackURL, Status: r.Status, Attempts: r.Attempts, LastError: r.LastError, UpdatedAt: r.UpdatedAt}
}
func callbackToRow(e *domain.CallbackDelivery) *CallbackDeliveryRow {
	return &CallbackDeliveryRow{TransactionID: e.TransactionID, AgentID: e.AgentID, CallbackURL: e.CallbackURL, Status: e.Status, Attempts: e.Attempts, LastError: e.LastError, UpdatedAt: e.UpdatedAt}
}
