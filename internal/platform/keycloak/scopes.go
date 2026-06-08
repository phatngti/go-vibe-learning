package keycloak

import (
	"context"
	"fmt"
	"sync"

	"github.com/Nerzal/gocloak/v13"
	"golang.org/x/sync/errgroup"
)

func (k *keycloak) GetScopes(ctx context.Context) ([]*gocloak.ScopeRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	scopes, err := k.Instance.GetScopes(ctx, client.AccessToken, k.Realm, client.ID, gocloak.GetScopeParams{})
	if err != nil {
		return nil, err
	}
	return scopes, nil
}

func (k *keycloak) GetScopeByIDs(ctx context.Context, scopeIDs []string) (*[]gocloak.ScopeRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}

	g, gCtx := errgroup.WithContext(ctx)
	var (
		mu     sync.Mutex
		scopes []gocloak.ScopeRepresentation
	)

	for _, scopeID := range scopeIDs {
		sID := scopeID
		g.Go(func() error {
			scope, err := k.Instance.GetScope(gCtx, client.AccessToken, k.Realm, client.ID, sID)
			if err != nil {
				k.logger.Bg().Error(fmt.Sprintf("get scope by id failed: %s", err))
				return err
			}
			mu.Lock()
			scopes = append(scopes, *scope)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &scopes, nil
}

func (k *keycloak) CreateScope(ctx context.Context, scopeData gocloak.ScopeRepresentation) (*gocloak.ScopeRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	scope, err := k.Instance.CreateScope(ctx, client.AccessToken, k.Realm, client.ID, scopeData)
	if err != nil {
		return nil, err
	}
	return scope, nil
}
