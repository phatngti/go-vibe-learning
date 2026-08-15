package command

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github/master_go_cqrs/internal/platform/eventstore"
	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/payment/internal/application/dto"
	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/config"
	"github/master_go_cqrs/services/payment/internal/domain"
)

// InitiatePaymentHandler creates a new PENDING transaction and appends a
// PaymentInitiated domain event to the event store within the same DB transaction.
type InitiatePaymentHandler struct {
	uow                port.UnitOfWork
	repo               port.TransactionRepository
	eventStore         eventstore.Repository
	requireIdempotency bool
}

func NewInitiatePaymentHandler(uow port.UnitOfWork, repo port.TransactionRepository, eventStore eventstore.Repository, cfg *config.Config) *InitiatePaymentHandler {
	return &InitiatePaymentHandler{
		uow:                uow,
		repo:               repo,
		eventStore:         eventStore,
		requireIdempotency: cfg.Initiation.RequireIdempotency,
	}
}

func (h *InitiatePaymentHandler) Handle(ctx context.Context, cmd *dto.InitiatePayment) (*domain.Transaction, error) {
	amount, err := sharedmoney.FromLegacyFloat64(cmd.Amount, cmd.Currency)
	if err != nil {
		return nil, fmt.Errorf("initiate_payment: invalid money: %w", err)
	}
	if !amount.IsPositive() {
		return nil, fmt.Errorf("initiate_payment: amount must be positive")
	}
	requestKey := strings.TrimSpace(cmd.IdempotencyKey)
	if h.requireIdempotency && requestKey == "" {
		return nil, domain.ErrInitiationIdempotencyRequired
	}
	if len(requestKey) > 200 {
		return nil, fmt.Errorf("initiation idempotency key exceeds 200 characters")
	}
	fingerprint, err := initiationFingerprint(cmd)
	if err != nil {
		return nil, err
	}
	tx := &domain.Transaction{
		AgentID:               cmd.AgentID,
		CustomerID:            cmd.CustomerID,
		Amount:                cmd.Amount,
		AmountMinor:           amount.AmountMinor,
		CurrencyExponent:      amount.Exponent,
		Currency:              amount.Currency,
		Status:                domain.StatePending,
		CallbackURL:           cmd.CallbackURL,
		RequestIdempotencyKey: requestKey,
		RequestFingerprint:    fingerprint,
	}

	var savedTx *domain.Transaction

	err = h.uow.Do(ctx, func(txCtx context.Context) error {
		created, err := h.repo.CreateOrGet(txCtx, tx)
		if err != nil {
			return err
		}
		savedTx = tx
		if !created {
			return nil
		}

		eventMoney := amount
		evt := domain.PaymentInitiated{
			TransactionID: tx.ID,
			AgentID:       tx.AgentID,
			CustomerID:    tx.CustomerID,
			Money:         &eventMoney,
			Amount:        tx.Amount,
			Currency:      tx.Currency,
			CallbackURL:   tx.CallbackURL,
			OccurredAt:    time.Now(),
		}
		payload, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("initiate_payment: marshal event: %w", err)
		}
		version, err := h.eventStore.NextVersion(txCtx, nil, "Transaction", tx.ID)
		if err != nil {
			return err
		}
		return h.eventStore.AppendEvent(txCtx, nil, &eventstore.DomainEvent{
			AggregateType: "Transaction",
			AggregateID:   tx.ID,
			EventType:     evt.EventType(),
			Payload:       string(payload),
			Version:       version,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("initiate_payment: %w", err)
	}

	return savedTx, nil
}

func initiationFingerprint(cmd *dto.InitiatePayment) (string, error) {
	canonical := struct {
		AgentID     string  `json:"agent_id"`
		CustomerID  string  `json:"customer_id"`
		Amount      float64 `json:"amount"`
		Currency    string  `json:"currency"`
		CallbackURL string  `json:"callback_url"`
	}{
		AgentID: strings.TrimSpace(cmd.AgentID), CustomerID: strings.TrimSpace(cmd.CustomerID),
		Amount: cmd.Amount, Currency: strings.ToUpper(strings.TrimSpace(cmd.Currency)),
		CallbackURL: strings.TrimSpace(cmd.CallbackURL),
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("initiate_payment: fingerprint request: %w", err)
	}
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("sha256:%x", digest[:]), nil
}
