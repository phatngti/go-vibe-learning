package dto

import "github/master_go_cqrs/internal/security/gatewayauth"

const (
	LoginSuccess        = "login_success"
	RefreshTokenSuccess = "refresh_token_success"
	LogoutSuccess       = "logout_success"
	InfoSuccess         = "info_success"
	VerifyTokenSuccess  = "verify_token_success"

	LoginFailed        = "login_failed"
	RefreshTokenFailed = "refresh_token_failed"
	LogoutFailed       = "logout_failed"
	InfoFailed         = "info_failed"
	VerifyTokenFailed  = "verify_token_failed"

	UsernameRequired = "username_required"
	InvalidUsername  = "invalid_username"
	PasswordRequired = "password_required"
	InvalidPassword  = "invalid_password"
)

var MessageCode = map[string]string{
	LoginSuccess:        "Login success",
	RefreshTokenSuccess: "Refresh token success",
	LogoutSuccess:       "Logout success",
	InfoSuccess:         "Info success",
	VerifyTokenSuccess:  "Verify token success",

	LoginFailed:        "Login failed",
	RefreshTokenFailed: "Refresh token failed",
	LogoutFailed:       "Logout failed",
	InfoFailed:         "Info failed",
	VerifyTokenFailed:  "Verify token failed",

	UsernameRequired: "Username is required",
	PasswordRequired: "Password is required",
	InvalidUsername:  "Username is invalid. Min 3 characters, max 20 characters, only letters and numbers",
	InvalidPassword:  "Password is invalid. Min 6 characters, max 20 characters, only letters and numbers and special characters",
}

type LoginRequestDTO struct {
	Username string `json:"username" form:"username" validate:"required,min=3,max=20" message:"username_required,invalid_username" trace:"username"`
	Password string `json:"password" form:"password" validate:"required,min=6,max=20" message:"password_required,invalid_password"`
}

type LoginRequest struct {
	Username string
	Password string
}

type LoginResponseDTO struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type RefreshTokenRequestDTO struct {
	RefreshToken string `json:"refresh_token" form:"refresh_token" validate:"required" message:"refresh_token_required"`
}

type RefreshTokenRequest struct {
	RefreshToken string
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type VerifyTokenResponseDTO struct {
	Valid bool `json:"valid"`
}

type InfoDataDTO struct {
	Info gatewayauth.User `json:"info"`
}

type InfoResponseDTO struct {
	Code    string      `json:"code" example:"info_success"`
	Message string      `json:"message" example:"Info success"`
	Data    InfoDataDTO `json:"data"`
}
