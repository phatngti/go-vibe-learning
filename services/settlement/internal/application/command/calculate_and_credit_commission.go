package command

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github/master_go_cqrs/contracts/messagebus"
	"github/master_go_cqrs/internal/platform/eventstore"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/settlement/internal/application/dto"
	"github/master_go_cqrs/services/settlement/internal/application/port"
	"github/master_go_cqrs/services/settlement/internal/config"
	"github/master_go_cqrs/services/settlement/internal/domain"

	"gorm.io/gorm"
)

type CalculateAndCreditCommissionHandler struct {
	db         *gorm.DB
	repo       port.SettlementRepository
	eventStore eventstore.Repository
	outbox     outbox.Repository
	eventTopic string
}

func NewCalculateAndCreditCommissionHandler(db *platformpostgres.DB, repo port.SettlementRepository, es eventstore.Repository, outboxRepo outbox.Repository, cfg *config.Config) *CalculateAndCreditCommissionHandler {
	return &CalculateAndCreditCommissionHandler{
		db: db.DB, repo: repo, eventStore: es, outbox: outboxRepo,
		eventTopic: cfg.Kafka.Topics.SettlementEvents,
	}
}

func (h *CalculateAndCreditCommissionHandler) Handle(ctx context.Context, cmd *dto.CalculateAndCreditCommission) (*domain.AgentBalance, float64, error) {
	var balance *domain.AgentBalance
	var commission float64
	err := h.db.Transaction(func(dbTx *gorm.DB) error {
		var err error
		txCtx := platformpostgres.WithTx(ctx, dbTx)
		balance, commission, err = h.HandleInTx(txCtx, dbTx, cmd)
		return err
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to calculate and credit commission: %w", err)
	}
	return balance, commission, nil
}

func (h *CalculateAndCreditCommissionHandler) HandleInTx(ctx context.Context, dbTx *gorm.DB, cmd *dto.CalculateAndCreditCommission) (*domain.AgentBalance, float64, error) {
	transactionMoney, err := messagebus.ResolveMoney(cmd.TransactionMoney, cmd.TransactionAmount, cmd.Currency)
	if err != nil {
		return nil, 0, fmt.Errorf("calculate_commission: invalid transaction money: %w", err)
	}
	if !transactionMoney.IsPositive() {
		return nil, 0, fmt.Errorf("calculate_commission: transaction amount must be positive")
	}
	basisPoints, err := commissionBasisPoints(cmd.Rank)
	if err != nil {
		return nil, 0, err
	}
	commissionMoney, err := transactionMoney.MultiplyBasisPoints(basisPoints)
	if err != nil {
		return nil, 0, fmt.Errorf("calculate_commission: checked multiplication: %w", err)
	}
	legacyTransactionAmount, err := messagebus.LegacyMajorFloat(transactionMoney)
	if err != nil {
		return nil, 0, fmt.Errorf("calculate_commission: render legacy transaction amount: %w", err)
	}
	legacyCommission, err := messagebus.LegacyMajorFloat(commissionMoney)
	if err != nil {
		return nil, 0, fmt.Errorf("calculate_commission: render legacy commission: %w", err)
	}
	var updatedBalance *domain.AgentBalance

	err = func() error {
		bal, err := h.repo.GetBalanceForUpdate(ctx, cmd.AgentID)
		if err != nil {
			return err
		}

		activeMoney, creditMoney, err := settlementBalanceMoney(bal, transactionMoney)
		if err != nil {
			return err
		}
		activeMoney, err = activeMoney.Add(commissionMoney)
		if err != nil {
			return fmt.Errorf("calculate_commission: update balance: %w", err)
		}
		legacyActive, err := messagebus.LegacyMajorFloat(activeMoney)
		if err != nil {
			return fmt.Errorf("calculate_commission: render legacy balance: %w", err)
		}
		legacyCredit, err := messagebus.LegacyMajorFloat(creditMoney)
		if err != nil {
			return fmt.Errorf("calculate_commission: render legacy credit limit: %w", err)
		}
		bal.ActiveBalance = legacyActive
		bal.CreditLimit = legacyCredit
		bal.ActiveBalanceMoney = messagebus.MoneyPointer(activeMoney)
		bal.CreditLimitMoney = messagebus.MoneyPointer(creditMoney)
		if err := h.repo.SaveBalance(ctx, bal); err != nil {
			return err
		}

		log := &domain.CommissionLog{
			AgentID:           cmd.AgentID,
			TransactionID:     cmd.TransactionID,
			TransactionMoney:  messagebus.MoneyPointer(transactionMoney),
			CommissionMoney:   messagebus.MoneyPointer(commissionMoney),
			TransactionAmount: legacyTransactionAmount,
			CommissionAmount:  legacyCommission,
			Rank:              cmd.Rank,
		}
		if err := h.repo.CreateCommissionLog(ctx, log); err != nil {
			return err
		}

		updatedBalance = bal

		// Append CommissionCredited domain event
		evt := domain.CommissionCredited{
			AgentID:           cmd.AgentID,
			TransactionID:     cmd.TransactionID,
			TransactionMoney:  messagebus.MoneyPointer(transactionMoney),
			CommissionMoney:   messagebus.MoneyPointer(commissionMoney),
			BalanceMoney:      messagebus.MoneyPointer(activeMoney),
			TransactionAmount: legacyTransactionAmount,
			CommissionAmount:  legacyCommission,
			Rank:              cmd.Rank,
			NewBalance:        legacyActive,
			OccurredAt:        time.Now(),
		}
		payload, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("calculate_commission: marshal event: %w", err)
		}
		version, err := h.eventStore.NextVersion(ctx, dbTx, "AgentBalance", cmd.AgentID)
		if err != nil {
			return err
		}
		if err := h.eventStore.AppendEvent(ctx, dbTx, &eventstore.DomainEvent{
			AggregateType: "AgentBalance",
			AggregateID:   cmd.AgentID,
			EventType:     evt.EventType(),
			Payload:       string(payload),
			Version:       version,
		}); err != nil {
			return err
		}
		return h.outbox.Enqueue(ctx, dbTx, outbox.EnqueueParams{
			MessageID: messagebus.MessageID(cmd.TransactionID, "commission-credited"),
			Topic:     h.eventTopic, MessageKey: cmd.TransactionID,
			MessageType:   messagebus.CommissionCreditedType,
			AggregateType: "AgentBalance", AggregateID: cmd.AgentID,
			Payload: payload,
		})
	}()

	if err != nil {
		return nil, 0, err
	}

	return updatedBalance, legacyCommission, nil
}

func commissionBasisPoints(rank string) (int64, error) {
	switch rank {
	case "BRONZE":
		return 100, nil
	case "SILVER":
		return 200, nil
	case "GOLD":
		return 300, nil
	case "PLATINUM":
		return 400, nil
	case "DIAMOND":
		return 500, nil
	default:
		return 0, fmt.Errorf("calculate_commission: unsupported affiliate rank %q", rank)
	}
}

func settlementBalanceMoney(balance *domain.AgentBalance, incoming sharedmoney.Money) (sharedmoney.Money, sharedmoney.Money, error) {
	if (balance.ActiveBalanceMoney == nil) != (balance.CreditLimitMoney == nil) {
		return sharedmoney.Money{}, sharedmoney.Money{}, fmt.Errorf("calculate_commission: stored Money v2 balance is incomplete")
	}
	if balance.ActiveBalanceMoney != nil {
		active, err := sharedmoney.New(balance.ActiveBalanceMoney.AmountMinor, balance.ActiveBalanceMoney.Currency, balance.ActiveBalanceMoney.Exponent)
		if err != nil {
			return sharedmoney.Money{}, sharedmoney.Money{}, fmt.Errorf("calculate_commission: invalid stored active balance: %w", err)
		}
		credit, err := sharedmoney.New(balance.CreditLimitMoney.AmountMinor, balance.CreditLimitMoney.Currency, balance.CreditLimitMoney.Exponent)
		if err != nil {
			return sharedmoney.Money{}, sharedmoney.Money{}, fmt.Errorf("calculate_commission: invalid stored credit limit: %w", err)
		}
		zero := incoming
		zero.AmountMinor = 0
		if _, err := active.Add(zero); err != nil {
			return sharedmoney.Money{}, sharedmoney.Money{}, fmt.Errorf("calculate_commission: active balance currency mismatch: %w", err)
		}
		if _, err := credit.Add(zero); err != nil {
			return sharedmoney.Money{}, sharedmoney.Money{}, fmt.Errorf("calculate_commission: credit limit currency mismatch: %w", err)
		}
		return active, credit, nil
	}
	if balance.ActiveBalance != 0 || balance.CreditLimit != 1000 {
		return sharedmoney.Money{}, sharedmoney.Money{}, domain.ErrMoneyBackfillRequired
	}
	active, err := sharedmoney.New(0, incoming.Currency, incoming.Exponent)
	if err != nil {
		return sharedmoney.Money{}, sharedmoney.Money{}, err
	}
	credit, err := sharedmoney.FromLegacyFloat64(balance.CreditLimit, incoming.Currency)
	if err != nil {
		return sharedmoney.Money{}, sharedmoney.Money{}, err
	}
	return active, credit, nil
}
