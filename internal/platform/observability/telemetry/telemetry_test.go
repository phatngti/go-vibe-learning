package telemetry

import "testing"

func TestNewTraceEndpointConfig(t *testing.T) {
	tests := []struct {
		name            string
		endpoint        string
		wantEndpoint    string
		wantEndpointURL string
		wantInsecure    bool
	}{
		{
			name:         "empty defaults to local otel collector http",
			wantEndpoint: "localhost:4318",
			wantInsecure: true,
		},
		{
			name:         "host port endpoint",
			endpoint:     "localhost:4318",
			wantEndpoint: "localhost:4318",
			wantInsecure: true,
		},
		{
			name:         "http url without path uses default traces path",
			endpoint:     "http://jaeger:4318",
			wantEndpoint: "jaeger:4318",
			wantInsecure: true,
		},
		{
			name:         "https url without path stays secure",
			endpoint:     "https://collector.example.com:4318",
			wantEndpoint: "collector.example.com:4318",
		},
		{
			name:            "trace specific endpoint url keeps path",
			endpoint:        "http://jaeger:4318/v1/traces",
			wantEndpointURL: "http://jaeger:4318/v1/traces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newTraceEndpointConfig(tt.endpoint)
			if got.endpoint != tt.wantEndpoint {
				t.Fatalf("endpoint = %q, want %q", got.endpoint, tt.wantEndpoint)
			}
			if got.endpointURL != tt.wantEndpointURL {
				t.Fatalf("endpointURL = %q, want %q", got.endpointURL, tt.wantEndpointURL)
			}
			if got.insecure != tt.wantInsecure {
				t.Fatalf("insecure = %t, want %t", got.insecure, tt.wantInsecure)
			}
		})
	}
}
