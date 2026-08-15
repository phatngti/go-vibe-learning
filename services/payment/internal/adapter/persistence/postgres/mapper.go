package postgres

import "github/master_go_cqrs/services/payment/internal/domain"

func toDomain(row *TransactionRow) *domain.Transaction {
	result := &domain.Transaction{
		ID:                    row.ID,
		AgentID:               row.AgentID,
		CustomerID:            row.CustomerID,
		Amount:                row.Amount,
		Currency:              row.Currency,
		Status:                domain.TransactionState(row.Status),
		CallbackURL:           row.CallbackURL,
		RequestIdempotencyKey: row.RequestIdempotencyKey,
		RequestFingerprint:    row.RequestFingerprint,
		IdempotencyKey:        row.IdempotencyKey,
		ProviderReference:     row.ProviderReference,
		Provider:              row.Provider,
		ProviderStatus:        row.ProviderStatus,
		CallbackEndpointID:    row.CallbackEndpointID,
		LastError:             row.LastError,
		ProcessingStartedAt:   row.ProcessingStartedAt,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
	if row.AmountMinor != nil {
		result.AmountMinor = *row.AmountMinor
	}
	if row.CurrencyExponent != nil {
		result.CurrencyExponent = *row.CurrencyExponent
	}
	return result
}

func toRow(entity *domain.Transaction) *TransactionRow {
	row := &TransactionRow{
		ID:                    entity.ID,
		AgentID:               entity.AgentID,
		CustomerID:            entity.CustomerID,
		Amount:                entity.Amount,
		Currency:              entity.Currency,
		Status:                string(entity.Status),
		CallbackURL:           entity.CallbackURL,
		RequestIdempotencyKey: entity.RequestIdempotencyKey,
		RequestFingerprint:    entity.RequestFingerprint,
		IdempotencyKey:        entity.IdempotencyKey,
		ProviderReference:     entity.ProviderReference,
		Provider:              entity.Provider,
		ProviderStatus:        entity.ProviderStatus,
		CallbackEndpointID:    entity.CallbackEndpointID,
		LastError:             entity.LastError,
		ProcessingStartedAt:   entity.ProcessingStartedAt,
		CreatedAt:             entity.CreatedAt,
		UpdatedAt:             entity.UpdatedAt,
	}
	if entity.AmountMinor != 0 {
		amountMinor := entity.AmountMinor
		exponent := entity.CurrencyExponent
		row.AmountMinor = &amountMinor
		row.CurrencyExponent = &exponent
	}
	return row
}
