package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github/master_go_cqrs/internal/platform/inbox"
	"github/master_go_cqrs/internal/platform/outbox"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type callbackDeliveryRepository struct {
	db     *gorm.DB
	inbox  inbox.Repository
	outbox outbox.Repository
}

func NewCallbackDeliveryRepository(
	db *platformpostgres.DB,
	inboxRepo inbox.Repository,
	outboxRepo outbox.Repository,
) port.CallbackDeliveryRepository {
	return &callbackDeliveryRepository{db: db.DB, inbox: inboxRepo, outbox: outboxRepo}
}

func (r *callbackDeliveryRepository) Schedule(
	ctx context.Context,
	params port.ScheduleCallbackParams,
) (*domain.CallbackDelivery, bool, error) {
	if params.Delivery == nil || strings.TrimSpace(params.Delivery.EventID) == "" || strings.TrimSpace(params.Delivery.TransactionID) == "" {
		return nil, false, errors.New("callback_delivery_repository: delivery, event ID, and transaction ID are required")
	}
	if strings.TrimSpace(params.ConsumerName) == "" || strings.TrimSpace(params.MessageID) == "" {
		return nil, false, errors.New("callback_delivery_repository: consumer and message identity are required")
	}

	now := time.Now().UTC()
	delivery := *params.Delivery
	if delivery.Payload == nil {
		delivery.Payload = []byte{}
	}
	if delivery.Status == "" {
		delivery.Status = domain.CallbackDeliveryPending
	}
	if delivery.OutcomeGeneration == 0 {
		delivery.OutcomeGeneration = 1
	}
	if delivery.CreatedAt.IsZero() {
		delivery.CreatedAt = now
	}
	if delivery.NextAttemptAt == nil && delivery.Status == domain.CallbackDeliveryPending {
		delivery.NextAttemptAt = timePtr(now)
	}
	delivery.UpdatedAt = now
	created := false

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fresh, err := r.inbox.Begin(ctx, tx, params.ConsumerName, params.MessageID, params.Topic, params.MessageType)
		if err != nil || !fresh {
			return err
		}
		result := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(callbackToRow(&delivery))
		if result.Error != nil {
			return fmt.Errorf("callback_delivery_repository: schedule %s: %w", delivery.EventID, result.Error)
		}
		if result.RowsAffected == 0 {
			return r.inbox.MarkProcessed(ctx, tx, params.ConsumerName, params.MessageID)
		}
		created = true
		if params.Result != nil {
			if err := r.enqueueResult(ctx, tx, *params.Result); err != nil {
				return err
			}
		}
		return r.inbox.MarkProcessed(ctx, tx, params.ConsumerName, params.MessageID)
	})
	if err != nil {
		return nil, false, err
	}
	stored, err := r.GetByEventID(ctx, delivery.EventID)
	if err != nil {
		return nil, false, err
	}
	if stored == nil {
		return nil, false, fmt.Errorf("callback_delivery_repository: inbox was processed but delivery %s is missing", delivery.EventID)
	}
	return stored, created, nil
}

func (r *callbackDeliveryRepository) GetByEventID(ctx context.Context, eventID string) (*domain.CallbackDelivery, error) {
	var row CallbackDeliveryRow
	err := r.db.WithContext(ctx).Where("event_id = ?", eventID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("callback_delivery_repository: load %s: %w", eventID, err)
	}
	return callbackToDomain(&row), nil
}

func (r *callbackDeliveryRepository) ClaimNext(
	ctx context.Context,
	workerID string,
	now time.Time,
	leaseDuration time.Duration,
) (*domain.CallbackDelivery, error) {
	if strings.TrimSpace(workerID) == "" || leaseDuration <= 0 {
		return nil, errors.New("callback_delivery_repository: worker ID and positive lease duration are required")
	}
	leaseExpiresAt := now.Add(leaseDuration)
	var row CallbackDeliveryRow
	result := r.db.WithContext(ctx).Raw(`
		WITH picked AS (
			SELECT deliveries.transaction_id
			FROM public.callback_deliveries AS deliveries
			WHERE (deliveries.status IN ('PENDING', 'RETRY_SCHEDULED') AND deliveries.next_attempt_at <= ?)
			   OR (deliveries.status = 'IN_FLIGHT' AND deliveries.lease_expires_at < ?)
			ORDER BY deliveries.next_attempt_at, deliveries.created_at, deliveries.transaction_id
			FOR UPDATE OF deliveries SKIP LOCKED
			LIMIT 1
		)
		UPDATE public.callback_deliveries AS deliveries
		SET status = 'IN_FLIGHT', lease_owner = ?, lease_token = deliveries.lease_token + 1,
		    lease_expires_at = ?, last_attempt_at = ?, updated_at = ?
		FROM picked
		WHERE deliveries.transaction_id = picked.transaction_id
		RETURNING deliveries.*`, now, now, workerID, leaseExpiresAt, now, now).Scan(&row)
	if result.Error != nil {
		return nil, fmt.Errorf("callback_delivery_repository: claim next: %w", result.Error)
	}
	if result.RowsAffected == 0 || row.TransactionID == "" {
		return nil, nil
	}
	return callbackToDomain(&row), nil
}

func (r *callbackDeliveryRepository) MarkDelivered(
	ctx context.Context,
	delivery *domain.CallbackDelivery,
	result port.CallbackResultIntent,
	now time.Time,
) error {
	return r.transitionWithResult(ctx, delivery, domain.CallbackDeliveryDelivered, "", result, now, false)
}

func (r *callbackDeliveryRepository) MarkRetry(
	ctx context.Context,
	delivery *domain.CallbackDelivery,
	reason string,
	nextAttemptAt time.Time,
	now time.Time,
) error {
	if delivery == nil || nextAttemptAt.Before(now) {
		return errors.New("callback_delivery_repository: retry requires a delivery and a future schedule")
	}
	result := r.db.WithContext(ctx).Model(&CallbackDeliveryRow{}).
		Where("transaction_id = ? AND status = 'IN_FLIGHT' AND lease_owner = ? AND lease_token = ?",
			delivery.TransactionID, delivery.LeaseOwner, delivery.LeaseToken).
		Updates(map[string]any{
			"status":   domain.CallbackDeliveryRetryScheduled,
			"attempts": gorm.Expr("attempts + 1"), "generation_attempts": gorm.Expr("generation_attempts + 1"),
			"next_attempt_at": nextAttemptAt, "key_version": delivery.KeyVersion,
			"last_error": truncateCallbackError(reason), "lease_owner": "", "lease_expires_at": nil,
			"updated_at": now,
		})
	if result.Error != nil {
		return fmt.Errorf("callback_delivery_repository: schedule retry %s: %w", delivery.EventID, result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrCallbackDeliveryLeaseLost
	}
	return nil
}

func (r *callbackDeliveryRepository) MarkTerminal(
	ctx context.Context,
	delivery *domain.CallbackDelivery,
	reason string,
	result port.CallbackResultIntent,
	now time.Time,
) error {
	return r.transitionWithResult(ctx, delivery, domain.CallbackDeliveryTerminalFailed, reason, result, now, true)
}

func (r *callbackDeliveryRepository) transitionWithResult(
	ctx context.Context,
	delivery *domain.CallbackDelivery,
	status, reason string,
	resultIntent port.CallbackResultIntent,
	now time.Time,
	deadLetter bool,
) error {
	if delivery == nil {
		return errors.New("callback_delivery_repository: delivery is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"status": status, "attempts": gorm.Expr("attempts + 1"),
			"generation_attempts": gorm.Expr("generation_attempts + 1"), "key_version": delivery.KeyVersion,
			"last_error": truncateCallbackError(reason), "next_attempt_at": nil,
			"lease_owner": "", "lease_expires_at": nil, "updated_at": now,
		}
		if deadLetter {
			updates["terminal_at"] = now
		}
		update := tx.WithContext(ctx).Model(&CallbackDeliveryRow{}).
			Where("transaction_id = ? AND status = 'IN_FLIGHT' AND lease_owner = ? AND lease_token = ?",
				delivery.TransactionID, delivery.LeaseOwner, delivery.LeaseToken).
			Updates(updates)
		if update.Error != nil {
			return fmt.Errorf("callback_delivery_repository: complete %s: %w", delivery.EventID, update.Error)
		}
		if update.RowsAffected != 1 {
			return domain.ErrCallbackDeliveryLeaseLost
		}
		if deadLetter {
			dead := deadLetterFromDelivery(delivery, reason, now)
			dead.Attempts = delivery.GenerationAttempts + 1
			if err := tx.WithContext(ctx).Create(dead).Error; err != nil {
				return fmt.Errorf("callback_delivery_repository: persist DLQ %s: %w", delivery.EventID, err)
			}
		}
		return r.enqueueResult(ctx, tx, resultIntent)
	})
}

func (r *callbackDeliveryRepository) ReplayTerminal(
	ctx context.Context,
	request port.CallbackReplayRequest,
) (*domain.CallbackDelivery, error) {
	if strings.TrimSpace(request.EventID) == "" || strings.TrimSpace(request.TenantID) == "" ||
		strings.TrimSpace(request.OperatorID) == "" || strings.TrimSpace(request.Reason) == "" {
		return nil, errors.New("callback_delivery_repository: event, tenant, operator, and replay reason are required")
	}
	if request.AvailableAt.IsZero() {
		request.AvailableAt = time.Now().UTC()
	}
	var replayed CallbackDeliveryRow
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("event_id = ?", request.EventID).First(&replayed).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrCallbackDeliveryNotFound
			}
			return err
		}
		if replayed.TenantID != request.TenantID {
			return domain.ErrTenantOwnership
		}
		if replayed.Status != domain.CallbackDeliveryTerminalFailed {
			return domain.ErrCallbackDeliveryNotDead
		}
		updateDLQ := tx.Model(&CallbackDeadLetterRow{}).
			Where("event_id = ? AND outcome_generation = ? AND status = ?",
				replayed.EventID, replayed.OutcomeGeneration, domain.CallbackDeadLetterOpen).
			Updates(map[string]any{
				"status": domain.CallbackDeadLetterReplayed, "replayed_at": request.AvailableAt,
				"replayed_by": request.OperatorID, "replay_reason": request.Reason,
			})
		if updateDLQ.Error != nil {
			return updateDLQ.Error
		}
		if updateDLQ.RowsAffected != 1 {
			return domain.ErrCallbackDeliveryNotDead
		}
		replayed.Status = domain.CallbackDeliveryRetryScheduled
		replayed.GenerationAttempts = 0
		replayed.NextAttemptAt = timePtr(request.AvailableAt)
		replayed.LeaseOwner = ""
		replayed.LeaseExpiresAt = nil
		replayed.ReplayCount++
		replayed.OutcomeGeneration++
		replayed.LastError = ""
		replayed.TerminalAt = nil
		replayed.UpdatedAt = request.AvailableAt
		if err := tx.Save(&replayed).Error; err != nil {
			return err
		}
		return tx.Create(&AuditLogRow{
			AgentID: replayed.AgentID, TenantID: replayed.TenantID, Action: "REPLAY_CALLBACK_DLQ",
			Details: fmt.Sprintf("event_id=%s generation=%d operator=%s reason=%s",
				replayed.EventID, replayed.OutcomeGeneration, request.OperatorID, truncateCallbackError(request.Reason)),
			CreatedAt: request.AvailableAt,
		}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("callback_delivery_repository: replay %s: %w", request.EventID, err)
	}
	return callbackToDomain(&replayed), nil
}

func (r *callbackDeliveryRepository) enqueueResult(ctx context.Context, tx *gorm.DB, result port.CallbackResultIntent) error {
	return r.outbox.Enqueue(ctx, tx, outbox.EnqueueParams{
		MessageID: result.MessageID, Topic: result.Topic, MessageKey: result.MessageKey,
		MessageType: result.MessageType, AggregateType: result.AggregateType, AggregateID: result.AggregateID,
		TenantID: result.TenantID, CausationID: result.CausationID, Payload: result.Payload,
	})
}

func deadLetterFromDelivery(delivery *domain.CallbackDelivery, reason string, now time.Time) *CallbackDeadLetterRow {
	return &CallbackDeadLetterRow{
		EventID: delivery.EventID, TransactionID: delivery.TransactionID, AgentID: delivery.AgentID,
		TenantID: delivery.TenantID, OutcomeGeneration: delivery.OutcomeGeneration,
		Attempts: delivery.GenerationAttempts, Reason: truncateCallbackError(reason),
		Status: domain.CallbackDeadLetterOpen, CreatedAt: now,
	}
}

func truncateCallbackError(value string) string {
	runes := []rune(value)
	if len(runes) <= 4096 {
		return value
	}
	return string(runes[:4096])
}

func timePtr(value time.Time) *time.Time { return &value }

var _ port.CallbackDeliveryRepository = (*callbackDeliveryRepository)(nil)
