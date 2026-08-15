package messagebus

import (
	"fmt"
	"strconv"
	"strings"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
)

// Money is the v2 integration-contract representation. It aliases the shared
// value object so validation and checked arithmetic have one implementation.
type Money = sharedmoney.Money

// MoneyPointer returns an independent pointer suitable for an integration
// payload. Keeping pointers in additive JSON contracts lets consumers
// distinguish a missing v2 value from a valid zero amount.
func MoneyPointer(value Money) *Money {
	copy := value
	return &copy
}

// ResolveMoney prefers a v2 value and falls back to a legacy decimal amount.
// When both forms are present, currency metadata must agree. The legacy amount
// is intentionally not compared because binary float encoding can differ by a
// fraction below the currency minor unit; the integer representation remains
// authoritative.
func ResolveMoney(preferred *Money, legacyAmount float64, legacyCurrency string) (Money, error) {
	if preferred == nil {
		return sharedmoney.FromLegacyFloat64(legacyAmount, legacyCurrency)
	}
	value, err := sharedmoney.New(preferred.AmountMinor, preferred.Currency, preferred.Exponent)
	if err != nil {
		return Money{}, err
	}
	if legacyCurrency != "" && !strings.EqualFold(strings.TrimSpace(legacyCurrency), value.Currency) {
		return Money{}, fmt.Errorf("money v2 currency %s conflicts with legacy currency %s", value.Currency, legacyCurrency)
	}
	return value, nil
}

// LegacyMajorFloat is a compatibility-only conversion used for dual writes to
// v1 fields. Business arithmetic must happen before this conversion.
func LegacyMajorFloat(value Money) (float64, error) {
	rendered, err := value.MajorString()
	if err != nil {
		return 0, err
	}
	legacy, err := strconv.ParseFloat(rendered, 64)
	if err != nil {
		return 0, fmt.Errorf("render legacy amount: %w", err)
	}
	return legacy, nil
}
