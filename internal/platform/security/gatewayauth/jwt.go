package gatewayauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const maxJWKSResponseBytes = 1 << 20

var (
	ErrMissingBearerToken = errors.New("missing bearer token")
	ErrInvalidBearerToken = errors.New("invalid bearer token")
)

// VerificationConfig controls independent JWT verification at every service
// trust boundary. Forwarded gateway claims are accepted only when Required is
// false, which keeps local development compatible without weakening production.
type VerificationConfig struct {
	Required    bool          `yaml:"required" env:"AUTH_JWT_REQUIRED" env-default:"false"`
	Issuer      string        `yaml:"issuer" env:"AUTH_JWT_ISSUER"`
	Audience    string        `yaml:"audience" env:"AUTH_JWT_AUDIENCE"`
	JWKSURL     string        `yaml:"jwksUrl" env:"AUTH_JWT_JWKS_URL"`
	CacheTTL    time.Duration `yaml:"cacheTtl" env:"AUTH_JWT_CACHE_TTL" env-default:"5m"`
	HTTPTimeout time.Duration `yaml:"httpTimeout" env:"AUTH_JWT_HTTP_TIMEOUT" env-default:"5s"`
	Leeway      time.Duration `yaml:"leeway" env:"AUTH_JWT_LEEWAY" env-default:"30s"`
}

func (c VerificationConfig) validate() error {
	configured := strings.TrimSpace(c.Issuer) != "" || strings.TrimSpace(c.Audience) != "" || strings.TrimSpace(c.JWKSURL) != ""
	if c.Required || configured {
		if strings.TrimSpace(c.Issuer) == "" || strings.TrimSpace(c.Audience) == "" || strings.TrimSpace(c.JWKSURL) == "" {
			return errors.New("JWT issuer, audience, and JWKS URL must be configured together")
		}
	}
	if strings.TrimSpace(c.JWKSURL) != "" {
		parsed, err := url.Parse(c.JWKSURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return errors.New("JWT JWKS URL must be an absolute HTTP(S) URL")
		}
		if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("JWT JWKS URL must not contain credentials, query, or fragment")
		}
	}
	if c.CacheTTL < 0 || c.CacheTTL > 24*time.Hour {
		return errors.New("JWT JWKS cache TTL must be between 0 and 24h")
	}
	if c.HTTPTimeout < 0 || c.HTTPTimeout > 30*time.Second {
		return errors.New("JWT JWKS HTTP timeout must be between 0 and 30s")
	}
	if c.Leeway < 0 || c.Leeway > 5*time.Minute {
		return errors.New("JWT leeway must be between 0 and 5m")
	}
	return nil
}

// PrincipalAuthenticator converts request credentials into verified claims.
// Callers must never treat client-supplied resource ownership fields as a
// substitute for the returned tenant and agent identities.
type PrincipalAuthenticator interface {
	Authenticate(ctx context.Context, authorization, forwardedPayload string) (User, error)
}

type forwardedPayloadAuthenticator struct{}

func (forwardedPayloadAuthenticator) Authenticate(_ context.Context, _, forwardedPayload string) (User, error) {
	return DecodeUserPayload(forwardedPayload)
}

type JWTAuthenticator struct {
	config VerificationConfig
	client *http.Client
	now    func() time.Time

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

func NewJWTAuthenticator(config *VerificationConfig) (PrincipalAuthenticator, error) {
	if config == nil {
		return nil, errors.New("JWT verification config is required")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	cfg := *config
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 5 * time.Second
	}
	if cfg.Leeway == 0 {
		cfg.Leeway = 30 * time.Second
	}
	return &JWTAuthenticator{
		config: cfg,
		client: &http.Client{Timeout: cfg.HTTPTimeout},
		now:    time.Now,
		keys:   make(map[string]*rsa.PublicKey),
	}, nil
}

func (a *JWTAuthenticator) Authenticate(ctx context.Context, authorization, forwardedPayload string) (User, error) {
	token, tokenErr := bearerToken(authorization)
	if tokenErr != nil {
		if a.config.Required {
			return User{}, tokenErr
		}
		// Local/dev compatibility: the public gateway has already stripped the
		// caller's payload header and replaced it after JWT verification.
		return DecodeUserPayload(forwardedPayload)
	}
	if strings.TrimSpace(a.config.JWKSURL) == "" {
		if a.config.Required {
			return User{}, ErrInvalidBearerToken
		}
		return DecodeUserPayload(forwardedPayload)
	}

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithIssuer(strings.TrimSpace(a.config.Issuer)),
		jwt.WithAudience(strings.TrimSpace(a.config.Audience)),
		jwt.WithLeeway(a.config.Leeway),
		jwt.WithTimeFunc(a.now),
	)
	parsed, err := parser.Parse(token, func(parsed *jwt.Token) (any, error) {
		kid, ok := parsed.Header["kid"].(string)
		if !ok || strings.TrimSpace(kid) == "" {
			return nil, errors.New("JWT signing key id is required")
		}
		return a.signingKey(ctx, kid)
	})
	if err != nil || parsed == nil || !parsed.Valid {
		return User{}, fmt.Errorf("%w: verification failed", ErrInvalidBearerToken)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return User{}, fmt.Errorf("%w: claims are invalid", ErrInvalidBearerToken)
	}
	expiresAt, err := claims.GetExpirationTime()
	if err != nil || expiresAt == nil {
		return User{}, fmt.Errorf("%w: expiration is required", ErrInvalidBearerToken)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return User{}, ErrInvalidBearerToken
	}
	user, err := DecodeUserPayload(parts[1])
	if err != nil {
		return User{}, fmt.Errorf("%w: claims are invalid", ErrInvalidBearerToken)
	}
	return user, nil
}

func bearerToken(authorization string) (string, error) {
	parts := strings.Fields(strings.TrimSpace(authorization))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", ErrMissingBearerToken
	}
	return parts[1], nil
}

func (a *JWTAuthenticator) signingKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.now().Before(a.expiresAt) {
		if key := a.keys[kid]; key != nil {
			return key, nil
		}
	}
	keys, err := a.fetchKeys(ctx)
	if err != nil {
		return nil, err
	}
	a.keys = keys
	a.expiresAt = a.now().Add(a.config.CacheTTL)
	key := keys[kid]
	if key == nil {
		return nil, fmt.Errorf("JWT signing key %q was not found", kid)
	}
	return key, nil
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	KeyType string `json:"kty"`
	KeyID   string `json:"kid"`
	Use     string `json:"use"`
	Alg     string `json:"alg"`
	N       string `json:"n"`
	E       string `json:"e"`
}

func (a *JWTAuthenticator) fetchKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.config.JWKSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create JWKS request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	response, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return nil, fmt.Errorf("fetch JWKS: unexpected HTTP status %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxJWKSResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read JWKS: %w", err)
	}
	if len(raw) > maxJWKSResponseBytes {
		return nil, errors.New("JWKS response exceeds size limit")
	}
	var document jwksDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, encoded := range document.Keys {
		key, err := encoded.rsaPublicKey()
		if err != nil {
			continue
		}
		keys[encoded.KeyID] = key
	}
	if len(keys) == 0 {
		return nil, errors.New("JWKS contains no usable RSA signing keys")
	}
	return keys, nil
}

func (k jwk) rsaPublicKey() (*rsa.PublicKey, error) {
	if k.KeyType != "RSA" || strings.TrimSpace(k.KeyID) == "" {
		return nil, errors.New("unsupported JWK")
	}
	if k.Use != "" && k.Use != "sig" {
		return nil, errors.New("JWK is not a signing key")
	}
	if k.Alg != "" && k.Alg != jwt.SigningMethodRS256.Alg() {
		return nil, errors.New("JWK algorithm is not RS256")
	}
	modulusBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil || len(modulusBytes) == 0 {
		return nil, errors.New("JWK modulus is invalid")
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil || len(exponentBytes) == 0 || len(exponentBytes) > 4 {
		return nil, errors.New("JWK exponent is invalid")
	}
	var exponent uint64
	for _, value := range exponentBytes {
		exponent = exponent<<8 | uint64(value)
	}
	if exponent < 3 || exponent > math.MaxInt32 || exponent%2 == 0 {
		return nil, errors.New("JWK exponent is invalid")
	}
	modulus := new(big.Int).SetBytes(modulusBytes)
	if modulus.BitLen() < 2048 {
		return nil, errors.New("JWK RSA modulus must be at least 2048 bits")
	}
	return &rsa.PublicKey{N: modulus, E: int(exponent)}, nil
}
