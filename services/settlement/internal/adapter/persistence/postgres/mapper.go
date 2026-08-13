package postgres

import "github/master_go_cqrs/services/settlement/internal/domain"

func balanceToDomain(row *AgentBalanceRow) *domain.AgentBalance {
	return &domain.AgentBalance{
		AgentID: row.AgentID, ActiveBalance: row.ActiveBalance,
		CreditLimit: row.CreditLimit, UpdatedAt: row.UpdatedAt,
	}
}

func balanceToRow(e *domain.AgentBalance) *AgentBalanceRow {
	return &AgentBalanceRow{
		AgentID: e.AgentID, ActiveBalance: e.ActiveBalance,
		CreditLimit: e.CreditLimit, UpdatedAt: e.UpdatedAt,
	}
}

func commissionLogToRow(e *domain.CommissionLog) *CommissionLogRow {
	return &CommissionLogRow{
		ID: e.ID, AgentID: e.AgentID, TransactionID: e.TransactionID,
		TransactionAmount: e.TransactionAmount, CommissionAmount: e.CommissionAmount,
		Rank: e.Rank, CreatedAt: e.CreatedAt,
	}
}
