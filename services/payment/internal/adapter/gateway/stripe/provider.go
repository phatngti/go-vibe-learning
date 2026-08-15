package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/payment/internal/application/port"
)

const (
	defaultBaseURL          = "https://api.stripe.com"
	defaultHTTPTimeout      = 10 * time.Second
	defaultWebhookTolerance = 5 * time.Minute
	maxProviderResponse     = 1 << 20
)

var ErrInvalidWebhookSignature = errors.New("invalid Stripe webhook signature")

type Config struct {
	SecretKey        string
	WebhookSecrets   []string
	BaseURL          string
	HTTPTimeout      time.Duration
	WebhookTolerance time.Duration
}

type Provider struct {
	secretKey        string
	webhookSecrets   []string
	baseURL          string
	client           *http.Client
	webhookTolerance time.Duration
}

func NewProvider(cfg Config) (*Provider, error) {
	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	return NewProviderWithClient(cfg, &http.Client{Timeout: timeout})
}

func NewProviderWithClient(cfg Config, client *http.Client) (*Provider, error) {
	secretKey := strings.TrimSpace(cfg.SecretKey)
	if secretKey == "" {
		return nil, fmt.Errorf("Stripe secret key is required")
	}
	secrets := make([]string, 0, len(cfg.WebhookSecrets))
	for _, secret := range cfg.WebhookSecrets {
		if secret = strings.TrimSpace(secret); secret != "" {
			secrets = append(secrets, secret)
		}
	}
	if len(secrets) == 0 {
		return nil, fmt.Errorf("at least one Stripe webhook secret is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Stripe base URL")
	}
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	tolerance := cfg.WebhookTolerance
	if tolerance <= 0 {
		tolerance = defaultWebhookTolerance
	}
	return &Provider{
		secretKey: secretKey, webhookSecrets: secrets, baseURL: baseURL,
		client: client, webhookTolerance: tolerance,
	}, nil
}

func (p *Provider) Name() string { return "stripe" }

func (p *Provider) CreateIntent(ctx context.Context, request port.CreateIntentRequest) (*port.PaymentIntent, error) {
	if err := request.Money.Validate(); err != nil {
		return nil, fmt.Errorf("Stripe create intent: %w", err)
	}
	if !request.Money.IsPositive() || strings.TrimSpace(request.TransactionID) == "" || strings.TrimSpace(request.IdempotencyKey) == "" {
		return nil, fmt.Errorf("Stripe create intent: positive money, transaction ID, and idempotency key are required")
	}
	form := url.Values{
		"amount":                             {strconv.FormatInt(request.Money.AmountMinor, 10)},
		"currency":                           {strings.ToLower(request.Money.Currency)},
		"automatic_payment_methods[enabled]": {"true"},
	}
	for key, value := range request.Metadata {
		form.Set("metadata["+key+"]", value)
	}
	// System-owned correlation metadata is written last so callers cannot
	// override the internal transaction identity through provider metadata.
	form.Set("metadata[transaction_id]", request.TransactionID)
	if customer := strings.TrimSpace(request.CustomerReference); customer != "" {
		form.Set("metadata[customer_reference]", customer)
	}
	var response stripeIntent
	if err := p.do(ctx, http.MethodPost, "/v1/payment_intents", form, request.IdempotencyKey, &response); err != nil {
		return nil, err
	}
	return response.toPort()
}

func (p *Provider) RetrieveIntent(ctx context.Context, providerReference string) (*port.PaymentIntent, error) {
	if strings.TrimSpace(providerReference) == "" {
		return nil, fmt.Errorf("Stripe retrieve intent: provider reference is required")
	}
	var response stripeIntent
	if err := p.do(ctx, http.MethodGet, "/v1/payment_intents/"+url.PathEscape(providerReference), nil, "", &response); err != nil {
		return nil, err
	}
	return response.toPort()
}

func (p *Provider) CancelIntent(ctx context.Context, providerReference string) (*port.PaymentIntent, error) {
	if strings.TrimSpace(providerReference) == "" {
		return nil, fmt.Errorf("Stripe cancel intent: provider reference is required")
	}
	var response stripeIntent
	if err := p.do(ctx, http.MethodPost, "/v1/payment_intents/"+url.PathEscape(providerReference)+"/cancel", url.Values{}, "", &response); err != nil {
		return nil, err
	}
	return response.toPort()
}

func (p *Provider) ParseWebhook(payload []byte, signature string, receivedAt time.Time) (*port.ProviderWebhookEvent, error) {
	timestamp, signatures, err := parseSignatureHeader(signature)
	if err != nil {
		return nil, err
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}
	delta := receivedAt.Sub(time.Unix(timestamp, 0))
	if delta < 0 {
		delta = -delta
	}
	if delta > p.webhookTolerance {
		return nil, fmt.Errorf("%w: timestamp outside tolerance", ErrInvalidWebhookSignature)
	}
	signedPayload := []byte(strconv.FormatInt(timestamp, 10) + "." + string(payload))
	verified := false
	for _, secret := range p.webhookSecrets {
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(signedPayload)
		expected := mac.Sum(nil)
		for _, candidate := range signatures {
			decoded, decodeErr := hex.DecodeString(candidate)
			if decodeErr == nil && hmac.Equal(expected, decoded) {
				verified = true
				break
			}
		}
		if verified {
			break
		}
	}
	if !verified {
		return nil, ErrInvalidWebhookSignature
	}

	var event stripeEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("parse Stripe webhook JSON: %w", err)
	}
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Type) == "" {
		return nil, fmt.Errorf("parse Stripe webhook JSON: event ID and type are required")
	}
	var intent stripeIntent
	if len(event.Data.Object) > 0 {
		if err := json.Unmarshal(event.Data.Object, &intent); err != nil {
			return nil, fmt.Errorf("parse Stripe webhook object: %w", err)
		}
	}
	digest := sha256.Sum256(payload)
	occurredAt := time.Unix(event.Created, 0)
	if event.Created == 0 {
		occurredAt = receivedAt
	}
	return &port.ProviderWebhookEvent{
		Provider: "stripe", EventID: event.ID, EventType: event.Type,
		TransactionID: intent.Metadata["transaction_id"], ProviderReference: intent.ID,
		Status: mapStatus(intent.Status), FailureReason: intent.failureReason(),
		PayloadSHA256: hex.EncodeToString(digest[:]), OccurredAt: occurredAt, ReceivedAt: receivedAt,
	}, nil
}

func (p *Provider) do(ctx context.Context, method, path string, form url.Values, idempotencyKey string, target any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build Stripe request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+p.secretKey)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "master-go-cqrs/payment")
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("Stripe request failed: %w", err)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxProviderResponse)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var providerError struct {
			Error struct {
				Type    string `json:"type"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if decodeErr := json.NewDecoder(limited).Decode(&providerError); decodeErr != nil {
			return fmt.Errorf("Stripe returned HTTP %d", response.StatusCode)
		}
		return fmt.Errorf("Stripe returned HTTP %d (%s/%s): %s", response.StatusCode,
			providerError.Error.Type, providerError.Error.Code, providerError.Error.Message)
	}
	if err := json.NewDecoder(limited).Decode(target); err != nil {
		return fmt.Errorf("decode Stripe response: %w", err)
	}
	return nil
}

type stripeIntent struct {
	ID               string            `json:"id"`
	Amount           int64             `json:"amount"`
	Currency         string            `json:"currency"`
	ClientSecret     string            `json:"client_secret"`
	Status           string            `json:"status"`
	Metadata         map[string]string `json:"metadata"`
	LastPaymentError *struct {
		Message string `json:"message"`
	} `json:"last_payment_error"`
}

func (s stripeIntent) toPort() (*port.PaymentIntent, error) {
	if strings.TrimSpace(s.ID) == "" {
		return nil, fmt.Errorf("Stripe returned an intent without an ID")
	}
	value, err := sharedmoney.NewForCurrency(s.Amount, s.Currency)
	if err != nil {
		return nil, fmt.Errorf("Stripe returned invalid money: %w", err)
	}
	return &port.PaymentIntent{
		ProviderReference: s.ID, ClientSecret: s.ClientSecret, Status: mapStatus(s.Status),
		Money: value, TransactionID: s.Metadata["transaction_id"], FailureReason: s.failureReason(),
	}, nil
}

func (s stripeIntent) failureReason() string {
	if s.LastPaymentError == nil {
		return ""
	}
	return s.LastPaymentError.Message
}

type stripeEvent struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Created int64  `json:"created"`
	Data    struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

func mapStatus(status string) port.ProviderPaymentStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "requires_payment_method":
		return port.ProviderStatusRequiresPaymentMethod
	case "requires_confirmation":
		return port.ProviderStatusRequiresConfirmation
	case "requires_action":
		return port.ProviderStatusRequiresAction
	case "processing", "requires_capture":
		return port.ProviderStatusProcessing
	case "succeeded":
		return port.ProviderStatusSucceeded
	case "canceled":
		return port.ProviderStatusCanceled
	case "failed":
		return port.ProviderStatusFailed
	default:
		return port.ProviderStatusUnknown
	}
}

func parseSignatureHeader(header string) (int64, []string, error) {
	var timestamp int64
	var signatures []string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return 0, nil, ErrInvalidWebhookSignature
			}
			timestamp = parsed
		case "v1":
			signatures = append(signatures, value)
		}
	}
	if timestamp <= 0 || len(signatures) == 0 {
		return 0, nil, ErrInvalidWebhookSignature
	}
	return timestamp, signatures, nil
}
