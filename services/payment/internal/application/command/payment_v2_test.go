package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/payment/internal/application/dto"
	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/domain"

	"gorm.io/gorm"
)

type paymentV2RepoStub struct {
	*processingRepoStub
	stored          *domain.Transaction
	providerEvents  map[string]struct{}
	processingState map[string]string
}

func newPaymentV2RepoStub() *paymentV2RepoStub {
	return &paymentV2RepoStub{
		processingRepoStub: &processingRepoStub{tx: paymentTransaction(domain.StatePending)},
		providerEvents:     make(map[string]struct{}), processingState: make(map[string]string),
	}
}

func (r *paymentV2RepoStub) CreateOrGet(_ context.Context, tx *domain.Transaction) (bool, error) {
	if r.stored == nil {
		copy := *tx
		copy.CreatedAt = time.Now()
		copy.UpdatedAt = copy.CreatedAt
		r.stored = &copy
		*tx = copy
		return true, nil
	}
	if r.stored.AgentID != tx.AgentID || r.stored.RequestIdempotencyKey != tx.RequestIdempotencyKey || r.stored.RequestFingerprint != tx.RequestFingerprint {
		return false, domain.ErrInitiationIdempotencyConflict
	}
	*tx = *r.stored
	return false, nil
}

func (r *paymentV2RepoStub) GetByID(_ context.Context, id string) (*domain.Transaction, error) {
	if r.stored == nil || r.stored.ID != id {
		return nil, domain.ErrTransactionNotFound
	}
	copy := *r.stored
	return &copy, nil
}

func (r *paymentV2RepoStub) SaveProviderIntent(_ context.Context, id, provider, providerReference, providerStatus string) error {
	if r.stored == nil || r.stored.ID != id {
		return domain.ErrTransactionNotFound
	}
	if r.stored.ProviderReference != "" && r.stored.ProviderReference != providerReference {
		return errors.New("provider reference conflict")
	}
	r.stored.Provider = provider
	r.stored.ProviderReference = providerReference
	r.stored.ProviderStatus = providerStatus
	return nil
}

func (r *paymentV2RepoStub) TryRecordProviderEvent(_ context.Context, event port.ProviderWebhookEvent) (bool, error) {
	key := event.Provider + ":" + event.EventID
	if _, exists := r.providerEvents[key]; exists {
		return false, nil
	}
	r.providerEvents[key] = struct{}{}
	return true, nil
}

func (r *paymentV2RepoStub) ApplyProviderOutcome(_ context.Context, transactionID, provider, providerReference, providerStatus string, targetState *domain.TransactionState, lastError string) (*domain.Transaction, bool, error) {
	if r.stored == nil || r.stored.ID != transactionID {
		return nil, false, domain.ErrTransactionNotFound
	}
	r.stored.ProviderReference = providerReference
	r.stored.Provider = provider
	r.stored.ProviderStatus = providerStatus
	changed := targetState != nil && r.stored.Status != *targetState
	if changed {
		r.stored.Status = *targetState
		r.stored.LastError = lastError
	}
	copy := *r.stored
	return &copy, changed, nil
}

func (r *paymentV2RepoStub) MarkProviderEventProcessed(_ context.Context, provider, eventID, processingStatus, _ string) error {
	r.processingState[provider+":"+eventID] = processingStatus
	return nil
}

type paymentProviderStub struct {
	intent        *port.PaymentIntent
	event         *port.ProviderWebhookEvent
	createErrors  []error
	createKeys    []string
	retrieveCalls int
}

func (p *paymentProviderStub) Name() string { return "stripe" }

func (p *paymentProviderStub) CreateIntent(_ context.Context, request port.CreateIntentRequest) (*port.PaymentIntent, error) {
	p.createKeys = append(p.createKeys, request.IdempotencyKey)
	index := len(p.createKeys) - 1
	if index < len(p.createErrors) && p.createErrors[index] != nil {
		return nil, p.createErrors[index]
	}
	copy := *p.intent
	copy.TransactionID = request.TransactionID
	copy.Money = request.Money
	p.intent.TransactionID = request.TransactionID
	p.intent.Money = request.Money
	return &copy, nil
}

func (p *paymentProviderStub) RetrieveIntent(context.Context, string) (*port.PaymentIntent, error) {
	p.retrieveCalls++
	copy := *p.intent
	return &copy, nil
}

func (p *paymentProviderStub) CancelIntent(context.Context, string) (*port.PaymentIntent, error) {
	copy := *p.intent
	copy.Status = port.ProviderStatusCanceled
	return &copy, nil
}

func (p *paymentProviderStub) ParseWebhook([]byte, string, time.Time) (*port.ProviderWebhookEvent, error) {
	copy := *p.event
	return &copy, nil
}

type countingEventStore struct {
	events []*eventstore.DomainEvent
}

func (s *countingEventStore) AppendEvent(_ context.Context, _ *gorm.DB, event *eventstore.DomainEvent) error {
	s.events = append(s.events, event)
	return nil
}

func (*countingEventStore) LoadEvents(context.Context, string, string) ([]*eventstore.DomainEvent, error) {
	return nil, nil
}

func (*countingEventStore) LoadEventsFrom(context.Context, string, string, int64) ([]*eventstore.DomainEvent, error) {
	return nil, nil
}

func (s *countingEventStore) NextVersion(context.Context, *gorm.DB, string, string) (int64, error) {
	return int64(len(s.events) + 1), nil
}

func TestCreatePaymentV2IsStableAcrossHTTPRetries(t *testing.T) {
	repo := newPaymentV2RepoStub()
	provider := &paymentProviderStub{intent: &port.PaymentIntent{
		ProviderReference: "pi_123", ClientSecret: "pi_123_secret", Status: port.ProviderStatusRequiresAction,
	}}
	handler := NewCreatePaymentV2Handler(&uowStub{}, repo, provider, &countingEventStore{})
	value, err := sharedmoney.New(1050, "USD", 2)
	if err != nil {
		t.Fatal(err)
	}
	request := dto.CreatePaymentV2{
		AgentID: "agent-1", Money: value, CustomerReference: "customer-1", IdempotencyKey: "order-123",
	}
	first, err := handler.Handle(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := handler.Handle(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Transaction.ID != second.Transaction.ID || first.ClientSecret != second.ClientSecret {
		t.Fatalf("retry changed result: first=%+v second=%+v", first, second)
	}
	if len(provider.createKeys) != 1 || provider.retrieveCalls != 1 {
		t.Fatalf("provider create keys=%v retrieve calls=%d", provider.createKeys, provider.retrieveCalls)
	}
	wantKey := "payment:" + first.Transaction.ID + ":intent:v1"
	if provider.createKeys[0] != wantKey {
		t.Fatalf("provider idempotency key = %q, want %q", provider.createKeys[0], wantKey)
	}
}

func TestCreatePaymentV2RetriesAmbiguousProviderCreateWithSameKey(t *testing.T) {
	repo := newPaymentV2RepoStub()
	provider := &paymentProviderStub{
		intent:       &port.PaymentIntent{ProviderReference: "pi_123", ClientSecret: "secret", Status: port.ProviderStatusRequiresAction},
		createErrors: []error{errors.New("provider timeout")},
	}
	handler := NewCreatePaymentV2Handler(&uowStub{}, repo, provider, &countingEventStore{})
	value, _ := sharedmoney.New(1050, "USD", 2)
	request := dto.CreatePaymentV2{AgentID: "agent-1", Money: value, IdempotencyKey: "order-123"}
	if _, err := handler.Handle(context.Background(), request); err == nil {
		t.Fatal("first Handle() error = nil, want provider timeout")
	}
	if _, err := handler.Handle(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(provider.createKeys) != 2 || provider.createKeys[0] != provider.createKeys[1] {
		t.Fatalf("provider retry keys = %v", provider.createKeys)
	}
}

func TestHandleProviderWebhookDeduplicatesBusinessEffect(t *testing.T) {
	repo := newPaymentV2RepoStub()
	repo.stored = &domain.Transaction{
		ID: "tx-1", AgentID: "agent-1", Amount: 10.50, AmountMinor: 1050,
		CurrencyExponent: 2, Currency: "USD", Status: domain.StatePending,
	}
	provider := &paymentProviderStub{
		intent: &port.PaymentIntent{},
		event: &port.ProviderWebhookEvent{
			Provider: "stripe", EventID: "evt_123", EventType: "payment_intent.succeeded",
			TransactionID: "tx-1", ProviderReference: "pi_123", Status: port.ProviderStatusSucceeded,
			OccurredAt: time.Now(), ReceivedAt: time.Now(),
		},
	}
	store := &countingEventStore{}
	handler := NewHandleProviderWebhookHandler(&uowStub{}, repo, provider, store)
	first, err := handler.Handle(context.Background(), []byte("signed"), "signature", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	second, err := handler.Handle(context.Background(), []byte("signed"), "signature", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if first.Duplicate || !second.Duplicate {
		t.Fatalf("duplicate flags first=%v second=%v", first.Duplicate, second.Duplicate)
	}
	if repo.stored.Status != domain.StateCompleted || len(store.events) != 1 {
		t.Fatalf("transaction status=%s events=%d", repo.stored.Status, len(store.events))
	}
	if got := repo.processingState["stripe:evt_123"]; got != "PROCESSED" {
		t.Fatalf("webhook processing state = %q", got)
	}
}
