package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/payment/internal/application/port"
)

func TestProviderPaymentIntentLifecycle(t *testing.T) {
	t.Helper()
	var calls []string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if got := r.Header.Get("Authorization"); got != "Bearer sk_test_123" {
			t.Errorf("Authorization = %q", got)
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/payment_intents":
			if got := r.Header.Get("Idempotency-Key"); got != "payment:tx-1:intent:v1" {
				t.Errorf("Idempotency-Key = %q", got)
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm() error = %v", err)
			}
			if got := r.Form.Get("amount"); got != "1050" {
				t.Errorf("amount = %q", got)
			}
			if got := r.Form.Get("currency"); got != "usd" {
				t.Errorf("currency = %q", got)
			}
			if got := r.Form.Get("metadata[transaction_id]"); got != "tx-1" {
				t.Errorf("transaction metadata = %q", got)
			}
			return intentResponse("requires_action"), nil
		case "GET /v1/payment_intents/pi_123":
			return intentResponse("processing"), nil
		case "POST /v1/payment_intents/pi_123/cancel":
			return intentResponse("canceled"), nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"not found"}}`)), Header: make(http.Header)}, nil
		}
	})}

	provider, err := NewProviderWithClient(Config{
		SecretKey: "sk_test_123", WebhookSecrets: []string{"whsec_test"}, BaseURL: "https://stripe.test",
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	moneyValue, err := sharedmoney.New(1050, "USD", 2)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := provider.CreateIntent(context.Background(), port.CreateIntentRequest{
		TransactionID: "tx-1", IdempotencyKey: "payment:tx-1:intent:v1", Money: moneyValue,
		CustomerReference: "customer-1", Metadata: map[string]string{"order": "order-1", "transaction_id": "untrusted"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if intent.ProviderReference != "pi_123" || intent.ClientSecret != "pi_123_secret" || intent.Status != port.ProviderStatusRequiresAction {
		t.Fatalf("created intent = %+v", intent)
	}

	intent, err = provider.RetrieveIntent(context.Background(), "pi_123")
	if err != nil || intent.Status != port.ProviderStatusProcessing {
		t.Fatalf("retrieved intent = %+v, error = %v", intent, err)
	}
	intent, err = provider.CancelIntent(context.Background(), "pi_123")
	if err != nil || intent.Status != port.ProviderStatusCanceled {
		t.Fatalf("canceled intent = %+v, error = %v", intent, err)
	}
	if got, want := strings.Join(calls, ","), "POST /v1/payment_intents,GET /v1/payment_intents/pi_123,POST /v1/payment_intents/pi_123/cancel"; got != want {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestProviderParseWebhookVerifiesSignatureAndSupportsRotatedSecret(t *testing.T) {
	receivedAt := time.Unix(1_800_000_000, 0)
	payload := []byte(fmt.Sprintf(`{"id":"evt_123","type":"payment_intent.succeeded","created":%d,"data":{"object":{"id":"pi_123","amount":1050,"currency":"usd","status":"succeeded","metadata":{"transaction_id":"tx-1"}}}}`, receivedAt.Unix()))
	provider, err := NewProviderWithClient(Config{
		SecretKey: "sk_test_123", WebhookSecrets: []string{"whsec_new", "whsec_old"},
		WebhookTolerance: 5 * time.Minute,
	}, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}

	event, err := provider.ParseWebhook(payload, stripeSignature(receivedAt.Unix(), payload, "whsec_old"), receivedAt)
	if err != nil {
		t.Fatal(err)
	}
	if event.EventID != "evt_123" || event.TransactionID != "tx-1" || event.ProviderReference != "pi_123" || event.Status != port.ProviderStatusSucceeded {
		t.Fatalf("event = %+v", event)
	}
	if len(event.PayloadSHA256) != sha256.Size*2 {
		t.Fatalf("payload digest length = %d", len(event.PayloadSHA256))
	}

	if _, err := provider.ParseWebhook(payload, stripeSignature(receivedAt.Unix(), payload, "wrong-secret"), receivedAt); !errorsIsInvalidSignature(err) {
		t.Fatalf("invalid signature error = %v", err)
	}
	stale := receivedAt.Add(-10 * time.Minute)
	if _, err := provider.ParseWebhook(payload, stripeSignature(stale.Unix(), payload, "whsec_new"), receivedAt); !errorsIsInvalidSignature(err) {
		t.Fatalf("stale signature error = %v", err)
	}
}

func intentResponse(status string) *http.Response {
	body := fmt.Sprintf(`{"id":"pi_123","amount":1050,"currency":"usd","client_secret":"pi_123_secret","status":%q,"metadata":{"transaction_id":"tx-1"}}`, status)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func stripeSignature(timestamp int64, payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d.%s", timestamp, payload)
	return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

func errorsIsInvalidSignature(err error) bool {
	return err != nil && (err == ErrInvalidWebhookSignature || strings.Contains(err.Error(), ErrInvalidWebhookSignature.Error()))
}
