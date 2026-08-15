package callback

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github/master_go_cqrs/services/agent/internal/application/port"
	"github/master_go_cqrs/services/agent/internal/config"
	"github/master_go_cqrs/services/agent/internal/domain"

	"github.com/google/uuid"
)

type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type EndpointValidator struct {
	settings config.Callback
	resolver Resolver
}

var _ port.CallbackEndpointValidator = (*EndpointValidator)(nil)

func NewEndpointValidator(cfg *config.Config) port.CallbackEndpointValidator {
	return NewEndpointValidatorWithResolver(cfg.Callback, net.DefaultResolver)
}

func NewEndpointValidatorWithResolver(settings config.Callback, resolver Resolver) *EndpointValidator {
	return &EndpointValidator{settings: settings, resolver: resolver}
}

func (v *EndpointValidator) Validate(ctx context.Context, rawURL string) (domain.ValidatedCallbackEndpoint, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return domain.ValidatedCallbackEndpoint{}, fmt.Errorf("%w: malformed absolute URL", domain.ErrInvalidCallbackURL)
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return domain.ValidatedCallbackEndpoint{}, fmt.Errorf("%w: userinfo and fragments are forbidden", domain.ErrInvalidCallbackURL)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "https" && (v.settings.RequireHTTPS || parsed.Scheme != "http") {
		return domain.ValidatedCallbackEndpoint{}, fmt.Errorf("%w: HTTPS is required", domain.ErrInvalidCallbackURL)
	}

	hostname := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if !validDomainName(hostname) || net.ParseIP(hostname) != nil {
		return domain.ValidatedCallbackEndpoint{}, fmt.Errorf("%w: a DNS hostname is required", domain.ErrInvalidCallbackURL)
	}
	lookupTimeout := v.settings.DialTimeout
	if lookupTimeout <= 0 {
		lookupTimeout = 3 * time.Second
	}
	lookupCtx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	addresses, err := v.resolver.LookupNetIP(lookupCtx, "ip", hostname)
	if err != nil || len(addresses) == 0 {
		return domain.ValidatedCallbackEndpoint{}, fmt.Errorf("%w: resolve %s: %v", domain.ErrInvalidCallbackURL, hostname, err)
	}
	for _, address := range addresses {
		if !isPublicAddress(address) {
			return domain.ValidatedCallbackEndpoint{}, fmt.Errorf("%w: %s resolves to a non-public address", domain.ErrInvalidCallbackURL, hostname)
		}
	}

	portNumber := parsed.Port()
	parsed.Host = hostname
	if portNumber != "" {
		parsed.Host = net.JoinHostPort(hostname, portNumber)
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	verified := matchesVerifiedDomain(hostname, v.settings.VerifiedDomains)
	if v.settings.RequireVerified && !verified {
		return domain.ValidatedCallbackEndpoint{}, fmt.Errorf("%w: %s", domain.ErrCallbackNotVerified, hostname)
	}
	return domain.ValidatedCallbackEndpoint{URL: parsed.String(), Hostname: hostname, Verified: verified}, nil
}

type Dispatcher struct {
	client *http.Client
}

var _ port.CallbackDispatcher = (*Dispatcher)(nil)

func NewDispatcher(cfg *config.Config) port.CallbackDispatcher {
	return &Dispatcher{client: newSafeHTTPClient(cfg.Callback)}
}

func newSafeHTTPClient(settings config.Callback) *http.Client {
	resolver := net.DefaultResolver
	dialer := &net.Dialer{Timeout: settings.DialTimeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   settings.TLSHandshakeTimeout,
		ResponseHeaderTimeout: settings.ResponseHeaderTimeout,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	transport.DialContext = safeDialContext(resolver, dialer)
	return &http.Client{
		Transport: transport,
		Timeout:   settings.RequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (d *Dispatcher) Dispatch(ctx context.Context, request port.CallbackDispatchRequest) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, request.URL, bytes.NewReader(request.Body))
	if err != nil {
		return 0, err
	}
	for name, value := range request.Headers {
		req.Header.Set(name, value)
	}
	response, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	return response.StatusCode, nil
}

type ChallengeGenerator struct{}

var _ port.CallbackChallengeGenerator = (*ChallengeGenerator)(nil)

func NewChallengeGenerator() port.CallbackChallengeGenerator { return &ChallengeGenerator{} }

func (g *ChallengeGenerator) Generate() (domain.CallbackChallengeSecret, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return domain.CallbackChallengeSecret{}, fmt.Errorf("generate callback ownership challenge: %w", err)
	}
	token := "cqrs_cb_" + base64.RawURLEncoding.EncodeToString(random)
	return domain.CallbackChallengeSecret{ID: uuid.NewString(), Token: token, Digest: g.Digest(token)}, nil
}

func (*ChallengeGenerator) Digest(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

// OwnershipProver fetches a token that the tenant has pre-published at a fixed
// well-known URL. The token is deliberately not sent in the request, otherwise
// an unaffiliated echo endpoint could appear to prove ownership.
type OwnershipProver struct{ client *http.Client }

var _ port.CallbackOwnershipProver = (*OwnershipProver)(nil)

func NewOwnershipProver(cfg *config.Config) port.CallbackOwnershipProver {
	return &OwnershipProver{client: newSafeHTTPClient(cfg.Callback)}
}

func (p *OwnershipProver) Verify(ctx context.Context, proofURL, expectedToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, proofURL, nil)
	if err != nil {
		return fmt.Errorf("%w: construct proof request", domain.ErrCallbackChallengeInvalid)
	}
	req.Header.Set("Accept", "text/plain")
	response, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: retrieve proof: %v", domain.ErrCallbackChallengeInvalid, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: proof endpoint returned HTTP %d", domain.ErrCallbackChallengeInvalid, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(body) > 4096 {
		return fmt.Errorf("%w: proof response is unreadable or too large", domain.ErrCallbackChallengeInvalid)
	}
	actual := strings.TrimSpace(string(body))
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expectedToken)) != 1 {
		return domain.ErrCallbackChallengeInvalid
	}
	return nil
}

func safeDialContext(resolver Resolver, dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, portNumber, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("callback dial address: %w", err)
		}
		if net.ParseIP(host) != nil {
			return nil, fmt.Errorf("callback dial requires a DNS hostname")
		}
		addresses, err := resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("callback resolve %s: %w", host, err)
		}
		if len(addresses) == 0 {
			return nil, fmt.Errorf("callback resolve %s: no addresses", host)
		}
		for _, candidate := range addresses {
			if !isPublicAddress(candidate) {
				return nil, fmt.Errorf("callback destination %s resolved to a non-public address", host)
			}
		}
		var lastErr error
		for _, candidate := range addresses {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.String(), portNumber))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr == nil {
			return nil, fmt.Errorf("callback dial %s: no addresses attempted", host)
		}
		return nil, fmt.Errorf("callback dial %s: %w", host, lastErr)
	}
}

type Signer struct {
	version string
	secret  []byte
	now     func() time.Time
}

var _ port.CallbackSigner = (*Signer)(nil)

func NewSigner(cfg *config.Config) port.CallbackSigner {
	return &Signer{
		version: strings.TrimSpace(cfg.Callback.SigningKeyVersion),
		secret:  []byte(cfg.Callback.SigningSecret),
		now:     time.Now,
	}
}

func (s *Signer) Sign(input domain.CallbackSigningInput) (domain.CallbackSignature, error) {
	if s.version == "" || len(s.secret) < 32 {
		return domain.CallbackSignature{}, domain.ErrCallbackSigningUnavailable
	}
	timestamp := strconv.FormatInt(s.now().UTC().Unix(), 10)
	digest := sha256.Sum256(input.Body)
	digestHex := hex.EncodeToString(digest[:])
	canonical := strings.Join([]string{
		"v1", input.EventID, timestamp, input.TenantID, input.TransactionID, digestHex,
	}, "\n")
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(canonical))
	return domain.CallbackSignature{
		Timestamp: timestamp, BodySHA256: digestHex, KeyVersion: s.version,
		Signature: "v1=" + hex.EncodeToString(mac.Sum(nil)),
	}, nil
}

func matchesVerifiedDomain(hostname string, allowed []string) bool {
	for _, candidate := range allowed {
		candidate = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(candidate), "."))
		if candidate == hostname {
			return true
		}
		if strings.HasPrefix(candidate, "*.") {
			suffix := strings.TrimPrefix(candidate, "*")
			if strings.HasSuffix(hostname, suffix) && hostname != strings.TrimPrefix(suffix, ".") {
				return true
			}
		}
	}
	return false
}

func validDomainName(hostname string) bool {
	if hostname == "" || len(hostname) > 253 || strings.Contains(hostname, "..") {
		return false
	}
	labels := strings.Split(hostname, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}

func isPublicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range deniedAddressPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

var deniedAddressPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}
