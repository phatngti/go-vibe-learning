package command

import (
	"errors"
	"testing"

	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/affiliate/internal/domain"
)

func TestEvaluateRankThresholds(t *testing.T) {
	tests := []struct {
		name   string
		volume float64
		want   domain.Rank
	}{
		{name: "bronze below silver", volume: 999, want: domain.RankBronze},
		{name: "silver threshold", volume: 1000, want: domain.RankSilver},
		{name: "gold threshold", volume: 10000, want: domain.RankGold},
		{name: "platinum threshold", volume: 100000, want: domain.RankPlatinum},
		{name: "diamond threshold", volume: 1000000, want: domain.RankDiamond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := evaluateRank(tt.volume); got != tt.want {
				t.Fatalf("evaluateRank(%v) = %s, want %s", tt.volume, got, tt.want)
			}
		})
	}
}

func TestEvaluateRankMoneyUsesCurrencyExponent(t *testing.T) {
	tests := []struct {
		money sharedmoney.Money
		want  domain.Rank
	}{
		{money: sharedmoney.Money{AmountMinor: 99_999, Currency: "USD", Exponent: 2}, want: domain.RankBronze},
		{money: sharedmoney.Money{AmountMinor: 100_000, Currency: "USD", Exponent: 2}, want: domain.RankSilver},
		{money: sharedmoney.Money{AmountMinor: 1_000, Currency: "VND", Exponent: 0}, want: domain.RankSilver},
	}
	for _, test := range tests {
		got, err := evaluateRankMoney(test.money)
		if err != nil || got != test.want {
			t.Fatalf("evaluateRankMoney(%+v) = %s, %v; want %s", test.money, got, err, test.want)
		}
	}
}

func TestMetricMoneyPreservesCurrencyInvariant(t *testing.T) {
	stored := sharedmoney.Money{AmountMinor: 100, Currency: "USD", Exponent: 2}
	current, err := metricMoney(&domain.AgentMetric{AccumulatedVolumeMoney: &stored}, sharedmoney.Money{AmountMinor: 100, Currency: "VND", Exponent: 0})
	if err != nil {
		t.Fatal(err)
	}
	_, err = current.Add(sharedmoney.Money{AmountMinor: 100, Currency: "VND", Exponent: 0})
	if !errors.Is(err, sharedmoney.ErrCurrencyMismatch) {
		t.Fatalf("currency mismatch error = %v", err)
	}
}
