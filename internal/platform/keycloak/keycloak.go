// File: internal/platform/keycloak/keycloak.go
// Description: Client interface and implementation setup for interacting with Keycloak Identity Provider.

package keycloak

import (
	"context"

	"github/master_go_cqrs/internal/platform/config"
	"github/master_go_cqrs/internal/platform/observability/logger"

	"github.com/Nerzal/gocloak/v13"
)

type (
	Client interface {
		RegisterUser(ctx context.Context, user gocloak.User) (string, error)
		LoginUser(ctx context.Context, username, password string) (*gocloak.JWT, error)
		RefreshToken(ctx context.Context, refreshToken string) (*gocloak.JWT, error)
		// GetUsers retrieves users from Keycloak using admin API (requires service account).
		GetUsers(ctx context.Context, params gocloak.GetUsersParams) ([]*gocloak.User, error)
		// GetUserByID retrieves a single user from Keycloak by their Keycloak ID.
		GetUserByID(ctx context.Context, userID string) (*gocloak.User, error)
	}
)

type keycloak struct {
	Instance     *gocloak.GoCloak
	Realm        string
	ClientId     string
	clientSecret string
	logger       logger.Factory
}

func NewClient(cfg *config.Keycloak, logger logger.Factory) Client {
	logger.Bg().Info("Creating Keycloak connection")
	client := gocloak.NewClient(cfg.Endpoint)

	return &keycloak{
		logger:       logger,
		Instance:     client,
		Realm:        cfg.Realm,
		ClientId:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
	}

}
