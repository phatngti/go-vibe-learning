package keycloak

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
)

func (k *keycloak) GetPolicies(ctx context.Context) ([]*gocloak.PolicyRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	policies, err := k.Instance.GetPolicies(ctx, client.AccessToken, k.Realm, client.ID, gocloak.GetPolicyParams{})
	if err != nil {
		return nil, err
	}
	return policies, nil
}

func (k *keycloak) GetPolicyByID(ctx context.Context, policyID string) (*gocloak.PolicyRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	policy, err := k.Instance.GetPolicy(ctx, client.AccessToken, k.Realm, client.ID, policyID)
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func (k *keycloak) GetPolicyByName(ctx context.Context, policyName string) (*gocloak.PolicyRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	policies, err := k.Instance.GetPolicies(ctx, client.AccessToken, k.Realm, client.ID, gocloak.GetPolicyParams{Name: &policyName})
	if err != nil {
		return nil, err
	}
	if len(policies) > 0 {
		return policies[0], nil
	}
	return nil, nil
}

func (k *keycloak) CreatePolicy(ctx context.Context, policyData gocloak.PolicyRepresentation) (*gocloak.PolicyRepresentation, error) {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return nil, err
	}
	policy, err := k.Instance.CreatePolicy(ctx, client.AccessToken, k.Realm, client.ID, policyData)
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func (k *keycloak) UpdatePolicy(ctx context.Context, policyData gocloak.PolicyRepresentation) error {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return err
	}
	err = k.Instance.UpdatePolicy(ctx, client.AccessToken, k.Realm, client.ID, policyData)
	if err != nil {
		return err
	}
	return nil
}

func (k *keycloak) DeletePolicy(ctx context.Context, policyID string) error {
	client, err := k.AuthenticateClient(ctx)
	if err != nil {
		return err
	}
	err = k.Instance.DeletePolicy(ctx, client.AccessToken, k.Realm, client.ID, policyID)
	if err != nil {
		return err
	}
	return nil
}
