package postgres

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github/master_go_cqrs/internal/platform/config"
)

func TestPostgresDSNEscapesCredentialsAndCarriesTLSSettings(t *testing.T) {
	dsn, err := postgresDSN(&config.Postgres{
		Host: "db.internal", Port: "5432", User: "payment@service", Password: "secret:/ value",
		DBName: "payment", SSLMode: "verify-full", SSLRootCert: "/certs/ca.pem",
		ApplicationName: "payment", ConnectTimeout: 2500 * time.Millisecond, StatementTimeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := parsed.User.Password()
	if parsed.User.Username() != "payment@service" || password != "secret:/ value" {
		t.Fatalf("credentials were not safely encoded: %s", dsn)
	}
	if parsed.Query().Get("sslmode") != "verify-full" || parsed.Query().Get("sslrootcert") != "/certs/ca.pem" {
		t.Fatalf("TLS query = %v", parsed.Query())
	}
	if parsed.Query().Get("connect_timeout") != "3" || parsed.Query().Get("statement_timeout") != "3000" {
		t.Fatalf("timeout query = %v", parsed.Query())
	}
}

func TestPostgresDSNRejectsUnsafeConfiguration(t *testing.T) {
	if _, err := postgresDSN(&config.Postgres{SSLMode: "trust-everything"}); err == nil {
		t.Fatal("unsupported sslmode error = nil")
	}
	_, err := postgresDSN(&config.Postgres{SSLMode: "verify-full", SSLCert: "/certs/client.pem"})
	if err == nil || !strings.Contains(err.Error(), "certificate and key") {
		t.Fatalf("incomplete mTLS error = %v", err)
	}
}
