package gatewayauth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type audience string

func (a *audience) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*a = audience(single)
		return nil
	}

	var multiple []string
	if err := json.Unmarshal(data, &multiple); err == nil {
		*a = audience(strings.Join(multiple, ","))
		return nil
	}

	return fmt.Errorf("aud must be a string or string array")
}

func DecodeUserPayload(payload string) (User, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return User{}, fmt.Errorf("missing gateway JWT payload")
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(payload)
		if err != nil {
			return User{}, fmt.Errorf("decode gateway JWT payload: %w", err)
		}
	}

	var claim userClaim
	if err := json.Unmarshal(raw, &claim); err != nil {
		return User{}, fmt.Errorf("parse gateway JWT payload: %w", err)
	}

	user := claim.toUser()
	if user.ID == "" {
		return User{}, fmt.Errorf("gateway JWT payload missing subject")
	}
	return user, nil
}

func (c userClaim) toUser() User {
	resources := make([]ResourceAccess, 0, len(c.Authorization.Permissions))
	for _, p := range c.Authorization.Permissions {
		resources = append(resources, ResourceAccess{
			ResourceID:   p.RsID,
			ResourceName: p.RsName,
			Scopes:       p.Scopes,
		})
	}

	return User{
		ID:             c.Sub,
		TenantID:       strings.TrimSpace(c.TenantID),
		AgentID:        strings.TrimSpace(c.AgentID),
		Username:       c.PreferredUsername,
		FirstName:      c.GivenName,
		LastName:       c.FamilyName,
		Email:          c.Email,
		Scope:          c.Scope,
		Scopes:         strings.Fields(c.Scope),
		Roles:          c.RealmAccess.Roles,
		Audience:       string(c.Audience),
		Issuer:         c.Issuer,
		EmailVerified:  c.EmailVerified,
		ResourceAccess: resources,
	}
}
