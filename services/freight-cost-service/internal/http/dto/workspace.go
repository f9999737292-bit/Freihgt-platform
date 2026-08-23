package dto

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/security"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

const (
	DataCapabilityAvailable    = "AVAILABLE"
	DataCapabilityNotAvailable = "NOT_AVAILABLE"
)

type WorkspaceSummaryDTO struct {
	TransportOrderID            string   `json:"transport_order_id"`
	ShipmentID                  *string  `json:"shipment_id"`
	BuyerCompanyID              string   `json:"buyer_company_id"`
	CarrierCompanyID            string   `json:"carrier_company_id"`
	CurrencyCode                string   `json:"currency_code"`
	DataStage                   string   `json:"data_stage"`
	FinancialFinality           string   `json:"financial_finality"`
	SourcesAvailable            []string `json:"sources_available"`
	PlannedAmount               *string  `json:"planned_amount"`
	AccruedAmount               *string  `json:"accrued_amount,omitempty"`
	ForecastExposure            *string  `json:"forecast_exposure,omitempty"`
	ForecastSourceStatus        string   `json:"forecast_source_status"`
	CurrentActualAmount         *string  `json:"current_actual_amount"`
	FinalActualAmount           *string  `json:"final_actual_amount"`
	BillingRegisterAmount       *string  `json:"billing_register_amount"`
	PaidAmount                  *string  `json:"paid_amount"`
	CurrentVarianceAmount       *string  `json:"current_variance_amount,omitempty"`
	FinalVarianceAmount         *string  `json:"final_variance_amount,omitempty"`
	CurrentVariancePercent      *string  `json:"current_variance_percent"`
	FinalVariancePercent        *string  `json:"final_variance_percent"`
	BillingReconciliationStatus *string  `json:"billing_reconciliation_status"`
	CostUpdatedAt               string   `json:"cost_updated_at"`
	AvailabilityReasons         []string `json:"availability_reasons,omitempty"`
}

type WorkspaceListResponse struct {
	Items  []WorkspaceSummaryDTO `json:"items"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

type WorkspacePeriodDTO struct {
	From           string `json:"from"`
	To             string `json:"to"`
	DateDimension  string `json:"date_dimension"`
}

type WorkspaceKpisDTO struct {
	PlannedTotal                   *string `json:"planned_total"`
	AccruedTotal                   *string `json:"accrued_total,omitempty"`
	ForecastExposureTotal          *string `json:"forecast_exposure_total,omitempty"`
	PendingProposedAccessorialTotal *string `json:"pending_proposed_accessorial_total,omitempty"`
	CurrentActualTotal             *string `json:"current_actual_total"`
	FinalActualTotal               *string `json:"final_actual_total"`
	CurrentVarianceTotal           *string `json:"current_variance_total,omitempty"`
	FinalVarianceTotal             *string `json:"final_variance_total,omitempty"`
	ReconciliationMismatchCount    int     `json:"reconciliation_mismatch_count,omitempty"`
}

type WorkspaceAggregateResponse struct {
	CurrencyCode   string             `json:"currency_code"`
	Period         WorkspacePeriodDTO `json:"period"`
	Kpis           WorkspaceKpisDTO   `json:"kpis"`
	MixedCurrency  bool               `json:"mixed_currency"`
}

type WorkspaceVarianceDriverDTO struct {
	DriverType  string  `json:"driver_type"`
	Category    *string `json:"category"`
	Amount      *string `json:"amount"`
	Description string  `json:"description"`
}

type WorkspaceReconciliationFindingDTO struct {
	FindingID   string `json:"finding_id"`
	FindingType string `json:"finding_type"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

type WorkspaceDetailResponse struct {
	Summary                WorkspaceSummaryDTO               `json:"summary"`
	OrderReference         string                            `json:"order_reference"`
	CarrierName            string                            `json:"carrier_name"`
	PlannedSource          *string                           `json:"planned_source"`
	VarianceDrivers        []WorkspaceVarianceDriverDTO      `json:"variance_drivers"`
	ReconciliationFindings []WorkspaceReconciliationFindingDTO `json:"reconciliation_findings"`
}

type WorkspaceVarianceDetailResponse struct {
	TransportOrderID       string                            `json:"transport_order_id"`
	VarianceDrivers        []WorkspaceVarianceDriverDTO      `json:"variance_drivers"`
	ReconciliationFindings []WorkspaceReconciliationFindingDTO `json:"reconciliation_findings"`
}

type WorkspaceAccessorialRowDTO struct {
	NormalizedCategory string `json:"normalized_category"`
	TotalAmount        string `json:"total_amount"`
	CurrencyCode       string `json:"currency_code"`
	OrderCount         int    `json:"order_count"`
}

type WorkspaceAccessorialResponse struct {
	Items           []WorkspaceAccessorialRowDTO `json:"items"`
	CurrencyCode    string                       `json:"currency_code"`
	DataCapability  string                       `json:"data_capability"`
}

type WorkspaceCarrierPerformanceRowDTO struct {
	CarrierCompanyID   string  `json:"carrier_company_id"`
	CarrierName        string  `json:"carrier_name"`
	OrderCount         int     `json:"order_count"`
	PlannedTotal       *string `json:"planned_total"`
	CurrentActualTotal *string `json:"current_actual_total"`
	FinalActualTotal   *string `json:"final_actual_total"`
	CurrentVarianceTotal *string `json:"current_variance_total"`
	CurrencyCode       string  `json:"currency_code"`
}

type WorkspaceCarrierPerformanceResponse struct {
	Items        []WorkspaceCarrierPerformanceRowDTO `json:"items"`
	CurrencyCode string                              `json:"currency_code"`
}

type WorkspaceLanePerformanceRowDTO struct {
	OriginLocationCode      string  `json:"origin_location_code"`
	DestinationLocationCode string  `json:"destination_location_code"`
	LaneLabel               string  `json:"lane_label"`
	OrderCount              int     `json:"order_count"`
	PlannedTotal            *string `json:"planned_total"`
	CurrentActualTotal      *string `json:"current_actual_total"`
	FinalActualTotal        *string `json:"final_actual_total"`
	CurrentVarianceTotal    *string `json:"current_variance_total"`
	CurrencyCode            string  `json:"currency_code"`
}

type WorkspaceLanePerformanceResponse struct {
	Items          []WorkspaceLanePerformanceRowDTO `json:"items"`
	CurrencyCode   string                           `json:"currency_code"`
	DataCapability string                           `json:"data_capability"`
}

func DecimalPtrToDTO(value *decimal.Decimal) *string {
	if value == nil {
		return nil
	}
	formatted := domain.FormatMoneyAmount(*value)
	return &formatted
}

func PercentPtrToDTO(value *decimal.Decimal) *string {
	if value == nil {
		return nil
	}
	formatted := value.StringFixed(4)
	return &formatted
}

func SanitizeDisplayLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if _, err := uuid.Parse(value); err == nil {
		return ""
	}
	return value
}

func ToWorkspaceSummaryDTO(projection *domain.CostSummaryProjection, actor security.TrustedActor, updatedAt time.Time) WorkspaceSummaryDTO {
	summary, err := domain.ProjectionToCostSummary(projection)
	if err != nil || summary == nil {
		return WorkspaceSummaryDTO{}
	}
	scope := domain.ViewScopeForActorKind(actor.ActorKind)
	masked := domain.ApplyViewScope(scope, summary)

	forecastStatus := projection.ForecastSourceStatus
	if forecastStatus == "" {
		forecastStatus = domain.ForecastSourceUnknown
	}
	if forecastStatus == domain.ForecastSourceKnown && projection.ForecastExposure == nil {
		forecastStatus = "KNOWN_EMPTY"
	}

	reconciliation := string(projection.BillingReconciliationStatus)
	var reasons []string
	if actor.ActorKind != security.ActorKindCarrier {
		for _, attr := range domain.BuildAvailabilityReasons(projection, domain.VarianceKindCurrent) {
			reasons = append(reasons, attr.ReasonCode)
		}
	}

	return WorkspaceSummaryDTO{
		TransportOrderID:            projection.TransportOrderID.String(),
		ShipmentID:                  nil,
		BuyerCompanyID:              projection.BuyerCompanyID.String(),
		CarrierCompanyID:            projection.CarrierCompanyID.String(),
		CurrencyCode:                projection.CurrencyCode,
		DataStage:                   string(projection.DataStage),
		FinancialFinality:           string(projection.FinancialFinality),
		SourcesAvailable:            projection.SourcesAvailable,
		PlannedAmount:               DecimalPtrToDTO(projection.PlannedAmount),
		AccruedAmount:               MoneyAmountToDTO(masked.AccruedAmount),
		ForecastExposure:            MoneyAmountToDTO(masked.ForecastExposure),
		ForecastSourceStatus:        forecastStatus,
		CurrentActualAmount:         DecimalPtrToDTO(projection.CurrentActualAmount),
		FinalActualAmount:           DecimalPtrToDTO(projection.FinalActualAmount),
		BillingRegisterAmount:       DecimalPtrToDTO(projection.BillingRegisterAmount),
		PaidAmount:                  DecimalPtrToDTO(projection.PaidAmount),
		CurrentVarianceAmount:       MoneyAmountToDTO(masked.CurrentVarianceAmount),
		FinalVarianceAmount:         MoneyAmountToDTO(masked.FinalVarianceAmount),
		CurrentVariancePercent:      PercentPtrToDTO(projection.CurrentVariancePercent),
		FinalVariancePercent:        PercentPtrToDTO(projection.FinalVariancePercent),
		BillingReconciliationStatus: &reconciliation,
		CostUpdatedAt:               updatedAt.UTC().Format(time.RFC3339),
		AvailabilityReasons:         reasons,
	}
}

func ToWorkspaceAggregateResponse(
	currencyCode string,
	mixedCurrency bool,
	period WorkspacePeriodDTO,
	planned, accrued, forecast, currentActual, finalActual, currentVar, finalVar *decimal.Decimal,
	mismatchCount int,
) WorkspaceAggregateResponse {
	return WorkspaceAggregateResponse{
		CurrencyCode:  currencyCode,
		Period:        period,
		MixedCurrency: mixedCurrency,
		Kpis: WorkspaceKpisDTO{
			PlannedTotal:                DecimalPtrToDTO(planned),
			AccruedTotal:                DecimalPtrToDTO(accrued),
			ForecastExposureTotal:       DecimalPtrToDTO(forecast),
			PendingProposedAccessorialTotal: nil,
			CurrentActualTotal:          DecimalPtrToDTO(currentActual),
			FinalActualTotal:            DecimalPtrToDTO(finalActual),
			CurrentVarianceTotal:        DecimalPtrToDTO(currentVar),
			FinalVarianceTotal:          DecimalPtrToDTO(finalVar),
			ReconciliationMismatchCount: mismatchCount,
		},
	}
}

func mapReasonToDriver(reason string) (string, *string) {
	switch reason {
	case domain.ReasonAccessorial:
		c := "OTHER"
		return "ACCESSORIAL", &c
	case domain.ReasonFuel:
		c := "FUEL"
		return "FUEL", &c
	case domain.ReasonDetention:
		c := "DETENTION"
		return "DETENTION", &c
	case domain.ReasonWaiting:
		c := "WAITING"
		return "WAITING", &c
	case domain.ReasonLegacyPricing:
		return "PRINCIPAL", nil
	default:
		return "UNATTRIBUTED", nil
	}
}

func MapPricingSourceToPlannedSource(source string) *string {
	switch source {
	case "CONTRACT_RATE":
		v := "CONTRACT_RATE"
		return &v
	case "SPOT_AWARD":
		v := "SPOT_AWARD"
		return &v
	case "MANUAL_OVERRIDE":
		v := "MANUAL_OVERRIDE"
		return &v
	default:
		if source == "" {
			return nil
		}
		v := "UNKNOWN"
		return &v
	}
}

func ToWorkspaceDetailResponse(result service.WorkspaceDetailResult, actor security.TrustedActor) WorkspaceDetailResponse {
	return WorkspaceDetailResponse{
		Summary:                ToWorkspaceSummaryDTO(result.Summary, actor, result.UpdatedAt),
		OrderReference:         SanitizeDisplayLabel(result.OrderReference),
		CarrierName:            SanitizeDisplayLabel(result.CarrierName),
		PlannedSource:          MapPricingSourceToPlannedSource(result.PlannedSource),
		VarianceDrivers:        toVarianceDriverDTOs(result.VarianceDrivers),
		ReconciliationFindings: toReconciliationFindingDTOs(result.ReconciliationFindings),
	}
}

func ToWorkspaceVarianceDetailResponse(result service.WorkspaceVarianceDetailResult) WorkspaceVarianceDetailResponse {
	return WorkspaceVarianceDetailResponse{
		TransportOrderID:       result.TransportOrderID.String(),
		VarianceDrivers:        toVarianceDriverDTOs(result.VarianceDrivers),
		ReconciliationFindings: toReconciliationFindingDTOs(result.ReconciliationFindings),
	}
}

func ToWorkspaceCarrierPerformanceResponse(result service.WorkspaceCarrierPerformanceResult, actor security.TrustedActor) WorkspaceCarrierPerformanceResponse {
	out := make([]WorkspaceCarrierPerformanceRowDTO, 0, len(result.Rows))
	for _, row := range result.Rows {
		item := WorkspaceCarrierPerformanceRowDTO{
			CarrierCompanyID:     row.CarrierCompanyID.String(),
			CarrierName:          SanitizeDisplayLabel(row.CarrierCompanyID.String()),
			OrderCount:           row.OrderCount,
			PlannedTotal:         DecimalPtrToDTO(row.PlannedTotal),
			CurrentActualTotal:   DecimalPtrToDTO(row.CurrentActualTotal),
			FinalActualTotal:     DecimalPtrToDTO(row.FinalActualTotal),
			CurrentVarianceTotal: DecimalPtrToDTO(row.CurrentVarianceTotal),
			CurrencyCode:         row.CurrencyCode,
		}
		if actor.ActorKind == security.ActorKindCarrier {
			item.CurrentVarianceTotal = nil
		}
		out = append(out, item)
	}
	return WorkspaceCarrierPerformanceResponse{Items: out, CurrencyCode: result.Currency}
}

func toVarianceDriverDTOs(drivers []domain.VarianceAttribution) []WorkspaceVarianceDriverDTO {
	out := make([]WorkspaceVarianceDriverDTO, 0, len(drivers))
	for _, driver := range drivers {
		driverType, category := mapReasonToDriver(driver.ReasonCode)
		out = append(out, WorkspaceVarianceDriverDTO{
			DriverType:  driverType,
			Category:    category,
			Amount:      nil,
			Description: driver.ReasonCode,
		})
	}
	return out
}

func toReconciliationFindingDTOs(findings []domain.ReconciliationFinding) []WorkspaceReconciliationFindingDTO {
	out := make([]WorkspaceReconciliationFindingDTO, 0, len(findings))
	for _, finding := range findings {
		out = append(out, WorkspaceReconciliationFindingDTO{
			FindingID:   finding.FindingID.String(),
			FindingType: finding.FindingKind,
			Status:      finding.Status,
			Message:     finding.FindingKind,
		})
	}
	return out
}
