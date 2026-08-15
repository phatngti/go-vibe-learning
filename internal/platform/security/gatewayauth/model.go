package gatewayauth

const PayloadHeader = "x-gateway-jwt-payload"

type User struct {
	ID             string           `json:"id"`
	TenantID       string           `json:"tenant_id"`
	AgentID        string           `json:"agent_id"`
	Username       string           `json:"username"`
	FirstName      string           `json:"first_name"`
	LastName       string           `json:"last_name"`
	Email          string           `json:"email"`
	Scope          string           `json:"scope"`
	Scopes         []string         `json:"scopes"`
	Roles          []string         `json:"roles"`
	Audience       string           `json:"aud"`
	Issuer         string           `json:"iss"`
	EmailVerified  bool             `json:"email_verified"`
	ResourceAccess []ResourceAccess `json:"resource_access"`
}

type ResourceAccess struct {
	ResourceID   string   `json:"resource_id"`
	ResourceName string   `json:"resource_name"`
	Scopes       []string `json:"scopes"`
}

type userClaim struct {
	Sub               string        `json:"sub"`
	TenantID          string        `json:"tenant_id"`
	AgentID           string        `json:"agent_id"`
	PreferredUsername string        `json:"preferred_username"`
	GivenName         string        `json:"given_name"`
	FamilyName        string        `json:"family_name"`
	Email             string        `json:"email"`
	Scope             string        `json:"scope"`
	Authorization     authorization `json:"authorization"`
	RealmAccess       realmAccess   `json:"realm_access"`
	Audience          audience      `json:"aud"`
	Issuer            string        `json:"iss"`
	EmailVerified     bool          `json:"email_verified"`
}

type authorization struct {
	Permissions []permission `json:"permissions"`
}

type permission struct {
	RsID   string   `json:"rsid"`
	RsName string   `json:"rsname"`
	Scopes []string `json:"scopes"`
}

type realmAccess struct {
	Roles []string `json:"roles"`
}
