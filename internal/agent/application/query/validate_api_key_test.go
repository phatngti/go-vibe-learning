package query

import (
	"context"
	"testing"

	"github/master_go_cqrs/internal/agent/domain"

	"gorm.io/gorm"
)

type agentRepoStub struct {
	apiKey string
	cfg    *domain.AgentConfig
}

func (r *agentRepoStub) GetConfig(ctx context.Context, agentID string) (*domain.AgentConfig, error) {
	return nil, nil
}

func (r *agentRepoStub) GetConfigByApiKey(ctx context.Context, apiKey string) (*domain.AgentConfig, error) {
	r.apiKey = apiKey
	return r.cfg, nil
}

func (r *agentRepoStub) SaveConfig(ctx context.Context, db *gorm.DB, cfg *domain.AgentConfig) error {
	return nil
}

func (r *agentRepoStub) CreateAuditLog(ctx context.Context, db *gorm.DB, log *domain.AuditLog) error {
	return nil
}

func (r *agentRepoStub) GetCallbackDelivery(context.Context, string) (*domain.CallbackDelivery, error) {
	return nil, nil
}

func (r *agentRepoStub) SaveCallbackDelivery(context.Context, *gorm.DB, *domain.CallbackDelivery) error {
	return nil
}

func TestValidateAPIKeyHandlerDelegatesToRepository(t *testing.T) {
	want := &domain.AgentConfig{AgentID: "agent-1", ApiKey: "agg_key_test"}
	repo := &agentRepoStub{cfg: want}
	handler := NewValidateAPIKeyHandler(repo)

	got, err := handler.Handle(context.Background(), "agg_key_test")
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if repo.apiKey != "agg_key_test" {
		t.Fatalf("expected repository lookup with api key, got %q", repo.apiKey)
	}
	if got != want {
		t.Fatal("expected repository result to be returned")
	}
}
