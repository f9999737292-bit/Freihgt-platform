package dto

import (
	"encoding/json"
	"strings"
	"time"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

const (
	BenchmarkCostBasisPerOrderFinality = "FINAL_ACTUAL_OR_CURRENT_ACTUAL_PER_ORDER"
)

type AnalyticsMoneyDTO struct {
	Amount       *string `json:"amount"`
	CurrencyCode string  `json:"currency_code"`
}

type AnalyticsFreshnessDTO struct {
	CalculatedAt       *string `json:"calculated_at,omitempty"`
	DataThrough        *string `json:"data_through,omitempty"`
	ProjectionVersion  int     `json:"projection_version"`
	BenchmarkCostBasis string  `json:"benchmark_cost_basis,omitempty"`
}

type AnalyticsPeriodDTO struct {
	From          string `json:"from"`
	To            string `json:"to"`
	DateDimension string `json:"date_dimension"`
}

type AnalyticsListEnvelope struct {
	CurrencyCode  string                `json:"currency_code"`
	Period        AnalyticsPeriodDTO    `json:"period"`
	DataQuality   string                `json:"data_quality"`
	MixedCurrency bool                  `json:"mixed_currency"`
	Freshness     AnalyticsFreshnessDTO `json:"freshness"`
	Items         any                   `json:"items"`
	Total         int                   `json:"total"`
	Limit         int                   `json:"limit"`
	Offset        int                   `json:"offset"`
}

type AnalyticsOverviewSummaryDTO struct {
	PlannedTotal              *string `json:"planned_total"`
	CurrentActualTotal        *string `json:"current_actual_total"`
	FinalActualTotal          *string `json:"final_actual_total"`
	CurrentVarianceTotal      *string `json:"current_variance_total,omitempty"`
	FinalVarianceTotal        *string `json:"final_variance_total,omitempty"`
	ReconciliationMismatchCount int   `json:"reconciliation_mismatch_count,omitempty"`
	OrderCount                int     `json:"order_count"`
}

type AnalyticsOverviewTopLaneDTO struct {
	LaneKey     string            `json:"lane_key"`
	LaneLabel   string            `json:"lane_label"`
	OrderCount  int               `json:"order_count"`
	SpendTotal  AnalyticsMoneyDTO `json:"spend_total"`
}

type AnalyticsOverviewAccessorialDTO struct {
	TotalAmount AnalyticsMoneyDTO `json:"total_amount"`
	OrderCount  int               `json:"order_count"`
}

type AnalyticsOverviewOpportunitySummaryDTO struct {
	Count      int                           `json:"count"`
	TopItems   []AnalyticsOpportunityItemDTO `json:"top_items"`
}

type AnalyticsOverviewResponse struct {
	CurrencyCode  string                              `json:"currency_code"`
	Period        AnalyticsPeriodDTO                  `json:"period"`
	DataQuality   string                              `json:"data_quality"`
	MixedCurrency bool                                `json:"mixed_currency"`
	Freshness     AnalyticsFreshnessDTO               `json:"freshness"`
	Summary       *AnalyticsOverviewSummaryDTO        `json:"summary,omitempty"`
	TopLanes      []AnalyticsOverviewTopLaneDTO       `json:"top_lanes,omitempty"`
	Accessorial   *AnalyticsOverviewAccessorialDTO    `json:"accessorial,omitempty"`
	Opportunities *AnalyticsOverviewOpportunitySummaryDTO `json:"opportunities,omitempty"`
}

type AnalyticsBenchmarkDTO struct {
	SampleSize  int               `json:"sample_size"`
	Mean        AnalyticsMoneyDTO `json:"mean"`
	Median      AnalyticsMoneyDTO `json:"median"`
	P25         AnalyticsMoneyDTO `json:"p25"`
	P75         AnalyticsMoneyDTO `json:"p75"`
	P90         AnalyticsMoneyDTO `json:"p90"`
	Min         AnalyticsMoneyDTO `json:"min"`
	Max         AnalyticsMoneyDTO `json:"max"`
	DataQuality string            `json:"data_quality"`
}

type AnalyticsLaneItemDTO struct {
	LaneKey            string              `json:"lane_key"`
	LaneLabel          string              `json:"lane_label"`
	OriginCountry      string              `json:"origin_country"`
	OriginCity         string              `json:"origin_city"`
	DestinationCountry string              `json:"destination_country"`
	DestinationCity    string              `json:"destination_city"`
	TransportMode      string              `json:"transport_mode"`
	EquipmentType      string              `json:"equipment_type"`
	OrderCount         int                 `json:"order_count"`
	CarrierCount       int                 `json:"carrier_count"`
	PlannedTotal       AnalyticsMoneyDTO   `json:"planned_total"`
	CurrentActualTotal AnalyticsMoneyDTO   `json:"current_actual_total"`
	FinalActualTotal   AnalyticsMoneyDTO   `json:"final_actual_total"`
	VarianceTotal      AnalyticsMoneyDTO   `json:"variance_total"`
	Benchmark          AnalyticsBenchmarkDTO `json:"benchmark"`
}

type AnalyticsCarrierItemDTO struct {
	CarrierCompanyID      string            `json:"carrier_company_id"`
	CarrierName           string            `json:"carrier_name"`
	OrderCount            int               `json:"order_count"`
	LaneCount             int               `json:"lane_count"`
	PlannedTotal          AnalyticsMoneyDTO `json:"planned_total"`
	CurrentActualTotal    AnalyticsMoneyDTO `json:"current_actual_total"`
	FinalActualTotal      AnalyticsMoneyDTO `json:"final_actual_total"`
	VarianceTotal         AnalyticsMoneyDTO `json:"variance_total"`
	ComparableOrderCount  int               `json:"comparable_order_count"`
	LaneNormalizedDelta   AnalyticsMoneyDTO `json:"lane_normalized_delta"`
	DataQuality           string            `json:"data_quality"`
}

type AnalyticsAccessorialItemDTO struct {
	NormalizedCategory string            `json:"normalized_category"`
	TotalAmount        AnalyticsMoneyDTO `json:"total_amount"`
	OrderCount         int               `json:"order_count"`
	LineCount          int               `json:"line_count"`
	ShareOfSpend       *string           `json:"share_of_spend"`
	AccessorialRate    *string           `json:"accessorial_order_rate"`
	DataQuality        string            `json:"data_quality"`
}

type AnalyticsOpportunityEvidenceDTO struct {
	ObservedCost     *string `json:"observed_cost,omitempty"`
	BaselineCost     *string `json:"baseline_cost,omitempty"`
	PotentialDelta   *string `json:"potential_delta,omitempty"`
	SampleSize       int     `json:"sample_size,omitempty"`
	CurrencyCode     string  `json:"currency_code,omitempty"`
	LaneKey          string  `json:"lane_key,omitempty"`
	CohortMedian     string  `json:"cohort_median,omitempty"`
	CohortP90        string  `json:"cohort_p90,omitempty"`
	CarrierCompanyID string  `json:"carrier_company_id,omitempty"`
	ReasonCode       string  `json:"reason_code,omitempty"`
	OccurrenceCount  int     `json:"occurrence_count,omitempty"`
	AccessorialRate  string  `json:"accessorial_rate,omitempty"`
	BaselineP75Rate  string  `json:"baseline_p75_rate,omitempty"`
}

type AnalyticsOpportunityItemDTO struct {
	OpportunityID   string                          `json:"opportunity_id"`
	Type            string                          `json:"type"`
	Scope           string                          `json:"scope"`
	EntityKey       string                          `json:"entity_key"`
	ObservedValue   AnalyticsMoneyDTO               `json:"observed_value"`
	BaselineValue   AnalyticsMoneyDTO               `json:"baseline_value"`
	EstimatedDelta  AnalyticsMoneyDTO               `json:"estimated_delta"`
	CurrencyCode    string                          `json:"currency_code"`
	SampleSize      int                             `json:"sample_size"`
	Evidence        AnalyticsOpportunityEvidenceDTO `json:"evidence"`
	DataQuality     string                          `json:"data_quality"`
	CalculatedAt    string                          `json:"calculated_at"`
	RuleVersion     int                             `json:"rule_version"`
}

func MoneyPtr(d *decimal.Decimal, currency string) AnalyticsMoneyDTO {
	if d == nil {
		return AnalyticsMoneyDTO{Amount: nil, CurrencyCode: currency}
	}
	s := d.Round(domain.MoneyScale).StringFixed(domain.MoneyScale)
	return AnalyticsMoneyDTO{Amount: &s, CurrencyCode: currency}
}

func MoneyValue(d decimal.Decimal, currency string) AnalyticsMoneyDTO {
	s := d.Round(domain.MoneyScale).StringFixed(domain.MoneyScale)
	return AnalyticsMoneyDTO{Amount: &s, CurrencyCode: currency}
}

func RatioString(d *decimal.Decimal) *string {
	if d == nil {
		return nil
	}
	s := d.StringFixed(4)
	return &s
}

func TimeRFC3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func TimePtrRFC3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func ToOpportunityEvidence(raw []byte) AnalyticsOpportunityEvidenceDTO {
	var internal domain.OpportunityEvidence
	if len(raw) == 0 {
		return AnalyticsOpportunityEvidenceDTO{}
	}
	_ = json.Unmarshal(raw, &internal)
	ev := AnalyticsOpportunityEvidenceDTO{
		SampleSize:       internal.SampleSize,
		CurrencyCode:     internal.CurrencyCode,
		LaneKey:          internal.LaneKey,
		CohortMedian:     internal.CohortMedian,
		CohortP90:        internal.CohortP90,
		CarrierCompanyID: internal.CarrierCompanyID,
		ReasonCode:       internal.ReasonCode,
		OccurrenceCount:  internal.OccurrenceCount,
		AccessorialRate:  internal.AccessorialRate,
		BaselineP75Rate:  internal.BaselineP75Rate,
	}
	if internal.ObservedCost != "" {
		ev.ObservedCost = &internal.ObservedCost
	}
	if internal.BaselineCost != "" {
		ev.BaselineCost = &internal.BaselineCost
	}
	if internal.PotentialDelta != "" {
		ev.PotentialDelta = &internal.PotentialDelta
	}
	return ev
}

func ToOpportunityItem(p domain.AnalyticsOpportunityProjection) AnalyticsOpportunityItemDTO {
	currency := p.CurrencyCode
	return AnalyticsOpportunityItemDTO{
		OpportunityID:  p.OpportunityID.String(),
		Type:           p.OpportunityType,
		Scope:          p.Scope,
		EntityKey:      p.EntityKey,
		ObservedValue:  MoneyValue(p.ObservedAmount, currency),
		BaselineValue:  MoneyValue(p.BaselineAmount, currency),
		EstimatedDelta: MoneyValue(p.EstimatedDelta, currency),
		CurrencyCode:   currency,
		SampleSize:     p.SampleSize,
		Evidence:       ToOpportunityEvidence(p.EvidenceJSON),
		DataQuality:    p.DataQuality,
		CalculatedAt:   TimeRFC3339(p.CalculatedAt),
		RuleVersion:    p.RuleVersion,
	}
}

func ToBenchmarkDTO(b *domain.AnalyticsBenchmarkProjection) AnalyticsBenchmarkDTO {
	currency := b.CurrencyCode
	dto := AnalyticsBenchmarkDTO{
		SampleSize:  b.SampleCount,
		DataQuality: b.DataQuality,
		Mean:        MoneyPtr(b.MeanAmount, currency),
		Median:      MoneyPtr(b.MedianAmount, currency),
		P25:         MoneyPtr(b.P25Amount, currency),
		P75:         MoneyPtr(b.P75Amount, currency),
		P90:         MoneyPtr(b.P90Amount, currency),
		Min:         MoneyPtr(b.MinAmount, currency),
		Max:         MoneyPtr(b.MaxAmount, currency),
	}
	return dto
}

func LaneLabelFromKey(laneKey string, snapshot *string) string {
	if snapshot != nil {
		if label := strings.TrimSpace(*snapshot); label != "" {
			return label
		}
	}
	_, originCity, _, destCity, mode, equipment, ok := domain.ParseLaneKeyComponents(laneKey)
	if !ok {
		return laneKey
	}
	return domain.BuildLaneLabel(originCity, destCity, mode, equipment)
}
