package command

import (
	"testing"

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
