package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/kafka"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/config"
	"github/master_go_cqrs/services/agent/internal/domain"
)

type callbackAgentRepositoryStub struct{ cfg *domain.AgentConfig }

func (r callbackAgentRepositoryStub) GetConfig(context.Context, string) (*domain.AgentConfig, error) {
	if r.cfg == nil {
		return &domain.AgentConfig{}, nil
	}
	copy := *r.cfg
	return &copy, nil
}
func (callbackAgentRepositoryStub) GetConfigByApiKey(context.Context, string) (*domain.AgentConfig, error) {
	return nil, errors.New("not implemented")
}
func (callbackAgentRepositoryStub) SaveConfig(context.Context, *domain.AgentConfig) error { return nil }
func (callbackAgentRepositoryStub) CreateAuditLog(context.Context, *domain.AuditLog) error {
	return nil
}
func (callbackAgentRepositoryStub) GetCallbackDelivery(context.Context, string) (*domain.CallbackDelivery, error) {
	return nil, nil
}
func (callbackAgentRepositoryStub) SaveCallbackDelivery(context.Context, *domain.CallbackDelivery) error {
	return nil
}

type callbackDeliveryRepositoryStub struct {
	scheduled       []port.ScheduleCallbackParams
	claimed         *domain.CallbackDelivery
	markedDelivered bool
	retryAt         time.Time
	terminalReason  string
}

func (r *callbackDeliveryRepositoryStub) Schedule(_ context.Context, params port.ScheduleCallbackParams) (*domain.CallbackDelivery, bool, error) {
	r.scheduled = append(r.scheduled, params)
	return params.Delivery, len(r.scheduled) == 1, nil
}
func (*callbackDeliveryRepositoryStub) GetByEventID(context.Context, string) (*domain.CallbackDelivery, error) {
	return nil, nil
}
func (r *callbackDeliveryRepositoryStub) ClaimNext(context.Context, string, time.Time, time.Duration) (*domain.CallbackDelivery, error) {
	claimed := r.claimed
	r.claimed = nil
	return claimed, nil
}
func (r *callbackDeliveryRepositoryStub) MarkDelivered(context.Context, *domain.CallbackDelivery, port.CallbackResultIntent, time.Time) error {
	r.markedDelivered = true
	return nil
}
func (r *callbackDeliveryRepositoryStub) MarkRetry(_ context.Context, _ *domain.CallbackDelivery, _ string, next time.Time, _ time.Time) error {
	r.retryAt = next
	return nil
}
func (r *callbackDeliveryRepositoryStub) MarkTerminal(_ context.Context, _ *domain.CallbackDelivery, reason string, _ port.CallbackResultIntent, _ time.Time) error {
	r.terminalReason = reason
	return nil
}
func (*callbackDeliveryRepositoryStub) ReplayTerminal(context.Context, port.CallbackReplayRequest) (*domain.CallbackDelivery, error) {
	return nil, nil
}

func TestCallbackCommandOnlySchedulesDurableIntent(t *testing.T) {
	verifiedAt := time.Now().UTC()
	deliveries := &callbackDeliveryRepositoryStub{}
	cfg := callbackTestConfig()
	cfg.Callback.RequireVerified = true
	handler := NewCallbackCommandHandler(callbackAgentRepositoryStub{cfg: &domain.AgentConfig{
		AgentID: "agent-1", TenantID: "tenant-1", CallbackURL: "https://callback.test/hook",
		CallbackStatus: domain.CallbackStatusVerified, CallbackVerifiedAt: &verifiedAt,
	}}, deliveries, cfg)

	msg := callbackMessage("transport-id", "tx-1", "agent-1")
	if err := handler.Handle(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if len(deliveries.scheduled) != 1 || deliveries.scheduled[0].Result != nil {
		t.Fatalf("scheduled = %+v", deliveries.scheduled)
	}
	delivery := deliveries.scheduled[0].Delivery
	if delivery.Status != domain.CallbackDeliveryPending || delivery.CallbackURL == "" || len(delivery.Payload) == 0 {
		t.Fatalf("delivery = %+v", delivery)
	}
	if msg.ID != messagebus.MessageID("tx-1", messagebus.StepCallback) {
		t.Fatalf("message id = %q", msg.ID)
	}
}

func TestCallbackCommandPreservesMoneyV2InDurableBody(t *testing.T) {
	deliveries := &callbackDeliveryRepositoryStub{}
	handler := NewCallbackCommandHandler(callbackAgentRepositoryStub{cfg: &domain.AgentConfig{
		AgentID: "agent-money", TenantID: "tenant-1", CallbackURL: "https://callback.test/hook",
	}}, deliveries, callbackTestConfig())
	authoritative := messagebus.Money{AmountMinor: 12_345, Currency: "USD", Exponent: 2}
	payload, _ := json.Marshal(messagebus.DispatchCallbackRequested{
		TransactionID: "tx-money", AgentID: "agent-money", Money: &authoritative,
		Amount: 999.99, Currency: "USD",
	})
	if err := handler.Handle(context.Background(), &kafka.Message{
		ID: "money", EventType: messagebus.DispatchCallbackRequestedType, Payload: payload,
	}); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Money  messagebus.Money `json:"money"`
		Amount float64          `json:"amount"`
	}
	if err := json.Unmarshal(deliveries.scheduled[0].Delivery.Payload, &body); err != nil {
		t.Fatal(err)
	}
	if body.Money != authoritative || body.Amount != 123.45 {
		t.Fatalf("callback body = %s", deliveries.scheduled[0].Delivery.Payload)
	}
}

func TestCallbackCommandCreatesAtomicTerminalResult(t *testing.T) {
	for name, testCase := range map[string]struct {
		cfg        *domain.AgentConfig
		require    bool
		wantStatus string
		wantEvent  string
	}{
		"not configured": {cfg: &domain.AgentConfig{}, wantStatus: domain.CallbackDeliverySkipped, wantEvent: messagebus.CallbackDispatchSkippedType},
		"not verified": {
			cfg:     &domain.AgentConfig{AgentID: "agent-1", TenantID: "tenant-1", CallbackURL: "https://callback.test/hook"},
			require: true, wantStatus: domain.CallbackDeliveryTerminalFailed, wantEvent: messagebus.CallbackDispatchFailedType,
		},
	} {
		t.Run(name, func(t *testing.T) {
			deliveries := &callbackDeliveryRepositoryStub{}
			cfg := callbackTestConfig()
			cfg.Callback.RequireVerified = testCase.require
			handler := NewCallbackCommandHandler(callbackAgentRepositoryStub{cfg: testCase.cfg}, deliveries, cfg)
			if err := handler.Handle(context.Background(), callbackMessage("id", "tx-terminal", "agent-1")); err != nil {
				t.Fatal(err)
			}
			scheduled := deliveries.scheduled[0]
			if scheduled.Delivery.Status != testCase.wantStatus || scheduled.Result == nil || scheduled.Result.MessageType != testCase.wantEvent {
				t.Fatalf("scheduled = %+v", scheduled)
			}
		})
	}
}

func TestCallbackWorkerDeliversWithVersionedSignature(t *testing.T) {
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	deliveries := &callbackDeliveryRepositoryStub{claimed: claimedDelivery()}
	var dispatched port.CallbackDispatchRequest
	runner := NewCallbackDeliveryRunner(deliveries, staticEndpointValidator{}, staticCallbackSigner{},
		callbackDispatcherFunc(func(_ context.Context, request port.CallbackDispatchRequest) (int, error) {
			dispatched = request
			return http.StatusNoContent, nil
		}), callbackTestConfig())
	runner.now = func() time.Time { return now }
	claimed, err := runner.RunNext(context.Background(), "worker-1")
	if err != nil || !claimed || !deliveries.markedDelivered {
		t.Fatalf("claimed=%v delivered=%v err=%v", claimed, deliveries.markedDelivered, err)
	}
	if dispatched.Headers[port.CallbackHeaderSignature] == "" || dispatched.Headers[port.CallbackHeaderKeyVersion] != "test-v1" {
		t.Fatalf("headers = %+v", dispatched.Headers)
	}
}

func TestCallbackWorkerUsesFixedRetryScheduleThenDLQ(t *testing.T) {
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	for attempts, wantDelay := range callbackRetrySchedule {
		deliveries := &callbackDeliveryRepositoryStub{claimed: claimedDelivery()}
		deliveries.claimed.GenerationAttempts = attempts
		runner := NewCallbackDeliveryRunner(deliveries, staticEndpointValidator{}, staticCallbackSigner{},
			callbackDispatcherFunc(func(context.Context, port.CallbackDispatchRequest) (int, error) {
				return http.StatusServiceUnavailable, nil
			}), callbackTestConfig())
		runner.now = func() time.Time { return now }
		if _, err := runner.RunNext(context.Background(), "worker-1"); err != nil {
			t.Fatal(err)
		}
		if got := deliveries.retryAt.Sub(now); got != wantDelay {
			t.Fatalf("attempt %d delay = %s, want %s", attempts, got, wantDelay)
		}
	}

	deliveries := &callbackDeliveryRepositoryStub{claimed: claimedDelivery()}
	deliveries.claimed.GenerationAttempts = len(callbackRetrySchedule)
	runner := NewCallbackDeliveryRunner(deliveries, staticEndpointValidator{}, staticCallbackSigner{},
		callbackDispatcherFunc(func(context.Context, port.CallbackDispatchRequest) (int, error) {
			return http.StatusServiceUnavailable, nil
		}), callbackTestConfig())
	runner.now = func() time.Time { return now }
	if _, err := runner.RunNext(context.Background(), "worker-1"); err != nil {
		t.Fatal(err)
	}
	if deliveries.terminalReason == "" {
		t.Fatal("exhausted callback was not moved to terminal DLQ")
	}
}

type callbackDispatcherFunc func(context.Context, port.CallbackDispatchRequest) (int, error)

func (f callbackDispatcherFunc) Dispatch(ctx context.Context, request port.CallbackDispatchRequest) (int, error) {
	return f(ctx, request)
}

type staticCallbackSigner struct{}

func (staticCallbackSigner) Sign(domain.CallbackSigningInput) (domain.CallbackSignature, error) {
	return domain.CallbackSignature{
		Timestamp: "1700000000", BodySHA256: "body-digest", KeyVersion: "test-v1", Signature: "v1=test-signature",
	}, nil
}

type staticEndpointValidator struct{}

func (staticEndpointValidator) Validate(_ context.Context, rawURL string) (domain.ValidatedCallbackEndpoint, error) {
	return domain.ValidatedCallbackEndpoint{URL: rawURL, Hostname: "callback.test", Verified: true}, nil
}

func callbackTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Kafka.Topics.AgentCommands = "agent.commands"
	cfg.Kafka.Topics.AgentEvents = "agent.events"
	cfg.Callback.WorkerLeaseDuration = 30 * time.Second
	return cfg
}

func callbackMessage(id, transactionID, agentID string) *kafka.Message {
	payload, _ := json.Marshal(messagebus.DispatchCallbackRequested{
		TransactionID: transactionID, AgentID: agentID, Amount: 100, Currency: "USD",
	})
	return &kafka.Message{ID: id, EventType: messagebus.DispatchCallbackRequestedType, Payload: payload}
}

func claimedDelivery() *domain.CallbackDelivery {
	return &domain.CallbackDelivery{
		TransactionID: "tx-1", AgentID: "agent-1", TenantID: "tenant-1",
		EventID: "event-1", CallbackURL: "https://callback.test/hook", Payload: []byte(`{"status":"COMPLETED"}`),
		Status: domain.CallbackDeliveryInFlight, LeaseOwner: "worker-1", LeaseToken: 1, OutcomeGeneration: 1,
	}
}

var _ port.AgentRepository = callbackAgentRepositoryStub{}
var _ port.CallbackDeliveryRepository = (*callbackDeliveryRepositoryStub)(nil)
