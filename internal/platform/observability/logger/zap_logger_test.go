package logger

import (
	"fmt"
	"syscall"
	"testing"

	"github/master_go_cqrs/internal/platform/config"

	"go.uber.org/zap"
)

func TestGetMessageAndFieldsSplitsZapFieldsFromFormatArgs(t *testing.T) {
	cfg := &config.Logger{}
	cfg.LogLevel = "info"
	cfg.Mode = "development"
	cfg.LogEncoding = "console"

	log, ok := NewapiLogger(cfg).(*apiLogger)
	if !ok {
		t.Fatal("expected apiLogger")
	}

	msg, fields := log.getMessageAndFields("Starting gRPC server", []interface{}{zap.String("addr", ":9889")})
	if msg != "Starting gRPC server" {
		t.Fatalf("message = %q, want unchanged message", msg)
	}
	if len(fields) != 1 || fields[0].Key != "addr" {
		t.Fatalf("fields = %#v, want addr field", fields)
	}

	msg, fields = log.getMessageAndFields("failed to bind %s", []interface{}{":9889", zap.Error(syscall.EADDRINUSE)})
	if msg != "failed to bind :9889" {
		t.Fatalf("formatted message = %q", msg)
	}
	if len(fields) != 1 || fields[0].Key != "error" {
		t.Fatalf("fields = %#v, want error field", fields)
	}
}

func TestIgnorableSyncError(t *testing.T) {
	for _, err := range []error{
		syscall.EINVAL,
		syscall.ENOTTY,
		fmt.Errorf("sync /dev/stderr: %w", syscall.ENOTTY),
	} {
		if !isIgnorableSyncError(err) {
			t.Fatalf("expected %v to be ignored", err)
		}
	}

	if isIgnorableSyncError(syscall.EIO) {
		t.Fatal("unexpectedly ignored EIO")
	}
}
