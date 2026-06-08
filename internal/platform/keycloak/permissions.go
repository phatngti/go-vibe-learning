package keycloak

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
)

func (k *keycloak) GetPermissions(ctx context.Context) ([]*gocloak.PermissionRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	permissions, err := k.Instance.GetPermissions(ctx, client.AccessToken, k.Realm, client.ID, gocloak.GetPermissionParams{})
	if err != nil {
		return nil, err
	}
	return permissions, nil
}

func (k *keycloak) GetPermissionByID(ctx context.Context, permissionID string) (*gocloak.PermissionRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	permission, err := k.Instance.GetPermission(ctx, client.AccessToken, k.Realm, client.ID, permissionID)
	if err != nil {
		return nil, err
	}
	return permission, nil
}

func (k *keycloak) GetPermissionByName(ctx context.Context, permissionName string) (*gocloak.PermissionRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	permissions, err := k.Instance.GetPermissions(ctx, client.AccessToken, k.Realm, client.ID, gocloak.GetPermissionParams{Name: &permissionName})
	if err != nil {
		return nil, err
	}
	if len(permissions) > 0 {
		return permissions[0], nil
	}
	return nil, nil
}

func (k *keycloak) CreatePermission(ctx context.Context, permissionData gocloak.PermissionRepresentation) (*gocloak.PermissionRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	permission, err := k.Instance.CreatePermission(ctx, client.AccessToken, k.Realm, client.ID, permissionData)
	if err != nil {
		return nil, err
	}
	return permission, nil
}

func (k *keycloak) UpdatePermission(ctx context.Context, permissionData gocloak.PermissionRepresentation) error {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return err
	}
	err = k.Instance.UpdatePermission(ctx, client.AccessToken, k.Realm, client.ID, permissionData)
	if err != nil {
		return err
	}
	return nil
}
