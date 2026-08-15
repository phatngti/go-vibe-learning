package model

import (
	"testing"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/orchestrator/internal/payment"
)

func TestMoneyV2SurvivesSagaPersistenceMapping(t *testing.T) {
	authoritative := sharedmoney.Money{AmountMinor: 10_001, Currency: "USD", Exponent: 2}
	snapshot := payment.Snapshot{
		TransactionID: "tx-money", AgentID: "agent-1", Money: &authoritative,
		Amount: 999.99, Currency: "USD", State: payment.StateWaitingAffiliate,
	}
	var stored Model
	ApplySnapshot(&stored, snapshot)
	if stored.AmountMinor == nil || *stored.AmountMinor != authoritative.AmountMinor || stored.CurrencyExponent == nil || *stored.CurrencyExponent != authoritative.Exponent {
		t.Fatalf("stored model = %+v", stored)
	}
	restored := stored.Snapshot()
	if restored.Money == nil || *restored.Money != authoritative {
		t.Fatalf("restored snapshot = %+v", restored)
	}
}
