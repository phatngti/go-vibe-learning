package dto

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

	RegisterSuccess = "register_success"
	RegisterFailed  = "register_failed"

	EmailRequired = "email_required"
	InvalidEmail  = "invalid_email"
)

// MessageCode maps message codes to human-readable messages.
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

	EmailRequired: "Email is required",
	InvalidEmail:  "Email is invalid",

	RegisterSuccess: "register_success",
	RegisterFailed:  "register_failed",
}

// LoginRequest is the application-layer command for user login.
// No transport tags — those belong in delivery/http/request.go.
type LoginRequest struct {
	Username string
	Password string
}

// LoginResponse is the application-layer result of a login.
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenRequest is the application-layer command for token refresh.
type RefreshTokenRequest struct {
	RefreshToken string
}

// RefreshTokenResponse is the application-layer result of a token refresh.
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// VerifyTokenResponse is the application-layer result of token verification.
type VerifyTokenResponse struct {
	Valid bool `json:"valid"`
}

// InfoDataDTO wraps user info for the response.
type InfoDataDTO struct {
	Info interface{} `json:"info"`
}

// RegisterRequest is the application-layer command for user registration.
// No transport tags — those belong in delivery/http/request.go.
type RegisterRequest struct {
	Email    string
	Password string
}

// RegisterResponse is the application-layer result of registration.
type RegisterResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
