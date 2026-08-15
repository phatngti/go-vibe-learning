package postgres

import (
	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/settlement/internal/domain"
)

func balanceToDomain(row *AgentBalanceRow) *domain.AgentBalance {
	balance := &domain.AgentBalance{
		AgentID: row.AgentID, ActiveBalance: row.ActiveBalance,
		CreditLimit: row.CreditLimit, UpdatedAt: row.UpdatedAt,
	}
	if row.ActiveBalanceMinor != nil && row.CreditLimitMinor != nil && row.Currency != nil && row.CurrencyExponent != nil {
		balance.ActiveBalanceMoney = &sharedmoney.Money{AmountMinor: *row.ActiveBalanceMinor, Currency: *row.Currency, Exponent: *row.CurrencyExponent}
		balance.CreditLimitMoney = &sharedmoney.Money{AmountMinor: *row.CreditLimitMinor, Currency: *row.Currency, Exponent: *row.CurrencyExponent}
	}
	return balance
}

func balanceToRow(e *domain.AgentBalance) *AgentBalanceRow {
	row := &AgentBalanceRow{
		AgentID: e.AgentID, ActiveBalance: e.ActiveBalance,
		CreditLimit: e.CreditLimit, UpdatedAt: e.UpdatedAt,
	}
	if e.ActiveBalanceMoney != nil && e.CreditLimitMoney != nil {
		activeMinor := e.ActiveBalanceMoney.AmountMinor
		creditMinor := e.CreditLimitMoney.AmountMinor
		currency := e.ActiveBalanceMoney.Currency
		exponent := e.ActiveBalanceMoney.Exponent
		row.ActiveBalanceMinor = &activeMinor
		row.CreditLimitMinor = &creditMinor
		row.Currency = &currency
		row.CurrencyExponent = &exponent
	}
	return row
}

func commissionLogToRow(e *domain.CommissionLog) *CommissionLogRow {
	row := &CommissionLogRow{
		ID: e.ID, AgentID: e.AgentID, TransactionID: e.TransactionID,
		TransactionAmount: e.TransactionAmount, CommissionAmount: e.CommissionAmount,
		Rank: e.Rank, CreatedAt: e.CreatedAt,
	}
	if e.TransactionMoney != nil && e.CommissionMoney != nil {
		transactionMinor := e.TransactionMoney.AmountMinor
		commissionMinor := e.CommissionMoney.AmountMinor
		currency := e.TransactionMoney.Currency
		exponent := e.TransactionMoney.Exponent
		row.TransactionAmountMinor = &transactionMinor
		row.CommissionAmountMinor = &commissionMinor
		row.Currency = &currency
		row.CurrencyExponent = &exponent
	}
	return row
}
