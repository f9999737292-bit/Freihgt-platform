package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Analytics projection tables are derived read models — never authoritative financial sources.
const (
	AnalyticsProjectionNamePeriod = "cost_analytics_period_projection"
	AnalyticsPeriodGrainMonth     = "MONTH"
	AnalyticsProjectionVersion    = 1

	AnalyticsProjectionStatusIdle   = "IDLE"
	AnalyticsProjectionStatusRunning = "RUNNING"
	AnalyticsProjectionStatusReady  = "READY"
	AnalyticsProjectionStatusStale  = "STALE"
	AnalyticsProjectionStatusError  = "ERROR"

	DataQualityAvailable = "AVAILABLE"
)

type AnalyticsOrderFact struct {
	TenantID               uuid.UUID
	TransportOrderID       uuid.UUID
	BuyerCompanyID         uuid.UUID
	CarrierCompanyID       uuid.UUID
	CurrencyCode           string
	PeriodStart            time.Time
	PeriodGrain            string
	PlannedAmount          *decimal.Decimal
	AccruedAmount          *decimal.Decimal
	CurrentActualAmount    *decimal.Decimal
	FinalActualAmount      *decimal.Decimal
	CurrentVarianceAmount  *decimal.Decimal
	FinalVarianceAmount    *decimal.Decimal
	DataStage              DataStage
	FinancialFinality      FinancialFinality
	SourceSummaryRevision  int64
	SourceSummaryUpdatedAt time.Time
	CalculatedAt           time.Time
}

type AnalyticsPeriodProjection struct {
	TenantID                 uuid.UUID
	BuyerCompanyID           uuid.UUID
	PeriodStart              time.Time
	PeriodGrain              string
	CurrencyCode             string
	OrderCount               int
	PlannedTotal             *decimal.Decimal
	AccruedTotal             *decimal.Decimal
	CurrentActualTotal       *decimal.Decimal
	FinalActualTotal         *decimal.Decimal
	CurrentVarianceTotal     *decimal.Decimal
	FinalVarianceTotal       *decimal.Decimal
	ReconciliationOpenCount  int
	CalculatedAt             time.Time
	DataThrough              time.Time
	ProjectionVersion        int
}

type AnalyticsProjectionState struct {
	ProjectionName       string
	TenantID             uuid.UUID
	ProjectionVersion    int
	SourceWatermark      *time.Time
	LastSuccessfulRunAt  *time.Time
	CalculatedAt         *time.Time
	DataThrough          *time.Time
	Status               string
	LastErrorCode        *string
	LastErrorMessage     *string
	UpdatedAt            time.Time
}

type AnalyticsDirtyEntry struct {
	TenantID         uuid.UUID
	TransportOrderID uuid.UUID
	BuyerCompanyID   uuid.UUID
	CurrencyCode     string
	PeriodStart      time.Time
	PeriodGrain      string
	DirtyAt          time.Time
	SourceEventID    *uuid.UUID
}

type AnalyticsPeriodKey struct {
	TenantID       uuid.UUID
	BuyerCompanyID uuid.UUID
	PeriodStart    time.Time
	PeriodGrain    string
	CurrencyCode   string
}

// PeriodStartFromSummaryUpdatedAt groups analytics by calendar month of cost_summary_projection.updated_at.
func PeriodStartFromSummaryUpdatedAt(updatedAt time.Time) time.Time {
	utc := updatedAt.UTC()
	return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func OrderFactFromCostSummary(projection *CostSummaryProjection, updatedAt time.Time, now time.Time) *AnalyticsOrderFact {
	if projection == nil || projection.CurrencyCode == "" {
		return nil
	}
	return &AnalyticsOrderFact{
		TenantID:               projection.TenantID,
		TransportOrderID:       projection.TransportOrderID,
		BuyerCompanyID:         projection.BuyerCompanyID,
		CarrierCompanyID:       projection.CarrierCompanyID,
		CurrencyCode:           projection.CurrencyCode,
		PeriodStart:            PeriodStartFromSummaryUpdatedAt(updatedAt),
		PeriodGrain:            AnalyticsPeriodGrainMonth,
		PlannedAmount:          projection.PlannedAmount,
		AccruedAmount:          projection.AccruedAmount,
		CurrentActualAmount:    projection.CurrentActualAmount,
		FinalActualAmount:      projection.FinalActualAmount,
		CurrentVarianceAmount:  projection.CurrentVarianceAmount,
		FinalVarianceAmount:    projection.FinalVarianceAmount,
		DataStage:              projection.DataStage,
		FinancialFinality:      projection.FinancialFinality,
		SourceSummaryRevision:  projection.ProjectionRevision,
		SourceSummaryUpdatedAt: updatedAt.UTC(),
		CalculatedAt:           now.UTC(),
	}
}

func SumDecimalPtr(values ...*decimal.Decimal) *decimal.Decimal {
	var total decimal.Decimal
	hasValue := false
	for _, value := range values {
		if value == nil {
			continue
		}
		total = total.Add(*value)
		hasValue = true
	}
	if !hasValue {
		return nil
	}
	rounded := total.Round(MoneyScale)
	return &rounded
}
