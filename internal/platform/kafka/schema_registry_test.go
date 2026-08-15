package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	platformconfig "github/master_go_cqrs/internal/platform/config"
)

func TestSchemaRegistryClientRegistersCompatibilityAndCaches(t *testing.T) {
	var mu sync.Mutex
	compatibilityRequests := 0
	registrationRequests := 0
	lookupRequests := 0
	versionRequests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, authenticated := r.BasicAuth()
		if !authenticated || username != "service" || password != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/config/payments.events.v2-value":
			compatibilityRequests++
			var request map[string]string
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode compatibility request: %v", err)
			}
			if request["compatibility"] != "BACKWARD_TRANSITIVE" {
				t.Errorf("compatibility = %q", request["compatibility"])
			}
			_, _ = w.Write([]byte(`{"compatibility":"BACKWARD_TRANSITIVE"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/subjects/payments.events.v2-value/versions":
			registrationRequests++
			var request RegistrySchema
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode registration request: %v", err)
			}
			if request.SchemaType != "PROTOBUF" || request.Schema != EventEnvelopeV2Schema {
				t.Errorf("unexpected registration request: %+v", request)
			}
			_, _ = w.Write([]byte(`{"id":41}`))
		case r.Method == http.MethodGet && r.URL.Path == "/schemas/ids/41":
			lookupRequests++
			_ = json.NewEncoder(w).Encode(RegistrySchema{Schema: EventEnvelopeV2Schema, SchemaType: "PROTOBUF"})
		case r.Method == http.MethodGet && r.URL.Path == "/schemas/ids/41/versions":
			versionRequests++
			_, _ = w.Write([]byte(`[{"subject":"payments.events.v2-value","version":3}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewSchemaRegistryClient(platformconfig.KafkaSchemaRegistry{
		URL: server.URL, Username: "service", Password: "secret",
		Compatibility: "BACKWARD_TRANSITIVE", RequestTimeout: time.Second,
		CacheTTL: time.Minute, CacheCapacity: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	schema := RegistrySchema{Schema: EventEnvelopeV2Schema, SchemaType: "PROTOBUF"}
	for range 2 {
		registration, err := client.Register(context.Background(), "payments.events.v2-value", schema)
		if err != nil {
			t.Fatal(err)
		}
		if registration.ID != 41 || registration.Version != 3 {
			t.Fatalf("schema registration = %+v, want id 41 version 3", registration)
		}
	}
	registered, err := client.Lookup(context.Background(), "payments.events.v2-value", 41)
	if err != nil {
		t.Fatal(err)
	}
	if registered.Schema != EventEnvelopeV2Schema {
		t.Fatal("lookup returned a different schema")
	}

	mu.Lock()
	defer mu.Unlock()
	if compatibilityRequests != 1 || registrationRequests != 1 || versionRequests != 1 || lookupRequests != 0 {
		t.Fatalf("requests: compatibility=%d registration=%d version=%d lookup=%d", compatibilityRequests, registrationRequests, versionRequests, lookupRequests)
	}
}

func TestSchemaRegistryClientLookupCachesSuccessfulResponse(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/schemas/ids/7":
			_ = json.NewEncoder(w).Encode(RegistrySchema{Schema: EventEnvelopeV2Schema, SchemaType: "PROTOBUF"})
		case "/schemas/ids/7/versions":
			_, _ = w.Write([]byte(`[{"subject":"payments.events.v2-value","version":1}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewSchemaRegistryClient(platformconfig.KafkaSchemaRegistry{URL: server.URL, CacheCapacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := client.Lookup(context.Background(), "payments.events.v2-value", 7); err != nil {
			t.Fatal(err)
		}
	}
	if requests != 2 {
		t.Fatalf("lookup requests = %d, want 2", requests)
	}
}

func TestSchemaRegistryClientFailsClosed(t *testing.T) {
	t.Run("registration failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error_code":50001,"message":"registry unavailable"}`))
		}))
		defer server.Close()

		client, err := NewSchemaRegistryClient(platformconfig.KafkaSchemaRegistry{URL: server.URL})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Register(context.Background(), "payments.events.v2-value", RegistrySchema{Schema: EventEnvelopeV2Schema})
		if err == nil || !strings.Contains(err.Error(), "registry unavailable") {
			t.Fatalf("Register() error = %v", err)
		}
	})

	t.Run("unknown schema", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error_code":40403,"message":"schema not found"}`))
		}))
		defer server.Close()

		client, err := NewSchemaRegistryClient(platformconfig.KafkaSchemaRegistry{URL: server.URL})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Lookup(context.Background(), "payments.events.v2-value", 999)
		if err == nil || !strings.Contains(err.Error(), "schema not found") {
			t.Fatalf("Lookup() error = %v", err)
		}
	})

	t.Run("schema belongs to another subject", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/versions") {
				_, _ = w.Write([]byte(`[{"subject":"another-topic-value","version":1}]`))
				return
			}
			_ = json.NewEncoder(w).Encode(RegistrySchema{Schema: EventEnvelopeV2Schema, SchemaType: "PROTOBUF"})
		}))
		defer server.Close()

		client, err := NewSchemaRegistryClient(platformconfig.KafkaSchemaRegistry{URL: server.URL})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Lookup(context.Background(), "payments.events.v2-value", 12)
		if err == nil || !strings.Contains(err.Error(), "no registered version") {
			t.Fatalf("Lookup() error = %v", err)
		}
	})
}

func TestSchemaRegistryClientBoundsRequestTime(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		time.Sleep(150 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(RegistrySchema{Schema: EventEnvelopeV2Schema, SchemaType: "PROTOBUF"})
	}))
	defer server.Close()

	client, err := NewSchemaRegistryClient(platformconfig.KafkaSchemaRegistry{
		URL: server.URL, RequestTimeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	_, err = client.Lookup(context.Background(), "payments.events.v2-value", 8)
	if err == nil {
		t.Fatal("Lookup() error = nil, want timeout")
	}
	<-started
	if elapsed := time.Since(before); elapsed > 120*time.Millisecond {
		t.Fatalf("request elapsed = %s, want bounded by client timeout", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "Client.Timeout") {
		t.Fatalf("Lookup() error = %v, want deadline error", err)
	}
}

func TestSchemaRegistryCacheCapacityIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			_, _ = w.Write([]byte(`{"compatibility":"BACKWARD_TRANSITIVE"}`))
		case http.MethodPost:
			_, _ = w.Write([]byte(`{"id":1}`))
		case http.MethodGet:
			_, _ = w.Write([]byte(`[{"subject":"one-value","version":1},{"subject":"two-value","version":1},{"subject":"three-value","version":1}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewSchemaRegistryClient(platformconfig.KafkaSchemaRegistry{URL: server.URL, CacheCapacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, subject := range []string{"one-value", "two-value", "three-value"} {
		if _, err := client.Register(context.Background(), subject, RegistrySchema{Schema: EventEnvelopeV2Schema}); err != nil {
			t.Fatal(err)
		}
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.registrationCache) > 2 || len(client.lookupCache) > 2 || len(client.compatibilityCache) > 2 {
		t.Fatalf("cache sizes: registrations=%d lookups=%d compatibility=%d", len(client.registrationCache), len(client.lookupCache), len(client.compatibilityCache))
	}
}
