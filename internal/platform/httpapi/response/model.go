package response

type (
	Response struct {
		Code    string `json:"code"`
		Message string `json:"message,omitempty"`
		Data    any    `json:"data,omitempty"`
	}

	ErrorResponse struct {
		Code    string         `json:"code"`
		Message string         `json:"message,omitempty"`
		Errors  []ErrorDetails `json:"errors"`
	}

	ErrorDetails struct {
		Code    string `json:"code"`
		Message string `json:"message,omitempty"`
		Data    any    `json:"data,omitempty"`
	}

	ErrorDetailsInfo struct {
		Code string `json:"code"`
		Data any    `json:"data,omitempty"`
	}
)
