package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *platformpostgres.DB) port.TransactionRepository {
	return &transactionRepository{db: db.DB}
}

func NewPaymentV2Repository(db *platformpostgres.DB) port.PaymentV2Repository {
	return &transactionRepository{db: db.DB}
}

func (r *transactionRepository) CreateOrGet(ctx context.Context, tx *domain.Transaction) (bool, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	row := toRow(tx)
	query := activeDB.WithContext(ctx)
	if tx.RequestIdempotencyKey != "" {
		query = query.Clauses(clause.OnConflict{
			Columns:     []clause.Column{{Name: "agent_id"}, {Name: "request_idempotency_key"}},
			TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "request_idempotency_key <> ''"}}},
			DoNothing:   true,
		})
	}
	result := query.Create(row)
	if result.Error != nil {
		return false, fmt.Errorf("failed to create transaction: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		var existing TransactionRow
		if err := activeDB.WithContext(ctx).
			Where("agent_id = ? AND request_idempotency_key = ?", tx.AgentID, tx.RequestIdempotencyKey).
			First(&existing).Error; err != nil {
			return false, fmt.Errorf("load idempotent transaction: %w", err)
		}
		if existing.RequestFingerprint != tx.RequestFingerprint {
			return false, domain.ErrInitiationIdempotencyConflict
		}
		*tx = *toDomain(&existing)
		return false, nil
	}
	tx.ID = row.ID
	tx.CreatedAt = row.CreatedAt
	tx.UpdatedAt = row.UpdatedAt
	return true, nil
}

func (r *transactionRepository) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	var row TransactionRow
	if err := activeDB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("failed to find transaction %s: %w", id, err)
	}
	return toDomain(&row), nil
}

func (r *transactionRepository) SaveProviderIntent(ctx context.Context, id, provider, providerReference, providerStatus string) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	result := activeDB.WithContext(ctx).Model(&TransactionRow{}).
		Where("id = ? AND (provider_reference = '' OR provider_reference = ?)", id, providerReference).
		Updates(map[string]interface{}{
			"provider": provider, "provider_reference": providerReference,
			"provider_status": gorm.Expr(
				"CASE WHEN provider_status IN ? THEN provider_status ELSE ? END",
				[]string{string(port.ProviderStatusSucceeded), string(port.ProviderStatusCanceled), string(port.ProviderStatusFailed)},
				providerStatus,
			),
			"last_error": gorm.Expr(
				"CASE WHEN status IN ? THEN last_error ELSE '' END",
				[]string{string(domain.StateCompleted), string(domain.StateFailed)},
			),
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("save provider intent: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("save provider intent: provider reference conflicts with transaction %s", id)
	}
	return nil
}

func (r *transactionRepository) TryRecordProviderEvent(ctx context.Context, event port.ProviderWebhookEvent) (bool, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	row := &ProviderWebhookEventRow{
		Provider: event.Provider, EventID: event.EventID, EventType: event.EventType,
		TransactionID: event.TransactionID, ProviderReference: event.ProviderReference,
		ProviderStatus: string(event.Status), PayloadSHA256: event.PayloadSHA256,
		ProcessingStatus: "RECEIVED", LastError: "", OccurredAt: event.OccurredAt, ReceivedAt: event.ReceivedAt,
	}
	result := activeDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row)
	if result.Error != nil {
		return false, fmt.Errorf("record provider webhook event: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *transactionRepository) ApplyProviderOutcome(
	ctx context.Context,
	transactionID, provider, providerReference, providerStatus string,
	targetState *domain.TransactionState,
	lastError string,
) (*domain.Transaction, bool, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	var row TransactionRow
	if err := activeDB.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&row, "id = ?", transactionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, domain.ErrTransactionNotFound
		}
		return nil, false, fmt.Errorf("load transaction for provider outcome: %w", err)
	}
	if row.ProviderReference != "" && providerReference != "" && row.ProviderReference != providerReference {
		return nil, false, fmt.Errorf("provider reference does not match transaction %s", transactionID)
	}
	if row.Provider != "" && provider != "" && row.Provider != provider {
		return nil, false, fmt.Errorf("provider does not match transaction %s", transactionID)
	}

	effectiveProviderStatus := providerStatus
	currentState := domain.TransactionState(row.Status)
	if (currentState == domain.StateCompleted || currentState == domain.StateFailed) &&
		(targetState == nil || *targetState != domain.StateCompleted) {
		effectiveProviderStatus = row.ProviderStatus
	}
	updates := map[string]interface{}{
		"provider_status": effectiveProviderStatus,
		"updated_at":      time.Now(),
	}
	if row.Provider == "" && provider != "" {
		updates["provider"] = provider
		row.Provider = provider
	}
	if row.ProviderReference == "" && providerReference != "" {
		updates["provider_reference"] = providerReference
		row.ProviderReference = providerReference
	}
	changed := false
	if targetState != nil {
		target := *targetState
		switch target {
		case domain.StateCompleted:
			if domain.TransactionState(row.Status) != domain.StateCompleted {
				updates["status"] = string(target)
				updates["last_error"] = ""
				row.Status = string(target)
				row.LastError = ""
				changed = true
			}
		case domain.StateFailed:
			current := domain.TransactionState(row.Status)
			if current != domain.StateCompleted && current != domain.StateFailed {
				updates["status"] = string(target)
				updates["last_error"] = lastError
				row.Status = string(target)
				row.LastError = lastError
				changed = true
			}
		default:
			return nil, false, fmt.Errorf("unsupported provider outcome state %q", target)
		}
	}
	if err := activeDB.WithContext(ctx).Model(&TransactionRow{}).Where("id = ?", transactionID).Updates(updates).Error; err != nil {
		return nil, false, fmt.Errorf("apply provider outcome: %w", err)
	}
	row.ProviderStatus = effectiveProviderStatus
	row.UpdatedAt = updates["updated_at"].(time.Time)
	return toDomain(&row), changed, nil
}

func (r *transactionRepository) MarkProviderEventProcessed(ctx context.Context, provider, eventID, processingStatus, lastError string) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	now := time.Now()
	result := activeDB.WithContext(ctx).Model(&ProviderWebhookEventRow{}).
		Where("provider = ? AND event_id = ? AND processing_status = 'RECEIVED'", provider, eventID).
		Updates(map[string]interface{}{
			"processing_status": processingStatus, "last_error": lastError, "processed_at": &now,
		})
	if result.Error != nil {
		return fmt.Errorf("mark provider webhook event processed: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("mark provider webhook event processed: event %s/%s was not claimable", provider, eventID)
	}
	return nil
}

func (r *transactionRepository) ClaimForProcessing(ctx context.Context, id, idempotencyKey string, staleBefore time.Time) (*domain.Transaction, bool, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	now := time.Now()
	result := activeDB.WithContext(ctx).
		Model(&TransactionRow{}).
		Where(`id = ? AND (
			status = ? OR (status = ? AND idempotency_key = ?) OR
			(status = ? AND idempotency_key = ? AND processing_started_at < ?)
		)`, id, domain.StatePending, domain.StateUnknown, idempotencyKey, domain.StateProcessing, idempotencyKey, staleBefore).
		Updates(map[string]interface{}{
			"status": string(domain.StateProcessing), "idempotency_key": idempotencyKey,
			"processing_started_at": now, "last_error": "", "updated_at": now,
		})
	if result.Error != nil {
		return nil, false, fmt.Errorf("claim payment transaction: %w", result.Error)
	}
	tx, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, false, err
	}
	if result.RowsAffected == 1 {
		return tx, true, nil
	}
	if tx.IdempotencyKey != "" && tx.IdempotencyKey != idempotencyKey {
		return tx, false, domain.ErrIdempotencyConflict
	}
	return tx, false, nil
}

func (r *transactionRepository) FinalizeProcessing(ctx context.Context, id, idempotencyKey string, status domain.TransactionState, providerReference, lastError string) error {
	if status != domain.StateCompleted && status != domain.StateFailed && status != domain.StateUnknown {
		return fmt.Errorf("invalid final payment state %q", status)
	}
	activeDB := platformpostgres.FromContext(ctx, r.db)
	result := activeDB.WithContext(ctx).Model(&TransactionRow{}).
		Where("id = ? AND idempotency_key = ? AND status = ?", id, idempotencyKey, domain.StateProcessing).
		Updates(map[string]interface{}{
			"status": string(status), "provider_reference": providerReference,
			"last_error": lastError, "processing_started_at": nil, "updated_at": time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("finalize payment transaction: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("finalize payment transaction: %w", errors.New("processing claim was lost"))
	}
	return nil
}

func (r *transactionRepository) ListForReconciliation(ctx context.Context, staleBefore time.Time, limit int) ([]*domain.Transaction, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []TransactionRow
	err := r.db.WithContext(ctx).
		Where("status = ? OR (status = ? AND processing_started_at < ?)", domain.StateUnknown, domain.StateProcessing, staleBefore).
		Order("updated_at, id").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list payments for reconciliation: %w", err)
	}
	transactions := make([]*domain.Transaction, 0, len(rows))
	for index := range rows {
		transactions = append(transactions, toDomain(&rows[index]))
	}
	return transactions, nil
}

func (r *transactionRepository) ResolveReconciliation(ctx context.Context, id, idempotencyKey string, status domain.TransactionState, providerReference, lastError string) (bool, error) {
	if status != domain.StateCompleted && status != domain.StateFailed {
		return false, fmt.Errorf("invalid reconciled payment state %q", status)
	}
	activeDB := platformpostgres.FromContext(ctx, r.db)
	result := activeDB.WithContext(ctx).Model(&TransactionRow{}).
		Where("id = ? AND idempotency_key = ? AND status IN ?", id, idempotencyKey, []domain.TransactionState{domain.StateUnknown, domain.StateProcessing}).
		Updates(map[string]interface{}{
			"status": string(status), "provider_reference": providerReference,
			"last_error": lastError, "processing_started_at": nil, "updated_at": time.Now(),
		})
	if result.Error != nil {
		return false, fmt.Errorf("resolve reconciled payment: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}
