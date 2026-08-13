package postgres

import "time"

// UserIdentityRow is the GORM persistence model for the user_identities table.
// This struct owns all GORM tags and database concerns, keeping the domain
// model free of infrastructure dependencies.
type UserIdentityRow struct {
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

func (UserIdentityRow) TableName() string {
	return "public.user_identities"
}
