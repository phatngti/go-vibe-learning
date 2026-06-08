package keycloak

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
)

func (k *keycloak) GetResources(ctx context.Context) ([]*gocloak.ResourceRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	resources, err := k.Instance.GetResources(ctx, client.AccessToken, k.Realm, client.ID, gocloak.GetResourceParams{})
	if err != nil {
		return nil, err
	}
	return resources, nil
}

func (k *keycloak) GetResourceByID(ctx context.Context, resourceID string) (*gocloak.ResourceRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	resource, err := k.Instance.GetResourceClient(ctx, client.AccessToken, k.Realm, resourceID)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

func (k *keycloak) CreateResource(ctx context.Context, resourceData gocloak.ResourceRepresentation) (*gocloak.ResourceRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	resource, err := k.Instance.CreateResource(ctx, client.AccessToken, k.Realm, client.ID, resourceData)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

func (k *keycloak) UpdateResource(ctx context.Context, resourceData gocloak.ResourceRepresentation) error {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return err
	}
	err = k.Instance.UpdateResource(ctx, client.AccessToken, k.Realm, client.ID, resourceData)
	if err != nil {
		return err
	}
	return nil
}

func (k *keycloak) DeleteResource(ctx context.Context, resourceID string) error {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return err
	}
	err = k.Instance.DeleteResource(ctx, client.AccessToken, k.Realm, client.ID, resourceID)
	if err != nil {
		return err
	}
	return nil
}
