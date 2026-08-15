package kafka

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	platformconfig "github/master_go_cqrs/internal/platform/config"
)

const (
	defaultSchemaRegistryTimeout       = 5 * time.Second
	defaultSchemaRegistryCacheTTL      = 15 * time.Minute
	defaultSchemaRegistryCacheCapacity = 1024
	maxSchemaRegistryResponseBytes     = 2 << 20
)

type SchemaReference struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Version int    `json:"version"`
}

type RegistrySchema struct {
	Schema     string            `json:"schema"`
	SchemaType string            `json:"schemaType,omitempty"`
	References []SchemaReference `json:"references,omitempty"`
}

type SchemaRegistration struct {
	ID      int
	Version int
}

// SchemaRegistry is the minimal contract needed by the Protobuf envelope
// codec. Implementations must return errors for unavailable or unknown
// schemas; callers intentionally do not fall back to another wire format.
type SchemaRegistry interface {
	Register(ctx context.Context, subject string, schema RegistrySchema) (SchemaRegistration, error)
	Lookup(ctx context.Context, subject string, id int) (RegistrySchema, error)
}

type schemaIDCacheEntry struct {
	registration SchemaRegistration
	expiresAt    time.Time
}

type schemaCacheEntry struct {
	schema    RegistrySchema
	expiresAt time.Time
}

type schemaLookupKey struct {
	subject string
	id      int
}

type SchemaRegistryClient struct {
	baseURL       string
	username      string
	password      string
	compatibility string
	cacheTTL      time.Duration
	cacheCapacity int
	httpClient    *http.Client

	mu                 sync.Mutex
	registrationCache  map[string]schemaIDCacheEntry
	lookupCache        map[schemaLookupKey]schemaCacheEntry
	compatibilityCache map[string]time.Time
}

func NewSchemaRegistryClient(cfg platformconfig.KafkaSchemaRegistry) (*SchemaRegistryClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	timeout := cfg.RequestTimeout
	if timeout == 0 {
		timeout = defaultSchemaRegistryTimeout
	}
	cacheTTL := cfg.CacheTTL
	if cacheTTL == 0 {
		cacheTTL = defaultSchemaRegistryCacheTTL
	}
	cacheCapacity := cfg.CacheCapacity
	if cacheCapacity == 0 {
		cacheCapacity = defaultSchemaRegistryCacheCapacity
	}
	compatibility := strings.ToUpper(strings.TrimSpace(cfg.Compatibility))
	if compatibility == "" {
		compatibility = "BACKWARD_TRANSITIVE"
	}

	return &SchemaRegistryClient{
		baseURL:  strings.TrimRight(strings.TrimSpace(cfg.URL), "/"),
		username: cfg.Username, password: cfg.Password,
		compatibility: compatibility, cacheTTL: cacheTTL, cacheCapacity: cacheCapacity,
		httpClient:         &http.Client{Timeout: timeout},
		registrationCache:  make(map[string]schemaIDCacheEntry),
		lookupCache:        make(map[schemaLookupKey]schemaCacheEntry),
		compatibilityCache: make(map[string]time.Time),
	}, nil
}

func (c *SchemaRegistryClient) Register(ctx context.Context, subject string, schema RegistrySchema) (SchemaRegistration, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return SchemaRegistration{}, fmt.Errorf("kafka: schema registry subject is required")
	}
	if strings.TrimSpace(schema.Schema) == "" {
		return SchemaRegistration{}, fmt.Errorf("kafka: schema definition is required")
	}
	if schema.SchemaType == "" {
		schema.SchemaType = "PROTOBUF"
	}
	if strings.ToUpper(schema.SchemaType) != "PROTOBUF" {
		return SchemaRegistration{}, fmt.Errorf("kafka: unsupported registry schema type %q", schema.SchemaType)
	}
	schema.SchemaType = "PROTOBUF"
	cacheKey, err := registrationCacheKey(subject, schema)
	if err != nil {
		return SchemaRegistration{}, err
	}
	if registration, ok := c.cachedRegistration(cacheKey); ok {
		return registration, nil
	}

	if err := c.ensureCompatibility(ctx, subject); err != nil {
		return SchemaRegistration{}, err
	}
	var response struct {
		ID int `json:"id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/subjects/"+url.PathEscape(subject)+"/versions", schema, &response); err != nil {
		return SchemaRegistration{}, fmt.Errorf("kafka: register schema for subject %q: %w", subject, err)
	}
	if response.ID <= 0 {
		return SchemaRegistration{}, fmt.Errorf("kafka: register schema for subject %q returned invalid id %d", subject, response.ID)
	}
	version, err := c.lookupSubjectVersion(ctx, subject, response.ID)
	if err != nil {
		return SchemaRegistration{}, err
	}
	registration := SchemaRegistration{ID: response.ID, Version: version}
	c.storeRegistration(cacheKey, registration)
	c.storeLookup(subject, response.ID, schema)
	return registration, nil
}

func (c *SchemaRegistryClient) lookupSubjectVersion(ctx context.Context, subject string, id int) (int, error) {
	var versions []struct {
		Subject string `json:"subject"`
		Version int    `json:"version"`
	}
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/schemas/ids/%d/versions", id), nil, &versions); err != nil {
		return 0, fmt.Errorf("kafka: resolve version for schema id %d subject %q: %w", id, subject, err)
	}
	version := 0
	for _, candidate := range versions {
		if candidate.Subject == subject && candidate.Version > version {
			version = candidate.Version
		}
	}
	if version <= 0 {
		return 0, fmt.Errorf("kafka: schema id %d has no registered version for subject %q", id, subject)
	}
	return version, nil
}

func (c *SchemaRegistryClient) Lookup(ctx context.Context, subject string, id int) (RegistrySchema, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return RegistrySchema{}, fmt.Errorf("kafka: schema registry subject is required")
	}
	if id <= 0 {
		return RegistrySchema{}, fmt.Errorf("kafka: schema id must be positive")
	}
	if schema, ok := c.cachedLookup(subject, id); ok {
		return schema, nil
	}
	var schema RegistrySchema
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/schemas/ids/%d", id), nil, &schema); err != nil {
		return RegistrySchema{}, fmt.Errorf("kafka: lookup schema id %d: %w", id, err)
	}
	if strings.TrimSpace(schema.Schema) == "" {
		return RegistrySchema{}, fmt.Errorf("kafka: lookup schema id %d returned an empty schema", id)
	}
	if schema.SchemaType == "" {
		return RegistrySchema{}, fmt.Errorf("kafka: schema id %d did not declare PROTOBUF type", id)
	}
	if strings.ToUpper(schema.SchemaType) != "PROTOBUF" {
		return RegistrySchema{}, fmt.Errorf("kafka: schema id %d has unsupported type %q", id, schema.SchemaType)
	}
	schema.SchemaType = "PROTOBUF"
	if _, err := c.lookupSubjectVersion(ctx, subject, id); err != nil {
		return RegistrySchema{}, err
	}
	c.storeLookup(subject, id, schema)
	return schema, nil
}

func (c *SchemaRegistryClient) ensureCompatibility(ctx context.Context, subject string) error {
	now := time.Now()
	c.mu.Lock()
	expiresAt, ok := c.compatibilityCache[subject]
	if ok && now.Before(expiresAt) {
		c.mu.Unlock()
		return nil
	}
	if ok {
		delete(c.compatibilityCache, subject)
	}
	c.mu.Unlock()

	request := struct {
		Compatibility string `json:"compatibility"`
	}{Compatibility: c.compatibility}
	var response struct {
		Compatibility      string `json:"compatibility"`
		CompatibilityLevel string `json:"compatibilityLevel"`
	}
	if err := c.doJSON(ctx, http.MethodPut, "/config/"+url.PathEscape(subject), request, &response); err != nil {
		return fmt.Errorf("kafka: configure compatibility for subject %q: %w", subject, err)
	}
	configured := response.Compatibility
	if configured == "" {
		configured = response.CompatibilityLevel
	}
	if configured != "" && !strings.EqualFold(configured, c.compatibility) {
		return fmt.Errorf("kafka: registry configured compatibility %q for subject %q, want %q", configured, subject, c.compatibility)
	}

	c.mu.Lock()
	c.evictCompatibilityLocked(now)
	c.compatibilityCache[subject] = now.Add(c.cacheTTL)
	c.mu.Unlock()
	return nil
}

func (c *SchemaRegistryClient) doJSON(ctx context.Context, method, path string, requestBody, responseBody any) error {
	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	}
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSchemaRegistryResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxSchemaRegistryResponseBytes {
		return fmt.Errorf("response exceeds %d bytes", maxSchemaRegistryResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var registryErr struct {
			ErrorCode int    `json:"error_code"`
			Message   string `json:"message"`
		}
		_ = json.Unmarshal(data, &registryErr)
		message := strings.TrimSpace(registryErr.Message)
		if len(message) > 512 {
			message = message[:512]
		}
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return fmt.Errorf("registry HTTP %d (code %d): %s", resp.StatusCode, registryErr.ErrorCode, message)
	}
	if responseBody != nil {
		if len(data) == 0 {
			return fmt.Errorf("registry returned an empty response")
		}
		if err := json.Unmarshal(data, responseBody); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func registrationCacheKey(subject string, schema RegistrySchema) (string, error) {
	encoded, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("kafka: encode schema cache key: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return subject + ":" + hex.EncodeToString(digest[:]), nil
}

func (c *SchemaRegistryClient) cachedRegistration(key string) (SchemaRegistration, bool) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.registrationCache[key]
	if !ok {
		return SchemaRegistration{}, false
	}
	if !now.Before(entry.expiresAt) {
		delete(c.registrationCache, key)
		return SchemaRegistration{}, false
	}
	return entry.registration, true
}

func (c *SchemaRegistryClient) storeRegistration(key string, registration SchemaRegistration) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	evictOldestLocked(c.registrationCache, c.cacheCapacity, now, func(entry schemaIDCacheEntry) time.Time { return entry.expiresAt })
	c.registrationCache[key] = schemaIDCacheEntry{registration: registration, expiresAt: now.Add(c.cacheTTL)}
}

func (c *SchemaRegistryClient) cachedLookup(subject string, id int) (RegistrySchema, bool) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	key := schemaLookupKey{subject: subject, id: id}
	entry, ok := c.lookupCache[key]
	if !ok {
		return RegistrySchema{}, false
	}
	if !now.Before(entry.expiresAt) {
		delete(c.lookupCache, key)
		return RegistrySchema{}, false
	}
	return entry.schema, true
}

func (c *SchemaRegistryClient) storeLookup(subject string, id int, schema RegistrySchema) {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	evictOldestLocked(c.lookupCache, c.cacheCapacity, now, func(entry schemaCacheEntry) time.Time { return entry.expiresAt })
	c.lookupCache[schemaLookupKey{subject: subject, id: id}] = schemaCacheEntry{schema: schema, expiresAt: now.Add(c.cacheTTL)}
}

func (c *SchemaRegistryClient) evictCompatibilityLocked(now time.Time) {
	for subject, expiry := range c.compatibilityCache {
		if !now.Before(expiry) {
			delete(c.compatibilityCache, subject)
		}
	}
	if len(c.compatibilityCache) < c.cacheCapacity {
		return
	}
	var oldestSubject string
	var oldestExpiry time.Time
	for subject, expiry := range c.compatibilityCache {
		if oldestSubject == "" || expiry.Before(oldestExpiry) {
			oldestSubject, oldestExpiry = subject, expiry
		}
	}
	delete(c.compatibilityCache, oldestSubject)
}

func evictOldestLocked[K comparable, V any](cache map[K]V, capacity int, now time.Time, expiresAt func(V) time.Time) {
	for key, entry := range cache {
		if !now.Before(expiresAt(entry)) {
			delete(cache, key)
		}
	}
	if len(cache) < capacity {
		return
	}
	var oldestKey K
	var oldestExpiry time.Time
	found := false
	for key, entry := range cache {
		expiry := expiresAt(entry)
		if !found || expiry.Before(oldestExpiry) {
			oldestKey, oldestExpiry, found = key, expiry, true
		}
	}
	if found {
		delete(cache, oldestKey)
	}
}
