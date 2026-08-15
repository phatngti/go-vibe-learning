package command

import (
	"context"
	"errors"
	"testing"
	"time"

	"github/master_go_cqrs/services/payment/internal/application/dto"
	"github/master_go_cqrs/services/payment/internal/domain"
)

type processingRepoStub struct {
	tx       *domain.Transaction
	claimed  bool
	claimErr error
	claimKey string
	resolved bool
}

func (r *processingRepoStub) CreateOrGet(context.Context, *domain.Transaction) (bool, error) {
	return true, nil
}
func (r *processingRepoStub) GetByID(context.Context, string) (*domain.Transaction, error) {
	copy := *r.tx
	return &copy, nil
}
func (r *processingRepoStub) ClaimForProcessing(_ context.Context, _ string, key string, _ time.Time) (*domain.Transaction, bool, error) {
	r.claimKey = key
	if r.claimErr != nil {
		return r.tx, false, r.claimErr
	}
	if r.claimed {
		r.tx.Status = domain.StateProcessing
		r.tx.IdempotencyKey = key
	}
	copy := *r.tx
	return &copy, r.claimed, nil
}
func (r *processingRepoStub) FinalizeProcessing(_ context.Context, _ string, key string, state domain.TransactionState, reference, lastError string) error {
	if key != r.claimKey {
		return errors.New("finalized with a different idempotency key")
	}
	r.tx.Status = state
	r.tx.ProviderReference = reference
	r.tx.LastError = lastError
	return nil
}
func (r *processingRepoStub) ListForReconciliation(context.Context, time.Time, int) ([]*domain.Transaction, error) {
	return nil, nil
}
func (r *processingRepoStub) ResolveReconciliation(context.Context, string, string, domain.TransactionState, string, string) (bool, error) {
	r.resolved = true
	return true, nil
}

type paymentGatewayStub struct {
	response *domain.GatewayChargeResponse
	err      error
	calls    int
	key      string
	lookup   *domain.GatewayLookupResponse
}

func (g *paymentGatewayStub) Charge(_ context.Context, request *domain.GatewayChargeRequest) (*domain.GatewayChargeResponse, error) {
	g.calls++
	g.key = request.IdempotencyKey
	return g.response, g.err
}
func (g *paymentGatewayStub) LookupCharge(context.Context, string, string) (*domain.GatewayLookupResponse, error) {
	if g.lookup == nil {
		return &domain.GatewayLookupResponse{Found: false}, nil
	}
	return g.lookup, nil
}

func TestProcessPaymentUsesStableKeyAndCommitsCompletedEvent(t *testing.T) {
	repo := &processingRepoStub{claimed: true, tx: paymentTransaction(domain.StatePending)}
	gateway := &paymentGatewayStub{response: &domain.GatewayChargeResponse{Success: true, Reference: "provider-1"}}
	es := &eventStoreStub{}
	handler := NewProcessPaymentHandler(&uowStub{}, repo, gateway, es)

	result, err := handler.Handle(context.Background(), &dto.ProcessPayment{TransactionID: repo.tx.ID})
	if err != nil {
		t.Fatal(err)
	}
	wantKey := "payment:" + repo.tx.ID + ":charge"
	if gateway.key != wantKey || repo.claimKey != wantKey {
		t.Fatalf("provider key = %q, claim key = %q, want %q", gateway.key, repo.claimKey, wantKey)
	}
	if result.Status != domain.StateCompleted || result.ProviderReference != "provider-1" {
		t.Fatalf("result = %+v", result)
	}
	if es.appended == nil || es.appended.EventType != (domain.PaymentCompleted{}).EventType() {
		t.Fatalf("appended event = %+v", es.appended)
	}
}

func TestProcessPaymentTreatsProviderTimeoutAsUnknown(t *testing.T) {
	repo := &processingRepoStub{claimed: true, tx: paymentTransaction(domain.StatePending)}
	gateway := &paymentGatewayStub{err: errors.New("provider timeout")}
	es := &eventStoreStub{}
	handler := NewProcessPaymentHandler(&uowStub{}, repo, gateway, es)

	result, err := handler.Handle(context.Background(), &dto.ProcessPayment{TransactionID: repo.tx.ID, IdempotencyKey: "attempt-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != domain.StateUnknown || result.LastError != "provider timeout" {
		t.Fatalf("result = %+v", result)
	}
	if es.appended == nil || es.appended.EventType != (domain.PaymentProcessingUncertain{}).EventType() {
		t.Fatalf("appended event = %+v", es.appended)
	}
}

func TestProcessPaymentDoesNotCallProviderWhileClaimIsActive(t *testing.T) {
	repo := &processingRepoStub{claimed: false, tx: paymentTransaction(domain.StateProcessing)}
	repo.tx.IdempotencyKey = "attempt-1"
	gateway := &paymentGatewayStub{}
	handler := NewProcessPaymentHandler(&uowStub{}, repo, gateway, &eventStoreStub{})

	_, err := handler.Handle(context.Background(), &dto.ProcessPayment{TransactionID: repo.tx.ID, IdempotencyKey: "attempt-1"})
	if !errors.Is(err, domain.ErrPaymentInProgress) {
		t.Fatalf("error = %v, want payment in progress", err)
	}
	if gateway.calls != 0 {
		t.Fatalf("gateway calls = %d, want 0", gateway.calls)
	}
}

func TestProcessPaymentReturnsExistingTerminalResultIdempotently(t *testing.T) {
	repo := &processingRepoStub{claimed: false, tx: paymentTransaction(domain.StateCompleted)}
	repo.tx.IdempotencyKey = "attempt-1"
	gateway := &paymentGatewayStub{}
	handler := NewProcessPaymentHandler(&uowStub{}, repo, gateway, &eventStoreStub{})

	result, err := handler.Handle(context.Background(), &dto.ProcessPayment{TransactionID: repo.tx.ID, IdempotencyKey: "attempt-1"})
	if err != nil || result.Status != domain.StateCompleted {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	if gateway.calls != 0 {
		t.Fatalf("gateway calls = %d, want 0", gateway.calls)
	}
}

func TestPaymentReconcilerResolvesUnknownWithoutNewCharge(t *testing.T) {
	repo := &processingRepoStub{tx: paymentTransaction(domain.StateUnknown)}
	repo.tx.IdempotencyKey = "attempt-1"
	gateway := &paymentGatewayStub{lookup: &domain.GatewayLookupResponse{Found: true, Completed: true, Reference: "provider-1"}}
	eventStore := &eventStoreStub{}
	reconciler := &PaymentReconciler{uow: &uowStub{}, repo: repo, gateway: gateway, eventStore: eventStore}

	if err := reconciler.reconcileOne(context.Background(), repo.tx); err != nil {
		t.Fatal(err)
	}
	if !repo.resolved || gateway.calls != 0 {
		t.Fatalf("resolved=%v charge calls=%d", repo.resolved, gateway.calls)
	}
	if eventStore.appended == nil || eventStore.appended.EventType != (domain.PaymentCompleted{}).EventType() {
		t.Fatalf("appended event = %+v", eventStore.appended)
	}
}

func TestPaymentReconcilerWaitsForAuthoritativeOutcome(t *testing.T) {
	repo := &processingRepoStub{tx: paymentTransaction(domain.StateUnknown)}
	repo.tx.IdempotencyKey = "attempt-1"
	eventStore := &eventStoreStub{}
	reconciler := &PaymentReconciler{uow: &uowStub{}, repo: repo, gateway: &paymentGatewayStub{}, eventStore: eventStore}

	if err := reconciler.reconcileOne(context.Background(), repo.tx); err != nil {
		t.Fatal(err)
	}
	if repo.resolved || eventStore.appended != nil {
		t.Fatalf("resolved=%v event=%+v", repo.resolved, eventStore.appended)
	}
}

func paymentTransaction(state domain.TransactionState) *domain.Transaction {
	return &domain.Transaction{
		ID: "tx-1", AgentID: "agent-1", CustomerID: "customer-1",
		Amount: 100, Currency: "USD", Status: state,
	}
}
