package dto

// GetUserResponse is the response DTO for a single user identity.
type GetUserResponse struct {
	ID                     string  `json:"id"`
	IdentityProvider       string  `json:"identity_provider"`
	IdentityProviderUserID string  `json:"identity_provider_user_id"`
	Username               string  `json:"username"`
	Email                  *string `json:"email,omitempty"`
	FirstName              *string `json:"first_name,omitempty"`
	LastName               *string `json:"last_name,omitempty"`
	EmailVerified          bool    `json:"email_verified"`
	Enabled                bool    `json:"enabled"`
}

// ListUsersResponse is the response DTO for paginated user listing.
type ListUsersResponse struct {
	Users []*GetUserResponse `json:"users"`
	Total int64              `json:"total"`
}

const (
	GetUserSuccess  = "get_user_success"
	GetUserFailed   = "get_user_failed"
	ListUserSuccess = "list_user_success"
	ListUserFailed  = "list_user_failed"
)

// UserMessageCode maps user management message codes to human-readable messages.
var UserMessageCode = map[string]string{
	GetUserSuccess:  "User retrieved successfully",
	GetUserFailed:   "Failed to retrieve user",
	ListUserSuccess: "Users listed successfully",
	ListUserFailed:  "Failed to list users",
}
