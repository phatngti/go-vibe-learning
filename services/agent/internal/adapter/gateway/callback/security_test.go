package callback

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github/master_go_cqrs/services/agent/internal/config"
	"github/master_go_cqrs/services/agent/internal/domain"
)

type staticResolver map[string][]netip.Addr

func (r staticResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	addresses, ok := r[host]
	if !ok {
		return nil, errors.New("not found")
	}
	return addresses, nil
}

func TestEndpointValidatorNormalizesVerifiedHTTPSDomain(t *testing.T) {
	validator := NewEndpointValidatorWithResolver(config.Callback{
		RequireHTTPS: true, RequireVerified: true, VerifiedDomains: []string{"*.merchant.example"},
	}, staticResolver{"pay.merchant.example": {netip.MustParseAddr("8.8.8.8")}})

	endpoint, err := validator.Validate(context.Background(), "https://PAY.merchant.example/callback")
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.URL != "https://pay.merchant.example/callback" || !endpoint.Verified {
		t.Fatalf("endpoint = %+v", endpoint)
	}
}

func TestEndpointValidatorRejectsSSRFAndUnsafeURLShapes(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		resolver staticResolver
	}{
		{name: "plain HTTP", rawURL: "http://merchant.example/callback", resolver: staticResolver{"merchant.example": {netip.MustParseAddr("8.8.8.8")}}},
		{name: "IP literal", rawURL: "https://127.0.0.1/callback"},
		{name: "private DNS", rawURL: "https://merchant.example/callback", resolver: staticResolver{"merchant.example": {netip.MustParseAddr("10.0.0.4")}}},
		{name: "mixed public and private DNS", rawURL: "https://merchant.example/callback", resolver: staticResolver{"merchant.example": {netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("169.254.169.254")}}},
		{name: "userinfo", rawURL: "https://user@merchant.example/callback", resolver: staticResolver{"merchant.example": {netip.MustParseAddr("8.8.8.8")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewEndpointValidatorWithResolver(config.Callback{
				RequireHTTPS: true, VerifiedDomains: []string{"merchant.example"},
			}, tt.resolver)
			if _, err := validator.Validate(context.Background(), tt.rawURL); !errors.Is(err, domain.ErrInvalidCallbackURL) {
				t.Fatalf("error = %v, want ErrInvalidCallbackURL", err)
			}
		})
	}
}

func TestSafeDialerRevalidatesDNSBeforeConnecting(t *testing.T) {
	dial := safeDialContext(
		staticResolver{"merchant.example": {netip.MustParseAddr("127.0.0.1")}},
		&net.Dialer{Timeout: time.Millisecond},
	)
	if _, err := dial(context.Background(), "tcp", "merchant.example:443"); err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("error = %v, want DNS rebinding rejection", err)
	}
}

func TestSignerProducesVersionedBodyBoundSignature(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	signer := &Signer{version: "key-7", secret: []byte(secret), now: func() time.Time {
		return time.Unix(1_700_000_000, 0)
	}}
	input := domain.CallbackSigningInput{
		EventID: "event-1", TenantID: "tenant-1", TransactionID: "tx-1", Body: []byte(`{"status":"COMPLETED"}`),
	}

	signature, err := signer.Sign(input)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(input.Body)
	canonical := strings.Join([]string{"v1", input.EventID, signature.Timestamp, input.TenantID, input.TransactionID, hex.EncodeToString(digest[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	want := "v1=" + hex.EncodeToString(mac.Sum(nil))
	if signature.Signature != want || signature.KeyVersion != "key-7" {
		t.Fatalf("signature = %+v, want %s", signature, want)
	}
}
