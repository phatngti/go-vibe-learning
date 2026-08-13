package postgres

import "github/master_go_cqrs/services/payment/internal/domain"

func toDomain(row *TransactionRow) *domain.Transaction {
	return &domain.Transaction{
		ID:          row.ID,
		AgentID:     row.AgentID,
		CustomerID:  row.CustomerID,
		Amount:      row.Amount,
		Currency:    row.Currency,
		Status:      domain.TransactionState(row.Status),
		CallbackURL: row.CallbackURL,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func toRow(entity *domain.Transaction) *TransactionRow {
	return &TransactionRow{
		ID:          entity.ID,
		AgentID:     entity.AgentID,
		CustomerID:  entity.CustomerID,
		Amount:      entity.Amount,
		Currency:    entity.Currency,
		Status:      string(entity.Status),
		CallbackURL: entity.CallbackURL,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}
}
