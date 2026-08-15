package simulator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github/master_go_cqrs/services/payment/internal/application/port"
)

type intentProvider struct {
	mu    sync.Mutex
	byKey map[string]*port.PaymentIntent
	byID  map[string]*port.PaymentIntent
}

func NewPaymentProvider() port.PaymentProvider {
	return &intentProvider{byKey: make(map[string]*port.PaymentIntent), byID: make(map[string]*port.PaymentIntent)}
}

func (p *intentProvider) Name() string { return "simulator" }

func (p *intentProvider) CreateIntent(_ context.Context, request port.CreateIntentRequest) (*port.PaymentIntent, error) {
	if err := request.Money.Validate(); err != nil || !request.Money.IsPositive() {
		return nil, fmt.Errorf("simulator create intent: invalid money")
	}
	if request.TransactionID == "" || request.IdempotencyKey == "" {
		return nil, fmt.Errorf("simulator create intent: transaction ID and idempotency key are required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if existing := p.byKey[request.IdempotencyKey]; existing != nil {
		copy := *existing
		return &copy, nil
	}
	intent := &port.PaymentIntent{
		ProviderReference: "sim-pi-" + request.TransactionID,
		ClientSecret:      "sim-secret-" + request.TransactionID,
		Status:            port.ProviderStatusRequiresPaymentMethod,
		Money:             request.Money,
		TransactionID:     request.TransactionID,
	}
	p.byKey[request.IdempotencyKey] = intent
	p.byID[intent.ProviderReference] = intent
	copy := *intent
	return &copy, nil
}

func (p *intentProvider) RetrieveIntent(_ context.Context, providerReference string) (*port.PaymentIntent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	intent := p.byID[providerReference]
	if intent == nil {
		return nil, fmt.Errorf("simulator intent %s not found", providerReference)
	}
	copy := *intent
	return &copy, nil
}

func (p *intentProvider) CancelIntent(ctx context.Context, providerReference string) (*port.PaymentIntent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	intent := p.byID[providerReference]
	if intent == nil {
		return nil, fmt.Errorf("simulator intent %s not found", providerReference)
	}
	intent.Status = port.ProviderStatusCanceled
	copy := *intent
	return &copy, nil
}

func (p *intentProvider) ParseWebhook(payload []byte, _ string, receivedAt time.Time) (*port.ProviderWebhookEvent, error) {
	var event port.ProviderWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("simulator webhook: %w", err)
	}
	if event.EventID == "" {
		return nil, fmt.Errorf("simulator webhook: event ID is required")
	}
	event.Provider = p.Name()
	event.ReceivedAt = receivedAt
	digest := sha256.Sum256(payload)
	event.PayloadSHA256 = hex.EncodeToString(digest[:])
	return &event, nil
}
