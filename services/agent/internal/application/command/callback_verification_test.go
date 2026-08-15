package command

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github/master_go_cqrs/services/agent/internal/application/dto"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/config"
	"github/master_go_cqrs/services/agent/internal/domain"
)

type callbackTxKey struct{}

type callbackUOWStub struct{}

func (callbackUOWStub) Do(ctx context.Context, fn func(context.Context) error) error {
	return fn(context.WithValue(ctx, callbackTxKey{}, true))
}

type callbackRepoStub struct {
	config  *domain.AgentConfig
	audits  []*domain.AuditLog
	getInTx bool
}

func (r *callbackRepoStub) GetConfig(ctx context.Context, _ string) (*domain.AgentConfig, error) {
	r.getInTx, _ = ctx.Value(callbackTxKey{}).(bool)
	copy := *r.config
	return &copy, nil
}
func (*callbackRepoStub) GetConfigByApiKey(context.Context, string) (*domain.AgentConfig, error) {
	return nil, errors.New("not implemented")
}
func (r *callbackRepoStub) SaveConfig(_ context.Context, cfg *domain.AgentConfig) error {
	copy := *cfg
	r.config = &copy
	return nil
}
func (r *callbackRepoStub) CreateAuditLog(_ context.Context, audit *domain.AuditLog) error {
	r.audits = append(r.audits, audit)
	return nil
}
func (*callbackRepoStub) GetCallbackDelivery(context.Context, string) (*domain.CallbackDelivery, error) {
	return nil, nil
}
func (*callbackRepoStub) SaveCallbackDelivery(context.Context, *domain.CallbackDelivery) error {
	return nil
}

type callbackValidatorStub struct {
	endpoint domain.ValidatedCallbackEndpoint
}

func (v callbackValidatorStub) Validate(context.Context, string) (domain.ValidatedCallbackEndpoint, error) {
	return v.endpoint, nil
}

type callbackChallengeStub struct {
	secret domain.CallbackChallengeSecret
}

func (s callbackChallengeStub) Generate() (domain.CallbackChallengeSecret, error) {
	return s.secret, nil
}
func (callbackChallengeStub) Digest(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

type callbackProverStub struct {
	called bool
	inTx   bool
}

func (p *callbackProverStub) Verify(ctx context.Context, _, _ string) error {
	p.called = true
	p.inTx, _ = ctx.Value(callbackTxKey{}).(bool)
	return nil
}

func TestConfigureCallbackCreatesPendingOneTimeChallenge(t *testing.T) {
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	repo := &callbackRepoStub{config: &domain.AgentConfig{AgentID: "agent-1", TenantID: "tenant-1"}}
	secret := domain.CallbackChallengeSecret{ID: "challenge-1", Token: "plain-token", Digest: callbackChallengeStub{}.Digest("plain-token")}
	handler := NewConfigureCallbackHandler(
		callbackUOWStub{}, repo,
		callbackValidatorStub{endpoint: domain.ValidatedCallbackEndpoint{URL: "https://merchant.example/callback", Hostname: "merchant.example", Verified: true}},
		callbackChallengeStub{secret: secret},
		&config.Config{Callback: config.Callback{ChallengeTTL: time.Hour}},
	)
	handler.now = func() time.Time { return now }

	result, err := handler.Handle(context.Background(), &dto.ConfigureCallback{
		AgentID: "agent-1", TenantID: "tenant-1", CallbackURL: "https://merchant.example/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Config.CallbackStatus != domain.CallbackStatusPending || result.Config.CallbackVerifiedAt != nil {
		t.Fatalf("configured callback was activated without proof: %+v", result.Config)
	}
	if result.Challenge.Token != "plain-token" || result.Challenge.ProofURL != "https://merchant.example/.well-known/cqrs-callback-verification/challenge-1" {
		t.Fatalf("challenge = %+v", result.Challenge)
	}
	if repo.config.CallbackChallengeDigest != secret.Digest || repo.config.CallbackChallengeDigest == result.Challenge.Token {
		t.Fatal("repository did not retain only the challenge digest")
	}
	if !repo.getInTx {
		t.Fatal("callback configuration was loaded outside the write transaction")
	}
}

func TestGenerateAPIKeyLoadsAndWritesConfigurationInOneTransaction(t *testing.T) {
	repo := &callbackRepoStub{config: &domain.AgentConfig{AgentID: "agent-1", TenantID: "tenant-1"}}
	handler := NewGenerateAPIKeyHandler(callbackUOWStub{}, repo)

	configured, err := handler.Handle(context.Background(), &dto.GenerateAPIKey{AgentID: "agent-1", TenantID: "tenant-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !repo.getInTx {
		t.Fatal("API key configuration was loaded outside the write transaction")
	}
	if configured.ApiKey == "" || configured.ApiKey != repo.config.ApiKey {
		t.Fatalf("API key was not persisted: configured=%q persisted=%q", configured.ApiKey, repo.config.ApiKey)
	}
	if len(repo.audits) != 1 || repo.audits[0].Action != "GENERATE_API_KEY" {
		t.Fatalf("audits = %+v", repo.audits)
	}
}

func TestVerifyCallbackActivatesAfterRemoteProofOutsideTransaction(t *testing.T) {
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	token := "plain-token"
	repo := &callbackRepoStub{config: &domain.AgentConfig{
		AgentID: "agent-1", TenantID: "tenant-1", CallbackURL: "https://merchant.example/callback",
		CallbackHost: "merchant.example", CallbackStatus: domain.CallbackStatusPending,
		CallbackChallengeID: "challenge-1", CallbackChallengeDigest: callbackChallengeStub{}.Digest(token),
		CallbackChallengeExpiresAt: timePointer(now.Add(time.Hour)),
	}}
	prover := &callbackProverStub{}
	handler := NewVerifyCallbackHandler(
		callbackUOWStub{}, repo,
		callbackValidatorStub{endpoint: domain.ValidatedCallbackEndpoint{URL: repo.config.CallbackURL, Hostname: "merchant.example"}},
		callbackChallengeStub{}, prover,
	)
	handler.now = func() time.Time { return now }

	verified, err := handler.Handle(context.Background(), &dto.VerifyCallback{
		AgentID: "agent-1", TenantID: "tenant-1", ChallengeID: "challenge-1", ChallengeToken: token,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !prover.called || prover.inTx {
		t.Fatalf("remote proof called=%v inTx=%v", prover.called, prover.inTx)
	}
	if verified.CallbackStatus != domain.CallbackStatusVerified || verified.CallbackVerifiedAt == nil || verified.CallbackChallengeUsedAt == nil {
		t.Fatalf("verified callback = %+v", verified)
	}
	if len(repo.audits) != 1 || repo.audits[0].Action != "VERIFY_CALLBACK_OWNERSHIP" {
		t.Fatalf("audits = %+v", repo.audits)
	}
}

func TestVerifyCallbackRejectsExpiredAndReplayedChallenges(t *testing.T) {
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	token := "plain-token"
	base := domain.AgentConfig{
		AgentID: "agent-1", TenantID: "tenant-1", CallbackURL: "https://merchant.example/callback",
		CallbackStatus: domain.CallbackStatusPending, CallbackChallengeID: "challenge-1",
		CallbackChallengeDigest:    callbackChallengeStub{}.Digest(token),
		CallbackChallengeExpiresAt: timePointer(now.Add(-time.Second)),
	}
	for name, testCase := range map[string]struct {
		mutate func(*domain.AgentConfig)
		want   error
	}{
		"expired": {mutate: func(*domain.AgentConfig) {}, want: domain.ErrCallbackChallengeExpired},
		"replayed": {mutate: func(cfg *domain.AgentConfig) {
			cfg.CallbackChallengeExpiresAt = timePointer(now.Add(time.Hour))
			cfg.CallbackChallengeUsedAt = timePointer(now.Add(-time.Minute))
		}, want: domain.ErrCallbackChallengeReplayed},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := base
			testCase.mutate(&cfg)
			repo := &callbackRepoStub{config: &cfg}
			prover := &callbackProverStub{}
			handler := NewVerifyCallbackHandler(callbackUOWStub{}, repo, callbackValidatorStub{}, callbackChallengeStub{}, prover)
			handler.now = func() time.Time { return now }
			_, err := handler.Handle(context.Background(), &dto.VerifyCallback{
				AgentID: "agent-1", TenantID: "tenant-1", ChallengeID: "challenge-1", ChallengeToken: token,
			})
			if !errors.Is(err, testCase.want) || prover.called {
				t.Fatalf("error = %v, prover called = %v; want %v", err, prover.called, testCase.want)
			}
		})
	}
}

func timePointer(value time.Time) *time.Time { return &value }

var _ port.UnitOfWork = callbackUOWStub{}
var _ port.AgentRepository = (*callbackRepoStub)(nil)
