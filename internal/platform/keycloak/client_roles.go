package keycloak

import (
	"context"
	"fmt"
	"sync"

	"github.com/Nerzal/gocloak/v13"
	"golang.org/x/sync/errgroup"
)

func (k *keycloak) GetClientRoles(ctx context.Context) ([]*gocloak.Role, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {

		return nil, err
	}

	rs, err := k.Instance.GetClientRoles(ctx, client.AccessToken, k.Realm, client.ID, gocloak.GetRoleParams{})
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (k *keycloak) GetClientRoleByName(ctx context.Context, roleName string) (*gocloak.Role, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := k.Instance.GetClientRole(ctx, client.AccessToken, k.Realm, client.ID, roleName)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (k *keycloak) GetClientRoleByNames(ctx context.Context, roleName []string) (*[]gocloak.Role, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}

	g, gCtx := errgroup.WithContext(ctx)
	var (
		mu sync.Mutex
		rs []gocloak.Role
	)

	for _, rName := range roleName {
		name := rName
		g.Go(func() error {
			role, err := k.Instance.GetClientRole(gCtx, client.AccessToken, k.Realm, client.ID, name)
			if err != nil {
				k.logger.Bg().Error(fmt.Sprintf("get client role by id failed: %s", err))
				return err
			}
			mu.Lock()
			rs = append(rs, *role)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return &rs, nil
}

func (k *keycloak) GetClientRole(ctx context.Context, roleName string) (*gocloak.Role, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := k.Instance.GetClientRole(ctx, client.AccessToken, k.Realm, client.ID, roleName)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (k *keycloak) CreateClientRole(ctx context.Context, roleName string, description string) (string, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return "", err
	}
	rs, err := k.Instance.CreateClientRole(ctx, client.AccessToken, k.Realm, client.ID, gocloak.Role{
		Name:        &roleName,
		Description: &description,
	})
	if err != nil {
		return "", err
	}
	return rs, nil
}

func (k *keycloak) UpdateClientRole(ctx context.Context, role gocloak.Role) (string, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return "", err
	}
	err = k.Instance.UpdateRole(ctx, client.AccessToken, k.Realm, client.ID, role)
	if err != nil {
		return "", err
	}
	return *role.Name, nil
}

func (k *keycloak) DeleteClientRole(ctx context.Context, roleName string) (string, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return "", err
	}
	err = k.Instance.DeleteClientRole(ctx, client.AccessToken, k.Realm, client.ID, roleName)
	if err != nil {
		return "", err
	}
	return roleName, nil
}

func (k *keycloak) AssignClientRoleToUser(ctx context.Context, userID string, roles []gocloak.Role) ([]string, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}

	err = k.Instance.AddClientRolesToUser(ctx, client.AccessToken, k.Realm, client.ID, userID, roles)
	if err != nil {
		k.logger.Bg().Error(fmt.Sprintf("assign role to user failed: %s", err))
		return nil, err
	}
	rs := []string{}
	for _, role := range roles {
		rs = append(rs, *role.Name)
	}
	return rs, nil
}

func (k *keycloak) UnassignClientRoleOfUser(ctx context.Context, userID string, roleName string) (string, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return "", err
	}
	err = k.Instance.DeleteClientRolesFromUser(ctx, client.AccessToken, k.Realm, k.ClientId, userID, []gocloak.Role{{Name: &roleName}})
	if err != nil {
		return "", err
	}
	return roleName, nil
}
