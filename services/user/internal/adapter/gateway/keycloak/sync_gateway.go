package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	platformkeycloak "github/master_go_cqrs/internal/platform/keycloak"
	"github/master_go_cqrs/services/user/internal/application/port"
	"github/master_go_cqrs/services/user/internal/domain"

	"github.com/Nerzal/gocloak/v13"
)

const providerName = "keycloak"

// SyncGateway is the Keycloak implementation of port.SyncGateway.
// It uses the Keycloak Admin API to fetch users and converts them to domain models.
type SyncGateway struct {
	client platformkeycloak.Client
}

// NewSyncGateway creates a new Keycloak-backed SyncGateway.
func NewSyncGateway(client platformkeycloak.Client) port.SyncGateway {
	return &SyncGateway{client: client}
}

func (g *SyncGateway) FetchAllUsers(ctx context.Context) ([]domain.UserIdentity, error) {
	const compatibilityPageSize = 200
	users := make([]domain.UserIdentity, 0, compatibilityPageSize)
	cursor := 0
	for {
		page, err := g.FetchUsersPage(ctx, cursor, compatibilityPageSize)
		if err != nil {
			return nil, fmt.Errorf("sync_gateway: fetch all users: %w", err)
		}
		users = append(users, page.Users...)
		if page.Done {
			return users, nil
		}
		if page.NextCursor <= cursor {
			return nil, fmt.Errorf("sync_gateway: provider cursor did not advance from %d", cursor)
		}
		cursor = page.NextCursor
	}
}

func (g *SyncGateway) FetchUsersPage(ctx context.Context, cursor, limit int) (port.UserSyncPage, error) {
	if cursor < 0 {
		return port.UserSyncPage{}, fmt.Errorf("sync_gateway: cursor cannot be negative")
	}
	if limit <= 0 || limit > 1000 {
		return port.UserSyncPage{}, fmt.Errorf("sync_gateway: page size must be between 1 and 1000")
	}

	first := cursor
	max := limit
	kcUsers, err := g.client.GetUsers(ctx, gocloak.GetUsersParams{First: &first, Max: &max})
	if err != nil {
		return port.UserSyncPage{}, fmt.Errorf("sync_gateway: fetch users at cursor %d: %w", cursor, err)
	}

	users := make([]domain.UserIdentity, 0, len(kcUsers))
	for _, kcu := range kcUsers {
		users = append(users, mapKeycloakUser(kcu))
	}
	return port.UserSyncPage{
		Users: users, NextCursor: cursor + len(users), Done: len(users) < limit,
	}, nil
}

func (g *SyncGateway) FetchUserByID(ctx context.Context, providerUserID string) (*domain.UserIdentity, error) {
	kcu, err := g.client.GetUserByID(ctx, providerUserID)
	if err != nil {
		return nil, fmt.Errorf("sync_gateway: fetch user %s: %w", providerUserID, err)
	}
	user := mapKeycloakUser(kcu)
	return &user, nil
}

// mapKeycloakUser converts a gocloak.User to a domain.UserIdentity.
func mapKeycloakUser(kcu *gocloak.User) domain.UserIdentity {
	now := time.Now()
	user := domain.UserIdentity{
		IdentityProvider:       providerName,
		IdentityProviderUserID: derefStr(kcu.ID),
		Username:               derefStr(kcu.Username),
		Email:                  kcu.Email,
		FirstName:              kcu.FirstName,
		LastName:               kcu.LastName,
		EmailVerified:          derefBool(kcu.EmailVerified),
		Enabled:                derefBool(kcu.Enabled),
		LastSyncedAt:           now,
	}

	// Serialize Keycloak attributes as JSON.
	if kcu.Attributes != nil {
		if attrJSON, err := json.Marshal(*kcu.Attributes); err == nil {
			user.Attributes = string(attrJSON)
		}
	}

	// Map Keycloak's createdTimestamp (epoch milliseconds).
	if kcu.CreatedTimestamp != nil {
		t := time.UnixMilli(*kcu.CreatedTimestamp)
		user.IdentityCreatedAt = &t
	}

	return user
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
