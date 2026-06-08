package domain

type User struct {
	ID             string           `json:"id"`
	Username       string           `json:"username"`
	FirstName      string           `json:"first_name"`
	LastName       string           `json:"last_name"`
	Email          string           `json:"email"`
	Scope          string           `json:"scope"`
	Roles          []string         `json:"roles"`
	Audience       string           `json:"aud"`
	EmailVerified  bool             `json:"email_verified"`
	ResourceAccess []ResourceAccess `json:"resource_access"`
}

type ResourceAccess struct {
	ResourceID   string   `json:"resource_id"`
	ResourceName string   `json:"resource_name"`
	Scopes       []string `json:"scopes"`
}
