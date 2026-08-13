package http

// LoginRequestDTO is the HTTP-specific request payload for user login.
// Transport-layer tags (json, form, validate, message, trace) live here,
// NOT in the application DTO layer.
type LoginRequestDTO struct {
	Username string `json:"username" form:"username" validate:"required,min=3,max=20" message:"username_required,invalid_username" trace:"username"`
	Password string `json:"password" form:"password" validate:"required,min=6,max=20" message:"password_required,invalid_password"`
}

// RefreshTokenRequestDTO is the HTTP-specific request payload for token refresh.
type RefreshTokenRequestDTO struct {
	RefreshToken string `json:"refresh_token" form:"refresh_token" validate:"required" message:"refresh_token_required"`
}

// RegisterUserRequestDTO is the HTTP-specific request payload for user registration.
type RegisterUserRequestDTO struct {
	Email    string `json:"email" form:"email" validate:"required" message:"email_required" trace:"email"`
	Password string `json:"password" form:"password" validate:"required" message:"password_required"`
}

// ResponseType is used for Swagger documentation.
type ResponseType struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}
