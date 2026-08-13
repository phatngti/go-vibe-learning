package postgres

import (
	"context"
	"fmt"
	"time"

	platformpostgres "github/master_go_cqrs/internal/platform/postgres"
	"github/master_go_cqrs/services/user/internal/application/port"
	"github/master_go_cqrs/services/user/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new GORM-backed UserRepository.
func NewUserRepository(db *platformpostgres.DB) port.UserRepository {
	return &userRepository{db: db.DB}
}

func (r *userRepository) Upsert(ctx context.Context, user *domain.UserIdentity) error {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	row := toRow(user)

	now := time.Now()
	row.LastSyncedAt = now
	row.UpdatedAt = now
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}

	err := activeDB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "identity_provider"},
			{Name: "identity_provider_user_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"username", "email", "first_name", "last_name",
			"email_verified", "enabled", "attributes",
			"identity_created_at", "last_synced_at", "updated_at",
		}),
	}).Create(row).Error
	if err != nil {
		return fmt.Errorf("user_repository: upsert user %s: %w", user.IdentityProviderUserID, err)
	}

	// Write back the generated ID to the domain entity.
	user.ID = row.ID
	user.LastSyncedAt = row.LastSyncedAt
	user.UpdatedAt = row.UpdatedAt
	user.CreatedAt = row.CreatedAt
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*domain.UserIdentity, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	var row UserIdentityRow
	if err := activeDB.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, fmt.Errorf("user_repository: get by id %s: %w", id, err)
	}
	return toDomain(&row), nil
}

func (r *userRepository) GetByProviderUserID(ctx context.Context, provider, providerUserID string) (*domain.UserIdentity, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	var row UserIdentityRow
	err := activeDB.WithContext(ctx).
		Where("identity_provider = ? AND identity_provider_user_id = ?", provider, providerUserID).
		First(&row).Error
	if err != nil {
		return nil, fmt.Errorf("user_repository: get by provider user id %s/%s: %w", provider, providerUserID, err)
	}
	return toDomain(&row), nil
}

func (r *userRepository) GetByUsername(ctx context.Context, provider, username string) (*domain.UserIdentity, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	var row UserIdentityRow
	err := activeDB.WithContext(ctx).
		Where("identity_provider = ? AND lower(username) = lower(?)", provider, username).
		First(&row).Error
	if err != nil {
		return nil, fmt.Errorf("user_repository: get by username %s/%s: %w", provider, username, err)
	}
	return toDomain(&row), nil
}

func (r *userRepository) ListAll(ctx context.Context, limit, offset int) ([]*domain.UserIdentity, int64, error) {
	activeDB := platformpostgres.FromContext(ctx, r.db)
	var rows []*UserIdentityRow
	var total int64

	if err := activeDB.WithContext(ctx).Model(&UserIdentityRow{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("user_repository: count users: %w", err)
	}

	err := activeDB.WithContext(ctx).
		Order("created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("user_repository: list users: %w", err)
	}
	return toDomainSlice(rows), total, nil
}
