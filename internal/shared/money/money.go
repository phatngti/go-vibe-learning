// Package money defines the shared, infrastructure-free representation used
// for monetary values at domain and integration boundaries.
package money

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

var (
	ErrUnsupportedCurrency = errors.New("unsupported ISO 4217 currency")
	ErrExponentMismatch    = errors.New("currency exponent mismatch")
	ErrInvalidAmount       = errors.New("invalid monetary amount")
	ErrOverflow            = errors.New("monetary arithmetic overflow")
	ErrCurrencyMismatch    = errors.New("money currency mismatch")
)

// currencyExponents contains the currencies supported by this service suite.
// Expanding the list is an explicit contract change and must be accompanied by
// provider, persistence, and API compatibility tests.
var currencyExponents = map[string]int32{
	"AUD": 2,
	"CAD": 2,
	"CHF": 2,
	"CNY": 2,
	"EUR": 2,
	"GBP": 2,
	"HKD": 2,
	"IDR": 2,
	"INR": 2,
	"JPY": 0,
	"KRW": 0,
	"MYR": 2,
	"NZD": 2,
	"PHP": 2,
	"SGD": 2,
	"THB": 2,
	"USD": 2,
	"VND": 0,
}

// Money stores an amount as integer minor units. Exponent is part of the wire
// representation so consumers can validate rather than assume scale.
type Money struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Exponent    int32  `json:"exponent"`
}

// New validates and normalizes a monetary value.
func New(amountMinor int64, currency string, exponent int32) (Money, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	want, ok := currencyExponents[currency]
	if !ok {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, currency)
	}
	if exponent != want {
		return Money{}, fmt.Errorf("%w: %s requires %d, got %d", ErrExponentMismatch, currency, want, exponent)
	}
	return Money{AmountMinor: amountMinor, Currency: currency, Exponent: exponent}, nil
}

// NewForCurrency uses the canonical exponent for a supported currency.
func NewForCurrency(amountMinor int64, currency string) (Money, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	exponent, ok := currencyExponents[currency]
	if !ok {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, currency)
	}
	return Money{AmountMinor: amountMinor, Currency: currency, Exponent: exponent}, nil
}

// ExponentForCurrency returns the canonical exponent for a supported currency.
func ExponentForCurrency(currency string) (int32, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	exponent, ok := currencyExponents[currency]
	if !ok {
		return 0, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, currency)
	}
	return exponent, nil
}

// Validate checks a value received from an untrusted boundary.
func (m Money) Validate() error {
	_, err := New(m.AmountMinor, m.Currency, m.Exponent)
	return err
}

// IsPositive reports whether the amount is strictly greater than zero.
func (m Money) IsPositive() bool { return m.AmountMinor > 0 }

// Add performs checked addition and requires identical currency metadata.
func (m Money) Add(other Money) (Money, error) {
	if err := sameCurrency(m, other); err != nil {
		return Money{}, err
	}
	if (other.AmountMinor > 0 && m.AmountMinor > math.MaxInt64-other.AmountMinor) ||
		(other.AmountMinor < 0 && m.AmountMinor < math.MinInt64-other.AmountMinor) {
		return Money{}, ErrOverflow
	}
	m.AmountMinor += other.AmountMinor
	return m, nil
}

// Subtract performs checked subtraction and requires identical currency metadata.
func (m Money) Subtract(other Money) (Money, error) {
	if err := sameCurrency(m, other); err != nil {
		return Money{}, err
	}
	if other.AmountMinor == math.MinInt64 {
		return Money{}, ErrOverflow
	}
	other.AmountMinor = -other.AmountMinor
	return m.Add(other)
}

// MultiplyBasisPoints multiplies the amount by basis points and rounds halves
// away from zero. big.Int avoids intermediate int64 overflow.
func (m Money) MultiplyBasisPoints(basisPoints int64) (Money, error) {
	if err := m.Validate(); err != nil {
		return Money{}, err
	}
	numerator := new(big.Int).Mul(big.NewInt(m.AmountMinor), big.NewInt(basisPoints))
	denominator := big.NewInt(10_000)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	absRemainder := new(big.Int).Abs(new(big.Int).Set(remainder))
	if new(big.Int).Mul(absRemainder, big.NewInt(2)).Cmp(denominator) >= 0 {
		if numerator.Sign() >= 0 {
			quotient.Add(quotient, big.NewInt(1))
		} else {
			quotient.Sub(quotient, big.NewInt(1))
		}
	}
	if !quotient.IsInt64() {
		return Money{}, ErrOverflow
	}
	m.AmountMinor = quotient.Int64()
	return m, nil
}

// ParseMajor parses a non-scientific decimal string without passing through a
// binary floating-point representation.
func ParseMajor(value, currency string) (Money, error) {
	exponent, err := ExponentForCurrency(currency)
	if err != nil {
		return Money{}, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return Money{}, ErrInvalidAmount
	}

	sign := int64(1)
	switch value[0] {
	case '-':
		sign = -1
		value = value[1:]
	case '+':
		value = value[1:]
	}
	if value == "" || strings.ContainsAny(value, "eE") {
		return Money{}, ErrInvalidAmount
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return Money{}, ErrInvalidAmount
	}
	whole, fraction := parts[0], ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if !decimalDigits(whole) || (fraction != "" && !decimalDigits(fraction)) {
		return Money{}, ErrInvalidAmount
	}
	if len(fraction) > int(exponent) {
		return Money{}, fmt.Errorf("%w: %s permits at most %d decimal places", ErrInvalidAmount, strings.ToUpper(currency), exponent)
	}
	fraction += strings.Repeat("0", int(exponent)-len(fraction))
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		digits = "0"
	}
	parsed, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return Money{}, ErrInvalidAmount
	}
	if sign < 0 {
		parsed.Neg(parsed)
	}
	if !parsed.IsInt64() {
		return Money{}, ErrOverflow
	}
	return New(parsed.Int64(), currency, exponent)
}

// FromLegacyFloat64 is restricted to compatibility adapters. It rejects NaN,
// infinity, and values carrying precision below the currency minor unit.
func FromLegacyFloat64(value float64, currency string) (Money, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Money{}, ErrInvalidAmount
	}
	exponent, err := ExponentForCurrency(currency)
	if err != nil {
		return Money{}, err
	}
	scale := math.Pow10(int(exponent))
	scaled := value * scale
	rounded := math.Round(scaled)
	tolerance := math.Max(1e-7, math.Abs(scaled)*1e-12)
	if math.Abs(scaled-rounded) > tolerance || rounded > math.MaxInt64 || rounded < math.MinInt64 {
		return Money{}, fmt.Errorf("%w: %s has precision below the minor unit", ErrInvalidAmount, strconv.FormatFloat(value, 'g', -1, 64))
	}
	return New(int64(rounded), currency, exponent)
}

// MajorString renders the amount without floating-point conversion.
func (m Money) MajorString() (string, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}
	negative := m.AmountMinor < 0
	abs := new(big.Int).Abs(big.NewInt(m.AmountMinor)).String()
	if m.Exponent > 0 {
		for len(abs) <= int(m.Exponent) {
			abs = "0" + abs
		}
		split := len(abs) - int(m.Exponent)
		abs = abs[:split] + "." + abs[split:]
	}
	if negative {
		return "-" + abs, nil
	}
	return abs, nil
}

func sameCurrency(left, right Money) error {
	if err := left.Validate(); err != nil {
		return err
	}
	if err := right.Validate(); err != nil {
		return err
	}
	if left.Currency != right.Currency || left.Exponent != right.Exponent {
		return fmt.Errorf("%w: %s/%d and %s/%d", ErrCurrencyMismatch, left.Currency, left.Exponent, right.Currency, right.Exponent)
	}
	return nil
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
