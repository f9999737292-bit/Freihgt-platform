package service

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/provider"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/security"
)

type WorkspaceListResult struct {
	Items  []*domain.CostSummaryProjection
	Total  int
	Limit  int
	Offset int
}

type WorkspaceSummaryResult struct {
	Aggregate *repository.WorkspaceAggregate
	Period    WorkspacePeriod
}

type WorkspacePeriod struct {
	From          string
	To            string
	DateDimension string
}

type WorkspaceDetailResult struct {
	Summary                *domain.CostSummaryProjection
	SummaryView            *domain.CostSummary
	OrderReference         string
	CarrierName            string
	PlannedSource          string
	VarianceDrivers        []domain.VarianceAttribution
	ReconciliationFindings []domain.ReconciliationFinding
	UpdatedAt              time.Time
}

type WorkspaceVarianceDetailResult struct {
	TransportOrderID       uuid.UUID
	VarianceDrivers        []domain.VarianceAttribution
	ReconciliationFindings []domain.ReconciliationFinding
}

type WorkspaceCarrierPerformanceResult struct {
	Rows     []repository.CarrierPerformanceRow
	Currency string
}

type WorkspaceService struct {
	projections *repository.CostSummaryProjectionRepository
	costs       *CostService
	transport   provider.TransportOrderPricingProvider
}

func NewWorkspaceService(
	projections *repository.CostSummaryProjectionRepository,
	costs *CostService,
	transport provider.TransportOrderPricingProvider,
) *WorkspaceService {
	return &WorkspaceService{projections: projections, costs: costs, transport: transport}
}

func (s *WorkspaceService) parseFilter(actor security.TrustedActor, values url.Values) repository.WorkspaceListFilter {
	filter := repository.WorkspaceListFilter{
		TenantID:            actor.TenantID,
		CompanyID:           actor.CompanyID,
		ActorKind:           actor.ActorKind,
		Currency:            strings.TrimSpace(values.Get("currency")),
		ReconciliationState: strings.ToUpper(strings.TrimSpace(values.Get("reconciliation_state"))),
	}
	if raw := strings.TrimSpace(values.Get("carrier_id")); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			filter.CarrierID = &id
		}
	}
	limit := 50
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	offset := 0
	if raw := strings.TrimSpace(values.Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			offset = parsed
		}
	}
	filter.Limit = limit
	filter.Offset = offset
	return filter
}

func (s *WorkspaceService) periodFromQuery(values url.Values) WorkspacePeriod {
	from := strings.TrimSpace(values.Get("from"))
	to := strings.TrimSpace(values.Get("to"))
	dimension := strings.TrimSpace(values.Get("date_dimension"))
	if dimension == "" {
		dimension = "TRANSPORT_ORDER_CREATED_AT"
	}
	return WorkspacePeriod{From: from, To: to, DateDimension: dimension}
}

func (s *WorkspaceService) List(ctx context.Context, actor security.TrustedActor, values url.Values) (WorkspaceListResult, error) {
	filter := s.parseFilter(actor, values)
	items, total, err := s.projections.ListForWorkspace(ctx, filter)
	if err != nil {
		return WorkspaceListResult{}, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return WorkspaceListResult{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: filter.Offset,
	}, nil
}

func (s *WorkspaceService) Summary(ctx context.Context, actor security.TrustedActor, values url.Values) (WorkspaceSummaryResult, error) {
	filter := s.parseFilter(actor, values)
	agg, err := s.projections.AggregateForWorkspace(ctx, filter)
	if err != nil {
		return WorkspaceSummaryResult{}, err
	}
	if actor.ActorKind == security.ActorKindCarrier && agg != nil {
		agg.AccruedTotal = nil
		agg.ForecastExposureTotal = nil
		agg.CurrentVarianceTotal = nil
		agg.FinalVarianceTotal = nil
		agg.ReconciliationMismatchCnt = 0
	}
	return WorkspaceSummaryResult{
		Aggregate: agg,
		Period:    s.periodFromQuery(values),
	}, nil
}

func (s *WorkspaceService) Detail(
	ctx context.Context,
	actor security.TrustedActor,
	transportOrderID uuid.UUID,
) (WorkspaceDetailResult, error) {
	summaryView, err := s.costs.GetCostSummaryByTransportOrder(ctx, actor, transportOrderID)
	if err != nil {
		return WorkspaceDetailResult{}, err
	}
	projection, err := s.projections.GetByTransportOrder(ctx, actor.TenantID, transportOrderID)
	if err != nil {
		projection = domainCostSummaryToProjection(summaryView)
	}
	now := time.Now().UTC()
	result := WorkspaceDetailResult{
		Summary:        projection,
		SummaryView:    summaryView,
		OrderReference: transportOrderID.String(),
		CarrierName:    summaryView.CarrierCompanyID.String(),
		UpdatedAt:      now,
	}
	if actor.ActorKind != security.ActorKindCarrier {
		result.VarianceDrivers = domain.BuildVarianceDrivers(projection, domain.VarianceKindCurrent, domain.DriverAttributionContext{})
		result.ReconciliationFindings = domain.DetectReconciliationFindings(projection)
	}
	if s.transport != nil {
		if snap, err := s.transport.GetRateSnapshot(ctx, actor.TenantID, transportOrderID); err == nil && snap != nil {
			result.PlannedSource = snap.PricingSource
			result.OrderReference = snap.TransportOrderID.String()
			result.CarrierName = snap.CarrierCompanyID.String()
		}
	}
	return result, nil
}

func (s *WorkspaceService) VarianceDetail(
	ctx context.Context,
	actor security.TrustedActor,
	transportOrderID uuid.UUID,
) (WorkspaceVarianceDetailResult, error) {
	if _, err := s.costs.GetCostSummaryByTransportOrder(ctx, actor, transportOrderID); err != nil {
		return WorkspaceVarianceDetailResult{}, err
	}
	if actor.ActorKind == security.ActorKindCarrier {
		return WorkspaceVarianceDetailResult{TransportOrderID: transportOrderID}, nil
	}
	projection, err := s.projections.GetByTransportOrder(ctx, actor.TenantID, transportOrderID)
	if err != nil {
		return WorkspaceVarianceDetailResult{TransportOrderID: transportOrderID}, nil
	}
	return WorkspaceVarianceDetailResult{
		TransportOrderID:       transportOrderID,
		VarianceDrivers:        domain.BuildVarianceDrivers(projection, domain.VarianceKindCurrent, domain.DriverAttributionContext{}),
		ReconciliationFindings: domain.DetectReconciliationFindings(projection),
	}, nil
}

func (s *WorkspaceService) AccessorialSummary(ctx context.Context, actor security.TrustedActor, values url.Values) (string, error) {
	filter := s.parseFilter(actor, values)
	agg, err := s.projections.AggregateForWorkspace(ctx, filter)
	if err != nil {
		return "", err
	}
	if agg == nil {
		return "", nil
	}
	return agg.CurrencyCode, nil
}

func (s *WorkspaceService) CarrierPerformance(ctx context.Context, actor security.TrustedActor, values url.Values) (WorkspaceCarrierPerformanceResult, error) {
	filter := s.parseFilter(actor, values)
	rows, err := s.projections.CarrierPerformance(ctx, filter)
	if err != nil {
		return WorkspaceCarrierPerformanceResult{}, err
	}
	currency := filter.Currency
	for _, row := range rows {
		if currency == "" {
			currency = row.CurrencyCode
		}
	}
	return WorkspaceCarrierPerformanceResult{Rows: rows, Currency: currency}, nil
}

func (s *WorkspaceService) LanePerformanceCurrency(values url.Values) string {
	return strings.TrimSpace(values.Get("currency"))
}

func domainCostSummaryToProjection(summary *domain.CostSummary) *domain.CostSummaryProjection {
	if summary == nil {
		return &domain.CostSummaryProjection{}
	}
	projection := &domain.CostSummaryProjection{
		TenantID:                    summary.TenantID,
		TransportOrderID:            summary.TransportOrderID,
		BuyerCompanyID:              summary.BuyerCompanyID,
		CarrierCompanyID:            summary.CarrierCompanyID,
		CurrencyCode:                summary.CurrencyCode,
		DataStage:                   summary.DataStage,
		FinancialFinality:           summary.FinancialFinality,
		SourcesAvailable:            summary.SourcesAvailable,
		BillingReconciliationStatus: domain.BillingReconciliationUnlinked,
	}
	if summary.BillingReconciliationStatus != nil {
		projection.BillingReconciliationStatus = *summary.BillingReconciliationStatus
	}
	if summary.PlannedAmount != nil {
		amount := summary.PlannedAmount.Amount
		projection.PlannedAmount = &amount
	}
	if summary.AccruedAmount != nil {
		amount := summary.AccruedAmount.Amount
		projection.AccruedAmount = &amount
	}
	if summary.CurrentActualAmount != nil {
		amount := summary.CurrentActualAmount.Amount
		projection.CurrentActualAmount = &amount
	}
	if summary.FinalActualAmount != nil {
		amount := summary.FinalActualAmount.Amount
		projection.FinalActualAmount = &amount
	}
	if summary.CurrentVarianceAmount != nil {
		amount := summary.CurrentVarianceAmount.Amount
		projection.CurrentVarianceAmount = &amount
	}
	if summary.FinalVarianceAmount != nil {
		amount := summary.FinalVarianceAmount.Amount
		projection.FinalVarianceAmount = &amount
	}
	if summary.ForecastExposure != nil {
		amount := summary.ForecastExposure.Amount
		projection.ForecastExposure = &amount
	}
	return projection
}
