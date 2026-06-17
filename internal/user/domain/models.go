package domain

import "time"

// UserIdentity is the aggregate root representing a synced user from an
// external identity provider (e.g. Keycloak).
// Maps to the public.user_identities table.
type UserIdentity struct {
	ID                     string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	IdentityProvider       string     `gorm:"type:text;not null;default:'keycloak'"`
	IdentityProviderUserID string     `gorm:"type:text;not null"`
	Username               string     `gorm:"type:text;not null"`
	Email                  *string    `gorm:"type:text"`
	FirstName              *string    `gorm:"type:text"`
	LastName               *string    `gorm:"type:text"`
	EmailVerified          bool       `gorm:"type:boolean;not null;default:false"`
	Enabled                bool       `gorm:"type:boolean;not null;default:true"`
	Attributes             string     `gorm:"type:jsonb;not null;default:'{}'"`
	IdentityCreatedAt      *time.Time `gorm:"type:timestamptz"`
	LastSyncedAt           time.Time  `gorm:"type:timestamptz;not null;default:now()"`
	CreatedAt              time.Time  `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt              time.Time  `gorm:"type:timestamptz;not null;default:now()"`
}

func (UserIdentity) TableName() string {
	return "public.user_identities"
}
