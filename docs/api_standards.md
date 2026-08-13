# REST API Standards & Guidelines

This document outlines the API design standards, endpoint conventions, standard error formats, authentication headers, and Swagger annotation practices implemented across all HTTP REST services in **Master Go CQRS**.

---

## 1. Endpoint Design & URL Conventions

### Path Prefix & Versioning
All HTTP REST endpoints must be versioned and rooted under `/api/v1/`:

```text
GET  /api/v1/<service-or-resource>
POST /api/v1/<service-or-resource>
```

### Resource Naming & HTTP Verbs
- Use lowercase, kebab-case or plural nouns for resources (e.g. `/api/v1/payment/checkout`, `/api/v1/author/profile`).
- Use standard HTTP verbs according to their semantic meaning:

| Verb | Usage | Idempotent | Body Allowed |
| --- | --- | --- | --- |
| `GET` | Retrieve resource details or list query results | Yes | No |
| `POST` | Create a new aggregate or invoke a state-changing Command | No | Yes |
| `PUT` | Fully replace an existing resource | Yes | Yes |
| `PATCH` | Partially update a resource | No | Yes |
| `DELETE` | Remove or disable a resource | Yes | Optional |

---

## 2. Standardized HTTP Response Format

### Success Responses

- **Resource Created (`201 Created`)**: Returned on successful command creation.
- **Success (`200 OK`)**: Returned for successful read queries or state mutations.

```json
{
  "transaction_id": "tx_987654321",
  "status": "SUCCESS",
  "amount": 150.00,
  "currency": "USD",
  "created_at": "2026-07-22T15:30:00Z"
}
```

### Unified Error Response Schema

All error responses must return a consistent JSON payload structure matching `httpapi.ErrorResponse`:

```json
{
  "code": "INVALID_ARGUMENT",
  "message": "Payment amount must be greater than zero",
  "details": {
    "field": "amount",
    "value": -10.0
  }
}
```

### HTTP Status Code Mapping

| Status Code | Code String | Condition |
| --- | --- | --- |
| `400 Bad Request` | `INVALID_ARGUMENT` | Malformed request body, JSON syntax error, or field validation failure |
| `401 Unauthorized` | `UNAUTHENTICATED` | Missing or invalid bearer token |
| `403 Forbidden` | `PERMISSION_DENIED` | Valid token, but user lacks required role or permission |
| `404 Not Found` | `NOT_FOUND` | Resource or aggregate does not exist |
| `409 Conflict` | `ALREADY_EXISTS` | Unique constraint violation (e.g. duplicate transaction ID) |
| `422 Unprocessable` | `BUSINESS_RULE_VIOLATION` | Business rule failed (e.g. insufficient agent balance) |
| `500 Internal Server Error` | `INTERNAL_ERROR` | Unexpected database error or system failure |

---

## 3. Data Transfer Objects (DTO) Rules

- **JSON Tag Naming**: DTO fields MUST use `snake_case` for JSON serialization tags.
- **Time Format**: ISO 8601 string format (`2026-07-22T15:30:00Z`).
- **Omit Empty**: Use `omitempty` for optional response fields.

```go
type InitiatePaymentRequestDTO struct {
    AgentID     string  `json:"agent_id" validate:"required"`
    Amount      float64 `json:"amount" validate:"required,gt=0"`
    Currency    string  `json:"currency" validate:"required,len=3"`
    CustomerID  string  `json:"customer_id,omitempty"`
    CallbackURL string  `json:"callback_url,omitempty"`
}

type InitiatePaymentResponseDTO struct {
    TransactionID string `json:"transaction_id"`
    PaymentURL    string `json:"payment_url"`
    Status        string `json:"status"`
}
```

---

## 4. Authentication & Claim Context

### Request Headers
- **`Authorization`**: Standard Bearer header for public client requests:
  ```http
  Authorization: Bearer <jwt_token>
  ```
- **`x-gateway-jwt-payload`**: Injected by Envoy Gateway after signature verification. Microservices extract user claims from this header via `gatewayauth` middleware:

```go
// Extracting authenticated user claims inside an Echo route handler
func (h *Handler) GetProfile(c echo.Context) error {
    claims, err := gatewayauth.FromContext(c.Request().Context())
    if err != nil {
        return c.JSON(http.StatusUnauthorized, httpapi.NewError("UNAUTHENTICATED", "Missing identity context"))
    }
    userID := claims.Subject
    // ...
}
```

---

## 5. Swagger / OpenAPI Documentation Standards

REST endpoints must be annotated using declarative `swag` comment tags placed directly above the Echo handler methods under `services/<service>/internal/transport/http/`.

### Required Annotation Template:

```go
// InitiatePayment godoc
// @Summary      Initiate a new payment
// @Description  Creates a payment transaction aggregate and returns checkout details
// @Tags         payment
// @Accept       json
// @Produce      json
// @Param        request body dto.InitiatePaymentRequestDTO true "Payment details"
// @Success      201  {object}  dto.InitiatePaymentResponseDTO
// @Failure      400  {object}  httpapi.ErrorResponse
// @Failure      401  {object}  httpapi.ErrorResponse
// @Failure      500  {object}  httpapi.ErrorResponse
// @Security     OAuth2Password || BearerAuth
// @Router       /api/v1/payment/initiate [post]
func (h *Handler) InitiatePayment(c echo.Context) error {
    // ...
}
```

### Documentation Refresh Command
After adding or modifying handler annotations, update generated OpenAPI files using Taskfile:

```bash
# Regenerate REST Swagger specifications
task gen-swagger

# Regenerate and deploy updated Swagger ConfigMaps to local k3d cluster
task swagger-refresh
```
