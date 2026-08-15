package command

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/payment/internal/application/dto"
	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/domain"

	"github.com/google/uuid"
)

const (
	maxProviderMetadataEntries = 20
	maxProviderMetadataKey     = 40
	maxProviderMetadataValue   = 500
)

type CreatePaymentV2Handler struct {
	uow        port.UnitOfWork
	repo       port.PaymentV2Repository
	provider   port.PaymentProvider
	eventStore eventstore.Repository
}

func NewCreatePaymentV2Handler(
	uow port.UnitOfWork,
	repo port.PaymentV2Repository,
	provider port.PaymentProvider,
	eventStore eventstore.Repository,
) *CreatePaymentV2Handler {
	return &CreatePaymentV2Handler{uow: uow, repo: repo, provider: provider, eventStore: eventStore}
}

func (h *CreatePaymentV2Handler) Handle(ctx context.Context, cmd dto.CreatePaymentV2) (*dto.CreatePaymentV2Result, error) {
	if strings.TrimSpace(cmd.AgentID) == "" {
		return nil, fmt.Errorf("create payment v2: authenticated owner is required")
	}
	if err := cmd.Money.Validate(); err != nil {
		return nil, fmt.Errorf("create payment v2: invalid money: %w", err)
	}
	if !cmd.Money.IsPositive() {
		return nil, fmt.Errorf("create payment v2: amount must be positive")
	}
	requestKey := strings.TrimSpace(cmd.IdempotencyKey)
	if requestKey == "" {
		return nil, domain.ErrInitiationIdempotencyRequired
	}
	if len(requestKey) > 200 {
		return nil, fmt.Errorf("create payment v2: idempotency key exceeds 200 characters")
	}
	if err := validateProviderMetadata(cmd.Metadata); err != nil {
		return nil, err
	}

	fingerprint, err := paymentV2Fingerprint(cmd)
	if err != nil {
		return nil, err
	}
	legacyAmount, err := legacyMajorFloat(cmd.Money)
	if err != nil {
		return nil, err
	}
	tx := &domain.Transaction{
		ID:                    uuid.NewString(),
		AgentID:               strings.TrimSpace(cmd.AgentID),
		CustomerID:            strings.TrimSpace(cmd.CustomerReference),
		Amount:                legacyAmount,
		AmountMinor:           cmd.Money.AmountMinor,
		CurrencyExponent:      cmd.Money.Exponent,
		Currency:              cmd.Money.Currency,
		Status:                domain.StatePending,
		CallbackEndpointID:    strings.TrimSpace(cmd.CallbackEndpointID),
		RequestIdempotencyKey: requestKey,
		RequestFingerprint:    fingerprint,
	}
	tx.IdempotencyKey = providerIntentIdempotencyKey(tx.ID)

	created := false
	err = h.uow.Do(ctx, func(txCtx context.Context) error {
		var createErr error
		created, createErr = h.repo.CreateOrGet(txCtx, tx)
		if createErr != nil || !created {
			return createErr
		}
		eventMoney := cmd.Money
		evt := domain.PaymentInitiated{
			TransactionID: tx.ID, AgentID: tx.AgentID, CustomerID: tx.CustomerID,
			Money: &eventMoney, Amount: tx.Amount, Currency: tx.Currency, OccurredAt: time.Now(),
		}
		payload, marshalErr := json.Marshal(evt)
		if marshalErr != nil {
			return fmt.Errorf("create payment v2: marshal event: %w", marshalErr)
		}
		version, versionErr := h.eventStore.NextVersion(txCtx, nil, "Transaction", tx.ID)
		if versionErr != nil {
			return versionErr
		}
		return h.eventStore.AppendEvent(txCtx, nil, &eventstore.DomainEvent{
			AggregateType: "Transaction", AggregateID: tx.ID, EventType: evt.EventType(),
			Payload: string(payload), Version: version,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("create payment v2: persist intent request: %w", err)
	}

	var intent *port.PaymentIntent
	if !created && tx.ProviderReference != "" {
		intent, err = h.provider.RetrieveIntent(ctx, tx.ProviderReference)
	} else {
		intent, err = h.provider.CreateIntent(ctx, port.CreateIntentRequest{
			TransactionID: tx.ID, IdempotencyKey: tx.IdempotencyKey, Money: cmd.Money,
			CustomerReference: cmd.CustomerReference, Metadata: cloneMetadata(cmd.Metadata),
		})
	}
	if err != nil {
		return nil, fmt.Errorf("create payment v2: provider intent: %w", err)
	}
	if intent == nil || strings.TrimSpace(intent.ProviderReference) == "" {
		return nil, fmt.Errorf("create payment v2: provider returned an invalid intent")
	}
	if err := validateProviderIntent(intent, tx.ID, cmd.Money); err != nil {
		return nil, err
	}
	if err := h.repo.SaveProviderIntent(ctx, tx.ID, h.provider.Name(), intent.ProviderReference, string(intent.Status)); err != nil {
		return nil, err
	}
	tx, err = h.repo.GetByID(ctx, tx.ID)
	if err != nil {
		return nil, err
	}
	response, err := transactionResponseV2(tx)
	if err != nil {
		return nil, fmt.Errorf("create payment v2: map response: %w", err)
	}
	return &dto.CreatePaymentV2Result{
		Transaction:  response,
		Provider:     h.provider.Name(),
		ClientSecret: intent.ClientSecret,
	}, nil
}

func paymentV2Fingerprint(cmd dto.CreatePaymentV2) (string, error) {
	canonical := struct {
		AgentID            string            `json:"agent_id"`
		AmountMinor        int64             `json:"amount_minor"`
		Currency           string            `json:"currency"`
		Exponent           int32             `json:"exponent"`
		CustomerReference  string            `json:"customer_reference"`
		CallbackEndpointID string            `json:"callback_endpoint_id"`
		Metadata           map[string]string `json:"metadata,omitempty"`
	}{
		AgentID: strings.TrimSpace(cmd.AgentID), AmountMinor: cmd.Money.AmountMinor,
		Currency: cmd.Money.Currency, Exponent: cmd.Money.Exponent,
		CustomerReference:  strings.TrimSpace(cmd.CustomerReference),
		CallbackEndpointID: strings.TrimSpace(cmd.CallbackEndpointID), Metadata: cmd.Metadata,
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("create payment v2: fingerprint request: %w", err)
	}
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("v2:sha256:%x", digest[:]), nil
}

func validateProviderMetadata(metadata map[string]string) error {
	if len(metadata) > maxProviderMetadataEntries {
		return fmt.Errorf("create payment v2: metadata exceeds %d entries", maxProviderMetadataEntries)
	}
	for key, value := range metadata {
		if key == "" || len(key) > maxProviderMetadataKey || len(value) > maxProviderMetadataValue {
			return fmt.Errorf("create payment v2: invalid provider metadata")
		}
		if !providerMetadataKeyIsSafe(key) {
			return fmt.Errorf("create payment v2: metadata key contains unsupported characters")
		}
		if key == "transaction_id" || key == "customer_reference" {
			return fmt.Errorf("create payment v2: metadata key %s is reserved", key)
		}
	}
	return nil
}

func providerMetadataKeyIsSafe(key string) bool {
	for _, character := range key {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validateProviderIntent(intent *port.PaymentIntent, transactionID string, expected sharedmoney.Money) error {
	if intent.TransactionID != "" && intent.TransactionID != transactionID {
		return fmt.Errorf("create payment v2: provider intent belongs to a different transaction")
	}
	if err := intent.Money.Validate(); err != nil {
		return fmt.Errorf("create payment v2: provider returned invalid money: %w", err)
	}
	if intent.Money != expected {
		return fmt.Errorf("create payment v2: provider intent money does not match the request")
	}
	return nil
}

func cloneMetadata(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func providerIntentIdempotencyKey(transactionID string) string {
	return "payment:" + transactionID + ":intent:v1"
}

func legacyMajorFloat(value sharedmoney.Money) (float64, error) {
	rendered, err := value.MajorString()
	if err != nil {
		return 0, err
	}
	legacy, err := strconv.ParseFloat(rendered, 64)
	if err != nil {
		return 0, fmt.Errorf("create payment v2: amount cannot be represented by the legacy column: %w", err)
	}
	return legacy, nil
}

func transactionResponseV2(tx *domain.Transaction) (*dto.TransactionResponseV2, error) {
	moneyValue, err := tx.MonetaryAmount()
	if err != nil {
		return nil, err
	}
	return &dto.TransactionResponseV2{
		ID: tx.ID, AgentID: tx.AgentID, CustomerReference: tx.CustomerID,
		Money: moneyValue, Status: string(tx.Status), Provider: tx.Provider,
		ProviderReference: tx.ProviderReference, ProviderStatus: tx.ProviderStatus,
		CallbackEndpointID: tx.CallbackEndpointID,
		CreatedAt:          tx.CreatedAt.Format(time.RFC3339), UpdatedAt: tx.UpdatedAt.Format(time.RFC3339),
	}, nil
}
