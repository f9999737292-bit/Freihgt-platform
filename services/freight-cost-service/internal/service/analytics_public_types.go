package service

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type AnalyticsPublicPeriod struct {
	From          string
	To            string
	DateDimension string
}

type AnalyticsPublicFreshness struct {
	CalculatedAt       *time.Time
	DataThrough        *time.Time
	ProjectionVersion  int
	BenchmarkCostBasis string
}

type AnalyticsPublicMoney struct {
	Amount       *decimal.Decimal
	CurrencyCode string
}

type AnalyticsPublicOverviewSummary struct {
	PlannedTotal                decimal.Decimal
	CurrentActualTotal          decimal.Decimal
	FinalActualTotal            decimal.Decimal
	CurrentVarianceTotal        decimal.Decimal
	FinalVarianceTotal          decimal.Decimal
	ReconciliationMismatchCount int
	OrderCount                  int
}

type AnalyticsPublicOverviewTopLane struct {
	LaneKey    string
	LaneLabel  string
	OrderCount int
	SpendTotal decimal.Decimal
	Currency   string
}

type AnalyticsPublicOverviewAccessorial struct {
	TotalAmount decimal.Decimal
	OrderCount  int
	Currency    string
}

type AnalyticsPublicOverviewResult struct {
	CurrencyCode  string
	Period        AnalyticsPublicPeriod
	DataQuality   string
	MixedCurrency bool
	Freshness     AnalyticsPublicFreshness
	Summary       *AnalyticsPublicOverviewSummary
	TopLanes      []AnalyticsPublicOverviewTopLane
	Accessorial   *AnalyticsPublicOverviewAccessorial
	Opportunities []domain.AnalyticsOpportunityProjection
	OpportunityCount int
}

type AnalyticsPublicBenchmark struct {
	SampleSize  int
	Mean        *decimal.Decimal
	Median      *decimal.Decimal
	P25         *decimal.Decimal
	P75         *decimal.Decimal
	P90         *decimal.Decimal
	Min         *decimal.Decimal
	Max         *decimal.Decimal
	DataQuality string
	Currency    string
}

type AnalyticsPublicLaneItem struct {
	Projection domain.AnalyticsLanePeriodProjection
	LaneLabel  string
	Benchmark  *AnalyticsPublicBenchmark
}

type AnalyticsPublicCarrierItem struct {
	Projection           domain.AnalyticsCarrierPeriodProjection
	CarrierName          string
	ComparableOrderCount int
	LaneNormalizedDelta  decimal.Decimal
	DataQuality          string
}

type AnalyticsPublicAccessorialItem struct {
	Projection domain.AnalyticsAccessorialPeriodProjection
}

type AnalyticsPublicListResult struct {
	CurrencyCode  string
	Period        AnalyticsPublicPeriod
	DataQuality   string
	MixedCurrency bool
	Freshness     AnalyticsPublicFreshness
	Lanes         []AnalyticsPublicLaneItem
	Carriers      []AnalyticsPublicCarrierItem
	Accessorials  []AnalyticsPublicAccessorialItem
	Opportunities []domain.AnalyticsOpportunityProjection
	Total         int
	Limit         int
	Offset        int
}

const analyticsPublicBenchmarkCostBasis = "FINAL_ACTUAL_OR_CURRENT_ACTUAL_PER_ORDER"
