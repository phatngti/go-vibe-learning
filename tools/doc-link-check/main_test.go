package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrokenLinks(t *testing.T) {
	directory := t.TempDir()
	existing := filepath.Join(directory, "existing.md")
	if err := os.WriteFile(existing, []byte("# Existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(directory, "source.md")
	contents := "[ok](existing.md#heading) [web](https://example.com) [bad](missing.md)\n"
	if err := os.WriteFile(source, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	failures, err := brokenLinks(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 {
		t.Fatalf("failures = %v, want one unresolved link", failures)
	}
}
