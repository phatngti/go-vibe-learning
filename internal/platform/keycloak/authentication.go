// File: internal/platform/keycloak/authentication.go
// Description: User login, token refresh, and admin API functions utilizing the GoCloak client.

package keycloak

import (
	"context"
	"fmt"

	"github.com/Nerzal/gocloak/v13"
)

func (k *keycloak) LoginUser(ctx context.Context, username, password string) (*gocloak.JWT, error) {
	jwt, err := k.Instance.Login(ctx, k.ClientId, k.clientSecret, k.Realm, username, password)
	if err != nil {
		return nil, err
	}
	return jwt, nil
}

func (k *keycloak) RefreshToken(ctx context.Context, refreshToken string) (*gocloak.JWT, error) {
	jwt, err := k.Instance.RefreshToken(ctx, refreshToken, k.ClientId, k.clientSecret, k.Realm)
	if err != nil {
		return nil, err
	}
	return jwt, nil
}

func (k *keycloak) RegisterUser(ctx context.Context, user gocloak.User) (string, error) {
	token, err := k.adminToken(ctx)
	if err != nil {
		return "", err
	}

	enabled := true
	emailVerified := false
	user.Enabled = &enabled
	user.EmailVerified = &emailVerified

	userID, err := k.Instance.CreateUser(ctx, token, k.Realm, user)
	if err != nil {
		return "", fmt.Errorf("keycloak: create user: %w", err)
	}

	return userID, nil
}

// adminToken obtains an access token using the client credentials grant (service account).
func (k *keycloak) adminToken(ctx context.Context) (string, error) {
	jwt, err := k.Instance.LoginClient(ctx, k.ClientId, k.clientSecret, k.Realm)
	if err != nil {
		return "", fmt.Errorf("keycloak: admin token: %w", err)
	}
	return jwt.AccessToken, nil
}

func (k *keycloak) GetUsers(ctx context.Context, params gocloak.GetUsersParams) ([]*gocloak.User, error) {
	token, err := k.adminToken(ctx)
	if err != nil {
		return nil, err
	}
	users, err := k.Instance.GetUsers(ctx, token, k.Realm, params)
	if err != nil {
		return nil, fmt.Errorf("keycloak: get users: %w", err)
	}
	return users, nil
}

func (k *keycloak) GetUserByID(ctx context.Context, userID string) (*gocloak.User, error) {
	token, err := k.adminToken(ctx)
	if err != nil {
		return nil, err
	}
	user, err := k.Instance.GetUserByID(ctx, token, k.Realm, userID)
	if err != nil {
		return nil, fmt.Errorf("keycloak: get user by id %s: %w", userID, err)
	}
	return user, nil
}
