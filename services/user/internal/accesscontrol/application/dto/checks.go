package dto

import "github/master_go_cqrs/services/user/internal/accesscontrol/domain"

const (
	AuthnCheckSuccess = "authn_check_success"
	RBACCheckSuccess  = "rbac_check_success"
	RBACCheckFailed   = "rbac_check_failed"

	InvalidResourceName = "invalid_resource_name"
)

var MessageCode = map[string]string{
	AuthnCheckSuccess:   "Authentication check success",
	RBACCheckSuccess:    "RBAC check success",
	RBACCheckFailed:     "RBAC check failed",
	InvalidResourceName: "Invalid resource name",
}

type CheckRBACRequestDTO struct {
	Resource string   `json:"resource" form:"resource" validate:"required" message:"invalid_resource_name" trace:"resource_name"`
	Scopes   []string `json:"scopes" form:"scopes" trace:"resource_scopes"`
}

type CheckRBACRequest struct {
	Resource string
	Scopes   []string
}

type AuthnCheckDataDTO struct {
	Authenticated bool        `json:"authenticated" example:"true"`
	User          domain.User `json:"user"`
}

type AuthnCheckResponseDTO struct {
	Code    string            `json:"code" example:"authn_check_success"`
	Message string            `json:"message" example:"Authentication check success"`
	Data    AuthnCheckDataDTO `json:"data"`
}

type RBACCheckDataDTO struct {
	Allowed  bool     `json:"allowed" example:"true"`
	Resource string   `json:"resource" example:"payment"`
	Scopes   []string `json:"scopes"`
}

type RBACCheckResponseDTO struct {
	Code    string           `json:"code" example:"rbac_check_success"`
	Message string           `json:"message" example:"RBAC check success"`
	Data    RBACCheckDataDTO `json:"data"`
}
