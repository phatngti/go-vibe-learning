package keycloak

import (
	"context"
	"time"
)

func (k *keycloak) AuthenticateClient(ctx context.Context) (*AuthenticateClient, error) {
	k.mu.RLock()
	if k.cachedClient != nil && time.Now().Before(k.cachedExpires.Add(-10*time.Second)) {
		client := k.cachedClient
		k.mu.RUnlock()
		return client, nil
	}
	k.mu.RUnlock()

	k.mu.Lock()
	defer k.mu.Unlock()

	// Double-check under write lock
	if k.cachedClient != nil && time.Now().Before(k.cachedExpires.Add(-10*time.Second)) {
		return k.cachedClient, nil
	}

	jwt, err := k.Instance.LoginClient(ctx, k.ClientId, k.clientSecret, k.Realm)
	if err != nil {
		return nil, err
	}
	admin, err := k.Instance.GetClientRepresentation(ctx, jwt.AccessToken, k.Realm, k.ClientId)
	if err != nil {
		return nil, err
	}

	client := &AuthenticateClient{
		AccessToken: jwt.AccessToken,
		ID:          *admin.ID,
	}

	k.cachedClient = client
	k.cachedExpires = time.Now().Add(time.Duration(jwt.ExpiresIn) * time.Second)

	return client, nil
}
