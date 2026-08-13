package query

import (
	"context"
	"testing"

	"github/master_go_cqrs/services/settlement/internal/domain"
)

type settlementRepoStub struct {
	agentID string
	balance *domain.AgentBalance
}

func (r *settlementRepoStub) GetBalance(ctx context.Context, agentID string) (*domain.AgentBalance, error) {
	r.agentID = agentID
	return r.balance, nil
}

func (r *settlementRepoStub) GetBalanceForUpdate(ctx context.Context, agentID string) (*domain.AgentBalance, error) {
	return r.balance, nil
}

func (r *settlementRepoStub) SaveBalance(ctx context.Context, bal *domain.AgentBalance) error {
	return nil
}

func (r *settlementRepoStub) CreateCommissionLog(ctx context.Context, log *domain.CommissionLog) error {
	return nil
}

func TestGetBalanceHandlerDelegatesToRepository(t *testing.T) {
	want := &domain.AgentBalance{AgentID: "agent-1", ActiveBalance: 200, CreditLimit: 1000}
	repo := &settlementRepoStub{balance: want}
	handler := NewGetBalanceHandler(repo)

	got, err := handler.Handle(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if repo.agentID != "agent-1" {
		t.Fatalf("expected repository lookup with agent id, got %q", repo.agentID)
	}
	if got != want {
		t.Fatal("expected repository result to be returned")
	}
}
