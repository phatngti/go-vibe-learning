package keycloak

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
)

func (k *keycloak) GetRealmRoles(ctx context.Context) ([]*gocloak.Role, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := k.Instance.GetRealmRoles(ctx, client.AccessToken, k.Realm, gocloak.GetRoleParams{})
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (k *keycloak) GetRealmRole(ctx context.Context, roleName string) (*gocloak.Role, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := k.Instance.GetRealmRole(ctx, client.AccessToken, k.Realm, roleName)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (k *keycloak) CreateRealmRole(ctx context.Context, roleName string, description string) (string, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return "", err
	}
	rs, err := k.Instance.CreateRealmRole(ctx, client.AccessToken, k.Realm, gocloak.Role{
		Name:        &roleName,
		Description: &description,
	})
	if err != nil {
		return "", err
	}
	return rs, nil
}

func (k *keycloak) UpdateRealmRole(ctx context.Context, role gocloak.Role) (string, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return "", err
	}
	err = k.Instance.UpdateRealmRole(ctx, client.AccessToken, k.Realm, *role.Name, role)
	if err != nil {
		return "", err
	}
	return *role.Name, nil
}

func (k *keycloak) DeleteRealmRole(ctx context.Context, roleName string) (string, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return "", err
	}
	err = k.Instance.DeleteRealmRole(ctx, client.AccessToken, k.Realm, roleName)
	if err != nil {
		return "", err
	}
	return roleName, nil
}

func (k *keycloak) AssignRealmRoleToUser(ctx context.Context, userID string, roleNames []string) ([]string, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	roles := []gocloak.Role{}
	for _, role := range roleNames {
		roles = append(roles, gocloak.Role{
			Name: &role,
		})
	}
	err = k.Instance.AddRealmRoleToUser(ctx, client.AccessToken, k.Realm, userID, roles)
	if err != nil {
		return nil, err
	}
	return roleNames, nil
}

func (k *keycloak) UnassignRealmRoleOfUser(ctx context.Context, userID string, roleName string) (string, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return "", err
	}
	err = k.Instance.DeleteRealmRoleFromUser(ctx, client.AccessToken, k.Realm, userID, []gocloak.Role{{Name: &roleName}})
	if err != nil {
		return "", err
	}
	return roleName, nil
}
