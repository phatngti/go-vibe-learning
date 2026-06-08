package command

import (
	"context"
	"encoding/json"
	"fmt"

	"github/master_go_cqrs/internal/payment/application/dto"
	"github/master_go_cqrs/internal/payment/domain"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
)

type ProcessPaymentHandler struct {
	db         *gorm.DB
	repo       domain.TransactionRepository
	gateway    domain.PaymentGateway
	outboxRepo outbox.Repository
}

func NewProcessPaymentHandler(db *platformpostgres.DB, repo domain.TransactionRepository, gateway domain.PaymentGateway, outboxRepo outbox.Repository) *ProcessPaymentHandler {
	return &ProcessPaymentHandler{
		db:         db.DB,
		repo:       repo,
		gateway:    gateway,
		outboxRepo: outboxRepo,
	}
}

func (h *ProcessPaymentHandler) Handle(ctx context.Context, cmd *dto.ProcessPayment) (*domain.Transaction, error) {
	txRecord, err := h.repo.GetByID(ctx, cmd.TransactionID)
	if err != nil {
		return nil, err
	}

	if txRecord.Status != domain.StatePending {
		return nil, fmt.Errorf("transaction %s is already processed", cmd.TransactionID)
	}

	chargeReq := &domain.GatewayChargeRequest{
		TransactionID: txRecord.ID,
		Amount:        txRecord.Amount,
		Currency:      txRecord.Currency,
		CardNumber:    cmd.CardNumber,
		CVV:           cmd.CVV,
	}

	chargeRes, err := h.gateway.Charge(ctx, chargeReq)
	var finalState domain.TransactionState
	if err != nil || !chargeRes.Success {
		finalState = domain.StateFailed
	} else {
		finalState = domain.StateCompleted
	}

	var updatedTx *domain.Transaction
	err = h.db.Transaction(func(dbTx *gorm.DB) error {
		if err := h.repo.UpdateStatus(ctx, dbTx, txRecord.ID, finalState); err != nil {
			return err
		}

		var err error
		updatedTx, err = h.repo.GetByIDWithDB(ctx, dbTx, txRecord.ID)
		if err != nil {
			return err
		}

		if finalState == domain.StateCompleted {
			payloadBytes, err := json.Marshal(updatedTx)
			if err != nil {
				return err
			}

			if err := h.outboxRepo.SaveEvent(ctx, dbTx, "Transaction", updatedTx.ID, "PaymentCompletedEvent", string(payloadBytes)); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to complete process payment transaction: %w", err)
	}

	return updatedTx, nil
}
