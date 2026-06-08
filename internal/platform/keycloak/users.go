package keycloak

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
)

func (k *keycloak) GetUser(ctx context.Context, accessToken string) (*gocloak.UserInfo, error) {
	token, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := k.Instance.GetUserInfo(ctx, token.AccessToken, k.Realm)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (k *keycloak) GetUserByID(ctx context.Context, userID string) (*gocloak.User, error) {
	token, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := k.Instance.GetUserByID(ctx, token.AccessToken, k.Realm, userID)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (k *keycloak) CreateNewUser(ctx context.Context, user gocloak.User) (string, error) {
	token, err := k.AuthenticateClient(ctx)
	if err != nil {
		return "", err
	}
	rs, err := k.Instance.CreateUser(ctx, token.AccessToken, k.Realm, user)
	if err != nil {
		return "", err
	}
	return rs, nil
}

func (k *keycloak) GetClientUserRoles(ctx context.Context, userID string) ([]*gocloak.Role, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := k.Instance.GetClientRolesByUserID(ctx, client.AccessToken, k.Realm, client.ID, userID)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (k *keycloak) GetRealmUserRoles(ctx context.Context, userID string) ([]*gocloak.Role, error) {
	token, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := k.Instance.GetRealmRolesByUserID(ctx, token.AccessToken, k.Realm, userID)
	if err != nil {
		return nil, err
	}
	return rs, nil
}
