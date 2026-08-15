package postgres

import "github/master_go_cqrs/services/agent/internal/domain"

func configToDomain(r *AgentConfigRow) *domain.AgentConfig {
	return &domain.AgentConfig{
		AgentID: r.AgentID, TenantID: r.TenantID, CallbackURL: r.CallbackURL,
		CallbackHost: r.CallbackHost, CallbackStatus: r.CallbackStatus,
		CallbackVerifiedAt: r.CallbackVerifiedAt, ApiKey: r.ApiKey,
		CallbackChallengeID: r.CallbackChallengeID, CallbackChallengeDigest: r.CallbackChallengeDigest,
		CallbackChallengeIssuedAt:  r.CallbackChallengeIssuedAt,
		CallbackChallengeExpiresAt: r.CallbackChallengeExpiresAt,
		CallbackChallengeUsedAt:    r.CallbackChallengeUsedAt,
		AllowedIPs:                 r.AllowedIPs, UpdatedAt: r.UpdatedAt,
	}
}
func configToRow(e *domain.AgentConfig) *AgentConfigRow {
	return &AgentConfigRow{
		AgentID: e.AgentID, TenantID: e.TenantID, CallbackURL: e.CallbackURL,
		CallbackHost: e.CallbackHost, CallbackStatus: e.CallbackStatus,
		CallbackVerifiedAt: e.CallbackVerifiedAt, ApiKey: e.ApiKey,
		CallbackChallengeID: e.CallbackChallengeID, CallbackChallengeDigest: e.CallbackChallengeDigest,
		CallbackChallengeIssuedAt:  e.CallbackChallengeIssuedAt,
		CallbackChallengeExpiresAt: e.CallbackChallengeExpiresAt,
		CallbackChallengeUsedAt:    e.CallbackChallengeUsedAt,
		AllowedIPs:                 e.AllowedIPs, UpdatedAt: e.UpdatedAt,
	}
}
func auditLogToRow(e *domain.AuditLog) *AuditLogRow {
	return &AuditLogRow{ID: e.ID, AgentID: e.AgentID, TenantID: e.TenantID, Action: e.Action, Details: e.Details, CreatedAt: e.CreatedAt}
}
func callbackToDomain(r *CallbackDeliveryRow) *domain.CallbackDelivery {
	return &domain.CallbackDelivery{
		TransactionID: r.TransactionID, AgentID: r.AgentID, TenantID: r.TenantID,
		EventID: r.EventID, CallbackURL: r.CallbackURL, Payload: r.Payload, KeyVersion: r.KeyVersion,
		Status: r.Status, Attempts: r.Attempts, GenerationAttempts: r.GenerationAttempts,
		NextAttemptAt: r.NextAttemptAt, LastAttemptAt: r.LastAttemptAt,
		LeaseOwner: r.LeaseOwner, LeaseToken: r.LeaseToken, LeaseExpiresAt: r.LeaseExpiresAt,
		ReplayCount: r.ReplayCount, OutcomeGeneration: r.OutcomeGeneration,
		LastError: r.LastError, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, TerminalAt: r.TerminalAt,
	}
}
func callbackToRow(e *domain.CallbackDelivery) *CallbackDeliveryRow {
	return &CallbackDeliveryRow{
		TransactionID: e.TransactionID, AgentID: e.AgentID, TenantID: e.TenantID,
		EventID: e.EventID, CallbackURL: e.CallbackURL, Payload: e.Payload, KeyVersion: e.KeyVersion,
		Status: e.Status, Attempts: e.Attempts, GenerationAttempts: e.GenerationAttempts,
		NextAttemptAt: e.NextAttemptAt, LastAttemptAt: e.LastAttemptAt,
		LeaseOwner: e.LeaseOwner, LeaseToken: e.LeaseToken, LeaseExpiresAt: e.LeaseExpiresAt,
		ReplayCount: e.ReplayCount, OutcomeGeneration: e.OutcomeGeneration,
		LastError: e.LastError, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, TerminalAt: e.TerminalAt,
	}
}
