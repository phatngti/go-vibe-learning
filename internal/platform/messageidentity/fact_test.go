package messageidentity

import "testing"

func TestFactIdentityIsUniquePerAggregateVersion(t *testing.T) {
	first := Fact("tenant:user:1", "user-synced", 1)
	second := Fact("tenant:user:1", "user-synced", 2)
	if first == second {
		t.Fatalf("recurring facts share identity %q", first)
	}
	if version, ok := FactVersion(second); !ok || version != 2 {
		t.Fatalf("FactVersion(%q) = %d, %t", second, version, ok)
	}
	if _, ok := FactVersion("tx-1:affiliate"); ok {
		t.Fatal("command identity was classified as a fact")
	}
}
