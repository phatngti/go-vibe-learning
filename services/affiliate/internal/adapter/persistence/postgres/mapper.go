package postgres

import (
	sharedmoney "github/master_go_cqrs/internal/shared/money"
	"github/master_go_cqrs/services/affiliate/internal/domain"
)

func referralToRow(e *domain.Referral) *ReferralRow {
	return &ReferralRow{ReferrerID: e.ReferrerID, RefereeID: e.RefereeID, CreatedAt: e.CreatedAt}
}
func metricToDomain(r *AgentMetricRow) *domain.AgentMetric {
	metric := &domain.AgentMetric{AgentID: r.AgentID, AccumulatedVolume: r.AccumulatedVolume, ReferralCount: r.ReferralCount, UpdatedAt: r.UpdatedAt}
	if r.AccumulatedVolumeMinor != nil && r.Currency != nil && r.CurrencyExponent != nil {
		metric.AccumulatedVolumeMoney = &sharedmoney.Money{
			AmountMinor: *r.AccumulatedVolumeMinor, Currency: *r.Currency, Exponent: *r.CurrencyExponent,
		}
	}
	return metric
}
func metricToRow(e *domain.AgentMetric) *AgentMetricRow {
	row := &AgentMetricRow{AgentID: e.AgentID, AccumulatedVolume: e.AccumulatedVolume, ReferralCount: e.ReferralCount, UpdatedAt: e.UpdatedAt}
	if e.AccumulatedVolumeMoney != nil {
		amountMinor := e.AccumulatedVolumeMoney.AmountMinor
		currency := e.AccumulatedVolumeMoney.Currency
		exponent := e.AccumulatedVolumeMoney.Exponent
		row.AccumulatedVolumeMinor = &amountMinor
		row.Currency = &currency
		row.CurrencyExponent = &exponent
	}
	return row
}
func levelToDomain(r *AffiliateLevelRow) *domain.AffiliateLevel {
	return &domain.AffiliateLevel{AgentID: r.AgentID, CurrentRank: domain.Rank(r.CurrentRank), PromotedAt: r.PromotedAt}
}
func levelToRow(e *domain.AffiliateLevel) *AffiliateLevelRow {
	return &AffiliateLevelRow{AgentID: e.AgentID, CurrentRank: string(e.CurrentRank), PromotedAt: e.PromotedAt}
}
