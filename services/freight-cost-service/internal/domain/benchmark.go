package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Benchmark and opportunity projections are tenant-only derived read models.
var NamespaceFreightCostOpportunity = uuid.MustParse("a7b3c4d5-e6f7-4890-abcd-ef1234567890")

const (
	AnalyticsProjectionNameBenchmark    = "cost_analytics_benchmark_projection"
	AnalyticsProjectionNameOpportunity  = "cost_analytics_opportunity_projection"
	AnalyticsBenchmarkProjectionVersion = 1
	AnalyticsOpportunityProjectionVersion = 1

	BenchmarkCohortTypeLane = "LANE"

	BenchmarkRuleVersion    = 1
	OpportunityRuleVersion  = 1

	OpportunityTypeLaneCostOutlier      = "LANE_COST_OUTLIER"
	OpportunityTypeCostAboveLaneMedian  = "COST_ABOVE_LANE_MEDIAN"
	OpportunityTypeCarrierCostOutlier   = "CARRIER_COST_OUTLIER"
	OpportunityTypeHighAccessorialRate  = "HIGH_ACCESSORIAL_RATE"
	OpportunityTypeRepeatedVariance     = "REPEATED_VARIANCE"

	OpportunityScopeLane        = "LANE"
	OpportunityScopeCarrier     = "CARRIER"
	OpportunityScopeOrder       = "ORDER"
	OpportunityScopeAccessorial = "ACCESSORIAL"
	OpportunityScopeVariance    = "VARIANCE"

	PercentileMethodPostgresContinuous = "percentile_cont"
)

// DefaultMinBenchmarkSample is the frozen default when config is unset.
const DefaultMinBenchmarkSample = 5

// DefaultRepeatedVarianceMinOccurrences is the minimum order count for REPEATED_VARIANCE.
const DefaultRepeatedVarianceMinOccurrences = 3

type AnalyticsBenchmarkConfig struct {
	MinBenchmarkSample             int
	RepeatedVarianceMinOccurrences int
}

func (c AnalyticsBenchmarkConfig) EffectiveMinSample() int {
	if c.MinBenchmarkSample <= 0 {
		return DefaultMinBenchmarkSample
	}
	return c.MinBenchmarkSample
}

func (c AnalyticsBenchmarkConfig) EffectiveRepeatedVarianceMin() int {
	if c.RepeatedVarianceMinOccurrences <= 0 {
		return DefaultRepeatedVarianceMinOccurrences
	}
	return c.RepeatedVarianceMinOccurrences
}

type AnalyticsBenchmarkKey struct {
	TenantID       uuid.UUID
	BuyerCompanyID uuid.UUID
	CohortType     string
	LaneKey        string
	TransportMode  string
	EquipmentType  string
	PeriodStart    time.Time
	PeriodGrain    string
	CurrencyCode   string
}

type AnalyticsBenchmarkProjection struct {
	TenantID          uuid.UUID
	BuyerCompanyID    uuid.UUID
	CohortType        string
	LaneKey           string
	TransportMode     string
	EquipmentType     string
	PeriodStart       time.Time
	PeriodGrain       string
	CurrencyCode      string
	SampleCount       int
	MeanAmount        *decimal.Decimal
	MedianAmount      *decimal.Decimal
	P25Amount         *decimal.Decimal
	P75Amount         *decimal.Decimal
	P90Amount         *decimal.Decimal
	MinAmount         *decimal.Decimal
	MaxAmount         *decimal.Decimal
	DataQuality       string
	RuleVersion       int
	CalculatedAt      time.Time
	DataThrough       time.Time
	ProjectionVersion int
}

type OpportunityEvidence struct {
	SchemaVersion   int    `json:"schema_version"`
	ObservedCost    string `json:"observed_cost,omitempty"`
	BaselineCost    string `json:"baseline_cost,omitempty"`
	PotentialDelta  string `json:"potential_delta,omitempty"`
	SampleSize      int    `json:"sample_size,omitempty"`
	CurrencyCode    string `json:"currency_code,omitempty"`
	LaneKey         string `json:"lane_key,omitempty"`
	CohortMedian    string `json:"cohort_median,omitempty"`
	CohortP90       string `json:"cohort_p90,omitempty"`
	CarrierCompanyID string `json:"carrier_company_id,omitempty"`
	ReasonCode      string `json:"reason_code,omitempty"`
	OccurrenceCount int    `json:"occurrence_count,omitempty"`
	AccessorialRate string `json:"accessorial_rate,omitempty"`
	BaselineP75Rate string `json:"baseline_p75_rate,omitempty"`
}

func (e OpportunityEvidence) JSON() ([]byte, error) {
	if e.SchemaVersion == 0 {
		e.SchemaVersion = 1
	}
	return json.Marshal(e)
}

type AnalyticsOpportunityProjection struct {
	TenantID           uuid.UUID
	BuyerCompanyID     uuid.UUID
	OpportunityID      uuid.UUID
	OpportunityType    string
	Scope              string
	EntityKey          string
	CurrencyCode       string
	TransportOrderID   *uuid.UUID
	CarrierCompanyID   *uuid.UUID
	LaneKey            *string
	PeriodStart        time.Time
	PeriodGrain        string
	ObservedAmount     decimal.Decimal
	BaselineAmount     decimal.Decimal
	EstimatedDelta     decimal.Decimal
	SampleSize         int
	DataQuality        string
	RuleVersion        int
	EvidenceJSON       []byte
	CalculatedAt       time.Time
	DataThrough        time.Time
	ProjectionVersion  int
}

// BenchmarkEligibleCostAmount returns the canonical per-order benchmark amount.
// Each order contributes exactly one amount based on financial finality — never mixed.
func BenchmarkEligibleCostAmount(fact *AnalyticsOrderFact) *decimal.Decimal {
	if fact == nil || !fact.LaneEligible || fact.LaneKey == nil {
		return nil
	}
	switch fact.FinancialFinality {
	case FinancialFinalityFinalActual:
		if fact.FinalActualAmount != nil {
			v := fact.FinalActualAmount.Round(MoneyScale)
			return &v
		}
	case FinancialFinalityCurrentActual:
		if fact.CurrentActualAmount != nil {
			v := fact.CurrentActualAmount.Round(MoneyScale)
			return &v
		}
	}
	return nil
}

func BenchmarkDataQualityForSample(sampleCount, minSample int) string {
	if sampleCount < minSample {
		return DataQualityInsufficientSample
	}
	return DataQualityAvailable
}

func DeriveOpportunityID(
	tenantID, buyerCompanyID uuid.UUID,
	opportunityType, scope, entityKey, currencyCode string,
	periodStart time.Time,
	ruleVersion int,
) uuid.UUID {
	key := strings.Join([]string{
		tenantID.String(),
		buyerCompanyID.String(),
		opportunityType,
		scope,
		entityKey,
		currencyCode,
		periodStart.Format("2006-01-02"),
		fmt.Sprintf("%d", ruleVersion),
	}, "|")
	return uuid.NewSHA1(NamespaceFreightCostOpportunity, []byte(key))
}

func MoneyString(d decimal.Decimal) string {
	return d.Round(MoneyScale).StringFixed(MoneyScale)
}
