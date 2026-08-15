package money

import (
	"errors"
	"math"
	"testing"
)

func TestParseMajorAndRender(t *testing.T) {
	tests := []struct {
		input, currency, rendered string
		minor                     int64
	}{
		{input: "100.50", currency: "usd", minor: 10_050, rendered: "100.50"},
		{input: "100", currency: "VND", minor: 100, rendered: "100"},
		{input: "-0.01", currency: "EUR", minor: -1, rendered: "-0.01"},
	}
	for _, test := range tests {
		got, err := ParseMajor(test.input, test.currency)
		if err != nil {
			t.Fatalf("ParseMajor(%q, %q): %v", test.input, test.currency, err)
		}
		if got.AmountMinor != test.minor {
			t.Fatalf("ParseMajor(%q) minor = %d, want %d", test.input, got.AmountMinor, test.minor)
		}
		rendered, err := got.MajorString()
		if err != nil {
			t.Fatal(err)
		}
		if rendered != test.rendered {
			t.Fatalf("MajorString() = %q, want %q", rendered, test.rendered)
		}
	}
}

func TestRejectsUnsupportedPrecisionAndCurrency(t *testing.T) {
	if _, err := ParseMajor("1.001", "USD"); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("expected invalid precision, got %v", err)
	}
	if _, err := NewForCurrency(1, "BTC"); !errors.Is(err, ErrUnsupportedCurrency) {
		t.Fatalf("expected unsupported currency, got %v", err)
	}
	if _, err := New(1, "USD", 0); !errors.Is(err, ErrExponentMismatch) {
		t.Fatalf("expected exponent mismatch, got %v", err)
	}
}

func TestCheckedArithmeticAndBasisPoints(t *testing.T) {
	base, _ := NewForCurrency(10_001, "USD")
	commission, err := base.MultiplyBasisPoints(200)
	if err != nil {
		t.Fatal(err)
	}
	if commission.AmountMinor != 200 {
		t.Fatalf("commission = %d, want 200", commission.AmountMinor)
	}

	max, _ := NewForCurrency(math.MaxInt64, "USD")
	one, _ := NewForCurrency(1, "USD")
	if _, err := max.Add(one); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected overflow, got %v", err)
	}
	if _, err := base.Add(Money{AmountMinor: 1, Currency: "VND", Exponent: 0}); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected currency mismatch, got %v", err)
	}
}

func TestLegacyFloatConversion(t *testing.T) {
	got, err := FromLegacyFloat64(100.50, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if got.AmountMinor != 10_050 {
		t.Fatalf("minor = %d, want 10050", got.AmountMinor)
	}
	if _, err := FromLegacyFloat64(1.001, "USD"); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("expected precision error, got %v", err)
	}
	if _, err := FromLegacyFloat64(math.Inf(1), "USD"); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("expected invalid amount, got %v", err)
	}
}
