package messagebus

import (
	"errors"
	"math"
	"testing"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
)

func TestResolveMoneyPrefersMinorUnits(t *testing.T) {
	value, err := ResolveMoney(&Money{AmountMinor: 12_345, Currency: "USD", Exponent: 2}, 999.99, "usd")
	if err != nil {
		t.Fatal(err)
	}
	if value.AmountMinor != 12_345 {
		t.Fatalf("minor amount = %d, want 12345", value.AmountMinor)
	}
}

func TestResolveMoneyRejectsInvalidCompatibilityValues(t *testing.T) {
	if _, err := ResolveMoney(nil, 1.001, "USD"); !errors.Is(err, sharedmoney.ErrInvalidAmount) {
		t.Fatalf("precision error = %v", err)
	}
	if _, err := ResolveMoney(&Money{AmountMinor: 1, Currency: "USD", Exponent: 0}, 0.01, "USD"); !errors.Is(err, sharedmoney.ErrExponentMismatch) {
		t.Fatalf("exponent error = %v", err)
	}
	if _, err := ResolveMoney(nil, math.MaxFloat64, "USD"); !errors.Is(err, sharedmoney.ErrInvalidAmount) {
		t.Fatalf("overflow error = %v", err)
	}
}

func TestLegacyMajorFloatIsCompatibilityOnlyAndExactAtMinorScale(t *testing.T) {
	value, _ := sharedmoney.NewForCurrency(10_050, "USD")
	legacy, err := LegacyMajorFloat(value)
	if err != nil {
		t.Fatal(err)
	}
	converted, err := sharedmoney.FromLegacyFloat64(legacy, "USD")
	if err != nil || converted != value {
		t.Fatalf("round trip = %+v, %v; want %+v", converted, err, value)
	}
}
