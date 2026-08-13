package postgres

import "github/master_go_cqrs/services/user/internal/domain"

// toDomain converts a persistence row to a domain entity.
func toDomain(row *UserIdentityRow) *domain.UserIdentity {
	return &domain.UserIdentity{
		ID:                     row.ID,
		IdentityProvider:       row.IdentityProvider,
		IdentityProviderUserID: row.IdentityProviderUserID,
		Username:               row.Username,
		Email:                  row.Email,
		FirstName:              row.FirstName,
		LastName:               row.LastName,
		EmailVerified:          row.EmailVerified,
		Enabled:                row.Enabled,
		Attributes:             row.Attributes,
		IdentityCreatedAt:      row.IdentityCreatedAt,
		LastSyncedAt:           row.LastSyncedAt,
		CreatedAt:              row.CreatedAt,
		UpdatedAt:              row.UpdatedAt,
	}
}

// toRow converts a domain entity to a persistence row.
func toRow(entity *domain.UserIdentity) *UserIdentityRow {
	return &UserIdentityRow{
		ID:                     entity.ID,
		IdentityProvider:       entity.IdentityProvider,
		IdentityProviderUserID: entity.IdentityProviderUserID,
		Username:               entity.Username,
		Email:                  entity.Email,
		FirstName:              entity.FirstName,
		LastName:               entity.LastName,
		EmailVerified:          entity.EmailVerified,
		Enabled:                entity.Enabled,
		Attributes:             entity.Attributes,
		IdentityCreatedAt:      entity.IdentityCreatedAt,
		LastSyncedAt:           entity.LastSyncedAt,
		CreatedAt:              entity.CreatedAt,
		UpdatedAt:              entity.UpdatedAt,
	}
}

// toDomainSlice converts a slice of persistence rows to domain entities.
func toDomainSlice(rows []*UserIdentityRow) []*domain.UserIdentity {
	entities := make([]*domain.UserIdentity, 0, len(rows))
	for _, row := range rows {
		entities = append(entities, toDomain(row))
	}
	return entities
}
