package gatewayauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTAuthenticatorVerifiesSignatureIssuerAudienceAndExpiry(t *testing.T) {
	privateKey := mustRSAKey(t)
	var requests atomic.Int32

	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	authenticator := mustAuthenticator(t, VerificationConfig{
		Required: true, Issuer: "https://identity.example/realms/backend",
		Audience: "master-go-cqrs", JWKSURL: "https://identity.example/jwks", CacheTTL: time.Minute,
	})
	authenticator.now = func() time.Time { return now }
	authenticator.client = jwksHTTPClient(t, func() (string, *rsa.PublicKey) {
		requests.Add(1)
		return "key-1", &privateKey.PublicKey
	})

	token := signedToken(t, privateKey, "key-1", jwt.MapClaims{
		"sub": "user-1", "tenant_id": "tenant-1", "agent_id": "agent-1",
		"iss": authenticator.config.Issuer, "aud": []string{"account", authenticator.config.Audience},
		"exp": now.Add(time.Minute).Unix(), "iat": now.Add(-time.Minute).Unix(),
	})
	user, err := authenticator.Authenticate(context.Background(), "Bearer "+token, "forged-payload")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if user.ID != "user-1" || user.TenantID != "tenant-1" || user.AgentID != "agent-1" {
		t.Fatalf("verified user = %+v", user)
	}

	// Cached keys avoid a remote lookup on every request.
	if _, err := authenticator.Authenticate(context.Background(), "Bearer "+token, ""); err != nil {
		t.Fatalf("cached Authenticate() error = %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("JWKS requests = %d, want 1", requests.Load())
	}
}

func TestJWTAuthenticatorRejectsUntrustedClaimsEvenWhenForwardedPayloadIsValid(t *testing.T) {
	trustedKey := mustRSAKey(t)
	attackerKey := mustRSAKey(t)

	now := time.Now().UTC()
	authenticator := mustAuthenticator(t, VerificationConfig{
		Required: true, Issuer: "https://identity.example/realms/backend",
		Audience: "master-go-cqrs", JWKSURL: "https://identity.example/jwks",
	})
	authenticator.client = jwksHTTPClient(t, func() (string, *rsa.PublicKey) { return "key-1", &trustedKey.PublicKey })
	token := signedToken(t, attackerKey, "key-1", jwt.MapClaims{
		"sub": "attacker", "tenant_id": "victim", "iss": authenticator.config.Issuer,
		"aud": authenticator.config.Audience, "exp": now.Add(time.Minute).Unix(),
	})
	forwarded, err := EncodeUserPayloadForTest(map[string]any{"sub": "user-1", "tenant_id": "tenant-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authenticator.Authenticate(context.Background(), "Bearer "+token, forwarded); err == nil {
		t.Fatal("Authenticate() accepted a token signed by an untrusted key")
	}
}

func TestJWTAuthenticatorRejectsExpiredAndWrongAudienceTokens(t *testing.T) {
	privateKey := mustRSAKey(t)

	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	authenticator := mustAuthenticator(t, VerificationConfig{
		Required: true, Issuer: "https://identity.example/realms/backend",
		Audience: "master-go-cqrs", JWKSURL: "https://identity.example/jwks", Leeway: time.Second,
	})
	authenticator.now = func() time.Time { return now }
	authenticator.client = jwksHTTPClient(t, func() (string, *rsa.PublicKey) { return "key-1", &privateKey.PublicKey })

	for name, claims := range map[string]jwt.MapClaims{
		"expired": {
			"sub": "user-1", "iss": authenticator.config.Issuer, "aud": authenticator.config.Audience,
			"exp": now.Add(-time.Minute).Unix(),
		},
		"wrong audience": {
			"sub": "user-1", "iss": authenticator.config.Issuer, "aud": "another-service",
			"exp": now.Add(time.Minute).Unix(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			token := signedToken(t, privateKey, "key-1", claims)
			if _, err := authenticator.Authenticate(context.Background(), "Bearer "+token, ""); err == nil {
				t.Fatal("Authenticate() accepted invalid registered claims")
			}
		})
	}
}

func TestJWTAuthenticatorRefreshesOnUnknownKeyID(t *testing.T) {
	firstKey := mustRSAKey(t)
	secondKey := mustRSAKey(t)
	var current atomic.Int32

	now := time.Now().UTC()
	authenticator := mustAuthenticator(t, VerificationConfig{
		Required: true, Issuer: "https://identity.example/realms/backend",
		Audience: "master-go-cqrs", JWKSURL: "https://identity.example/jwks", CacheTTL: time.Hour,
	})
	authenticator.client = jwksHTTPClient(t, func() (string, *rsa.PublicKey) {
		if current.Load() == 0 {
			return "key-1", &firstKey.PublicKey
		}
		return "key-2", &secondKey.PublicKey
	})
	claims := func() jwt.MapClaims {
		return jwt.MapClaims{"sub": "user-1", "iss": authenticator.config.Issuer, "aud": authenticator.config.Audience, "exp": now.Add(time.Minute).Unix()}
	}
	if _, err := authenticator.Authenticate(context.Background(), "Bearer "+signedToken(t, firstKey, "key-1", claims()), ""); err != nil {
		t.Fatalf("first Authenticate() error = %v", err)
	}
	current.Store(1)
	if _, err := authenticator.Authenticate(context.Background(), "Bearer "+signedToken(t, secondKey, "key-2", claims()), ""); err != nil {
		t.Fatalf("rotated Authenticate() error = %v", err)
	}
}

func TestJWTAuthenticatorLocalCompatibilityIsExplicit(t *testing.T) {
	authenticator := mustAuthenticator(t, VerificationConfig{})
	payload, err := EncodeUserPayloadForTest(map[string]any{"sub": "local-user"})
	if err != nil {
		t.Fatal(err)
	}
	user, err := authenticator.Authenticate(context.Background(), "", payload)
	if err != nil || user.ID != "local-user" {
		t.Fatalf("Authenticate() user = %+v, error = %v", user, err)
	}

	required := VerificationConfig{Required: true}
	if _, err := NewJWTAuthenticator(&required); err == nil {
		t.Fatal("NewJWTAuthenticator() accepted incomplete production config")
	}
}

func mustAuthenticator(t *testing.T, config VerificationConfig) *JWTAuthenticator {
	t.Helper()
	value, err := NewJWTAuthenticator(&config)
	if err != nil {
		t.Fatal(err)
	}
	authenticator, ok := value.(*JWTAuthenticator)
	if !ok {
		t.Fatalf("authenticator type = %T", value)
	}
	return authenticator
}

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func signedToken(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func jwksHTTPClient(t *testing.T, current func() (string, *rsa.PublicKey)) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		kid, key := current()
		exponent := big.NewInt(int64(key.E)).Bytes()
		document := jwksDocument{Keys: []jwk{{
			KeyType: "RSA", KeyID: kid, Use: "sig", Alg: "RS256",
			N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			E: base64.RawURLEncoding.EncodeToString(exponent),
		}}}
		raw, err := json.Marshal(document)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(raw)),
			Request:    request,
		}, nil
	})}
}
