package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Analytics projection tables are derived read models — never authoritative financial sources.
const (
	AnalyticsProjectionNamePeriod      = "cost_analytics_period_projection"
	AnalyticsProjectionNameLane        = "cost_analytics_lane_period_projection"
	AnalyticsProjectionNameCarrier     = "cost_analytics_carrier_period_projection"
	AnalyticsProjectionNameAccessorial = "cost_analytics_accessorial_period_projection"
	AnalyticsPeriodGrainMonth            = "MONTH"
	AnalyticsProjectionVersion           = 1
	AnalyticsLaneProjectionVersion       = 1
	AnalyticsCarrierProjectionVersion    = 1
	AnalyticsAccessorialProjectionVersion = 1

	AccessorialStatusApproved = "APPROVED"
	AccessorialStatusProposed = "PROPOSED"
	AccessorialStatusRejected = "REJECTED"
	AccessorialStatusDisputed = "DISPUTED"

	AnalyticsProjectionStatusIdle    = "IDLE"
	AnalyticsProjectionStatusRunning = "RUNNING"
	AnalyticsProjectionStatusReady   = "READY"
	AnalyticsProjectionStatusStale   = "STALE"
	AnalyticsProjectionStatusError   = "ERROR"

	DataQualityAvailable           = "AVAILABLE"
	DataQualityPartial             = "PARTIAL"
	DataQualityNotAvailable        = "NOT_AVAILABLE"
	DataQualityInsufficientSample  = "INSUFFICIENT_SAMPLE"
	DataQualityStale               = "STALE"
	DataQualityMixedCurrency       = "MIXED_CURRENCY"
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
	LaneKey                *string
	OriginCountry          *string
	OriginCity             *string
	DestinationCountry     *string
	DestinationCity        *string
	TransportMode          *string
	EquipmentType          *string
	LaneEligible           bool
	OrderReference         *string
	CarrierDisplayName     *string
	LaneLabel              *string
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

type AnalyticsLanePeriodKey struct {
	TenantID       uuid.UUID
	BuyerCompanyID uuid.UUID
	LaneKey        string
	TransportMode  string
	EquipmentType  string
	PeriodStart    time.Time
	PeriodGrain    string
	CurrencyCode   string
}

type AnalyticsLanePeriodProjection struct {
	TenantID            uuid.UUID
	BuyerCompanyID      uuid.UUID
	LaneKey             string
	TransportMode       string
	EquipmentType       string
	PeriodStart         time.Time
	PeriodGrain         string
	CurrencyCode        string
	OrderCount          int
	CarrierCount        int
	PlannedTotal        *decimal.Decimal
	AccruedTotal        *decimal.Decimal
	CurrentActualTotal  *decimal.Decimal
	FinalActualTotal    *decimal.Decimal
	CurrentVarianceTotal *decimal.Decimal
	FinalVarianceTotal  *decimal.Decimal
	CalculatedAt        time.Time
	DataThrough         time.Time
	ProjectionVersion   int
}

type AnalyticsCarrierPeriodKey struct {
	TenantID         uuid.UUID
	BuyerCompanyID   uuid.UUID
	CarrierCompanyID uuid.UUID
	PeriodStart      time.Time
	PeriodGrain      string
	CurrencyCode     string
}

type AnalyticsCarrierPeriodProjection struct {
	TenantID            uuid.UUID
	BuyerCompanyID      uuid.UUID
	CarrierCompanyID    uuid.UUID
	PeriodStart         time.Time
	PeriodGrain         string
	CurrencyCode        string
	OrderCount          int
	LaneCount           int
	PlannedTotal        *decimal.Decimal
	AccruedTotal        *decimal.Decimal
	CurrentActualTotal  *decimal.Decimal
	FinalActualTotal    *decimal.Decimal
	CurrentVarianceTotal *decimal.Decimal
	FinalVarianceTotal  *decimal.Decimal
	CalculatedAt        time.Time
	DataThrough         time.Time
	ProjectionVersion   int
}

type AnalyticsAccessorialFact struct {
	TenantID             uuid.UUID
	AccessorialID        uuid.UUID
	CurrencyCode         string
	TransportOrderID     uuid.UUID
	BuyerCompanyID       uuid.UUID
	SettlementID         uuid.UUID
	ChargeCode           string
	NormalizedCategory   string
	Amount               decimal.Decimal
	Status               string
	MappingVersion       int64
	MappingEvaluatedAt   time.Time
	PeriodStart          time.Time
	PeriodGrain          string
	Eligible             bool
	CalculatedAt         time.Time
}

type AnalyticsAccessorialPeriodKey struct {
	TenantID           uuid.UUID
	BuyerCompanyID     uuid.UUID
	NormalizedCategory string
	PeriodStart        time.Time
	PeriodGrain        string
	CurrencyCode       string
}

type AnalyticsAccessorialPeriodProjection struct {
	TenantID             uuid.UUID
	BuyerCompanyID       uuid.UUID
	NormalizedCategory   string
	PeriodStart          time.Time
	PeriodGrain          string
	CurrencyCode         string
	TotalAmount          *decimal.Decimal
	OrderCount           int
	LineCount            int
	ShareOfSpend         *decimal.Decimal
	AccessorialOrderRate *decimal.Decimal
	FreightSpendTotal    *decimal.Decimal
	CalculatedAt         time.Time
	DataThrough          time.Time
	ProjectionVersion    int
}

func (f *AnalyticsAccessorialFact) PeriodKey() AnalyticsAccessorialPeriodKey {
	return AnalyticsAccessorialPeriodKey{
		TenantID:           f.TenantID,
		BuyerCompanyID:     f.BuyerCompanyID,
		NormalizedCategory: f.NormalizedCategory,
		PeriodStart:        f.PeriodStart,
		PeriodGrain:        f.PeriodGrain,
		CurrencyCode:       f.CurrencyCode,
	}
}

type AnalyticsProjectionCoverage struct {
	ProjectionName                string
	TenantID                      uuid.UUID
	CalculatedAt                  time.Time
	SourceOrderCount              int
	EligibleOrderCount            int
	ExcludedOrderCount            int
	ExcludedMissingOriginCity     int
	ExcludedMissingDestinationCity int
	ExcludedMissingCountry        int
	ExcludedMissingMode           int
	ExcludedMissingCarrierID      int
	ExcludedProposedCount         int
	ExcludedRejectedCount         int
	ExcludedCancelledCount        int
	UnmappedChargeCodeCount       int
	MissingCarrierDisplayCount    int
	MissingOrderReferenceCount    int
	DataQuality                   string
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

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

// ApplyTransportDimension attaches canonical lane components to an order fact.
func ApplyTransportDimension(
	fact *AnalyticsOrderFact,
	originCountry string,
	originCity, destinationCity *string,
	destinationCountry, transportMode string,
	equipmentType *string,
) {
	if fact == nil {
		return
	}
	originCityValue := ""
	if originCity != nil {
		originCityValue = *originCity
	}
	destCityValue := ""
	if destinationCity != nil {
		destCityValue = *destinationCity
	}
	equipmentValue := ""
	if equipmentType != nil {
		equipmentValue = *equipmentType
	}
	lane := BuildLaneKey(LaneKeyInput{
		OriginCountry:      originCountry,
		OriginCity:         originCityValue,
		DestinationCountry: destinationCountry,
		DestinationCity:    destCityValue,
		TransportMode:      transportMode,
		EquipmentType:      equipmentValue,
	})
	if lane.Available {
		fact.LaneKey = stringPtr(lane.LaneKey)
		fact.LaneEligible = true
	} else {
		fact.LaneKey = nil
		fact.LaneEligible = false
	}
	if lane.OriginCountry != "" {
		fact.OriginCountry = stringPtr(lane.OriginCountry)
	}
	if lane.OriginCity != "" {
		fact.OriginCity = stringPtr(lane.OriginCity)
	}
	if lane.DestinationCountry != "" {
		fact.DestinationCountry = stringPtr(lane.DestinationCountry)
	}
	if lane.DestinationCity != "" {
		fact.DestinationCity = stringPtr(lane.DestinationCity)
	}
	if lane.TransportMode != "" {
		fact.TransportMode = stringPtr(lane.TransportMode)
	}
	if lane.EquipmentType != "" {
		fact.EquipmentType = stringPtr(lane.EquipmentType)
	}
}

func (f *AnalyticsOrderFact) LanePeriodKey() *AnalyticsLanePeriodKey {
	if f == nil || !f.LaneEligible || f.LaneKey == nil || f.TransportMode == nil || f.EquipmentType == nil {
		return nil
	}
	return &AnalyticsLanePeriodKey{
		TenantID:       f.TenantID,
		BuyerCompanyID: f.BuyerCompanyID,
		LaneKey:        *f.LaneKey,
		TransportMode:  *f.TransportMode,
		EquipmentType:  *f.EquipmentType,
		PeriodStart:    f.PeriodStart,
		PeriodGrain:    f.PeriodGrain,
		CurrencyCode:   f.CurrencyCode,
	}
}

func (f *AnalyticsOrderFact) CarrierPeriodKey() *AnalyticsCarrierPeriodKey {
	if f == nil || f.CarrierCompanyID == uuid.Nil {
		return nil
	}
	return &AnalyticsCarrierPeriodKey{
		TenantID:         f.TenantID,
		BuyerCompanyID:   f.BuyerCompanyID,
		CarrierCompanyID: f.CarrierCompanyID,
		PeriodStart:      f.PeriodStart,
		PeriodGrain:      f.PeriodGrain,
		CurrencyCode:     f.CurrencyCode,
	}
}

func (f *AnalyticsOrderFact) PeriodKey() *AnalyticsPeriodKey {
	if f == nil {
		return nil
	}
	return &AnalyticsPeriodKey{
		TenantID:       f.TenantID,
		BuyerCompanyID: f.BuyerCompanyID,
		PeriodStart:    f.PeriodStart,
		PeriodGrain:    f.PeriodGrain,
		CurrencyCode:   f.CurrencyCode,
	}
}
