package postgres

import (
	"context"
	"fmt"
	"time"

	"github/master_go_cqrs/internal/user/domain"
	platformpostgres "github/master_go_cqrs/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new GORM-backed UserRepository.
func NewUserRepository(db *platformpostgres.DB) domain.UserRepository {
	return &userRepository{db: db.DB}
}

func (r *userRepository) Upsert(ctx context.Context, db *gorm.DB, user *domain.UserIdentity) error {
	activeDB := r.db
	if db != nil {
		activeDB = db
	}
	now := time.Now()
	user.LastSyncedAt = now
	user.UpdatedAt = now
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
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
	}).Create(user).Error
	if err != nil {
		return fmt.Errorf("user_repository: upsert user %s: %w", user.IdentityProviderUserID, err)
	}
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*domain.UserIdentity, error) {
	var user domain.UserIdentity
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user_repository: get by id %s: %w", id, err)
	}
	return &user, nil
}

func (r *userRepository) GetByProviderUserID(ctx context.Context, provider, providerUserID string) (*domain.UserIdentity, error) {
	var user domain.UserIdentity
	err := r.db.WithContext(ctx).
		Where("identity_provider = ? AND identity_provider_user_id = ?", provider, providerUserID).
		First(&user).Error
	if err != nil {
		return nil, fmt.Errorf("user_repository: get by provider user id %s/%s: %w", provider, providerUserID, err)
	}
	return &user, nil
}

func (r *userRepository) GetByUsername(ctx context.Context, provider, username string) (*domain.UserIdentity, error) {
	var user domain.UserIdentity
	err := r.db.WithContext(ctx).
		Where("identity_provider = ? AND lower(username) = lower(?)", provider, username).
		First(&user).Error
	if err != nil {
		return nil, fmt.Errorf("user_repository: get by username %s/%s: %w", provider, username, err)
	}
	return &user, nil
}

func (r *userRepository) ListAll(ctx context.Context, limit, offset int) ([]*domain.UserIdentity, int64, error) {
	var users []*domain.UserIdentity
	var total int64

	if err := r.db.WithContext(ctx).Model(&domain.UserIdentity{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("user_repository: count users: %w", err)
	}

	err := r.db.WithContext(ctx).
		Order("created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error
	if err != nil {
		return nil, 0, fmt.Errorf("user_repository: list users: %w", err)
	}
	return users, total, nil
}
