package service

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/security"
)

const (
	analyticsDefaultLimit  = 20
	analyticsMaxLimit      = 100
	analyticsDefaultDays   = 90
	analyticsMaxSpanDays   = 731
	analyticsDateDimension = "COST_EFFECTIVE"
)

type AnalyticsPublicQuery struct {
	From             time.Time
	To               time.Time
	DateDimension    string
	Currency         string
	TransportMode    string
	EquipmentType    string
	CarrierCompanyID *uuid.UUID
	LaneKey          string
	Limit            int
	Offset           int
	Sort             string
	SortDesc         bool
}

type AnalyticsPublicService struct {
	analytics  *AnalyticsProjectionService
	orderFacts *repository.AnalyticsOrderFactRepository
	state      *repository.AnalyticsProjectionStateRepository
	enabled    bool
}

func NewAnalyticsPublicService(
	analytics *AnalyticsProjectionService,
	orderFacts *repository.AnalyticsOrderFactRepository,
	state *repository.AnalyticsProjectionStateRepository,
	enabled bool,
) *AnalyticsPublicService {
	return &AnalyticsPublicService{analytics: analytics, orderFacts: orderFacts, state: state, enabled: enabled}
}

func (s *AnalyticsPublicService) ensureBuyer(actor security.TrustedActor) error {
	if actor.ActorKind != security.ActorKindBuyer {
		return apperrors.Forbidden("freight cost buyer analytics access denied")
	}
	return nil
}

func (s *AnalyticsPublicService) ensureEnabled() error {
	if !s.enabled || s.analytics == nil {
		err := apperrors.Unavailable("analytics projections are not available", nil)
		err.Details = map[string]any{"code": "ANALYTICS_NOT_AVAILABLE"}
		return err
	}
	return nil
}

func ParseAnalyticsPublicQuery(values url.Values, sortAllowlist map[string]string) (AnalyticsPublicQuery, error) {
	now := time.Now().UTC()
	to := now
	from := now.AddDate(0, 0, -analyticsDefaultDays)
	if raw := strings.TrimSpace(values.Get("to")); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return AnalyticsPublicQuery{}, apperrors.Validation("invalid to date", map[string]any{"field": "to"})
		}
		to = parsed
	}
	if raw := strings.TrimSpace(values.Get("from")); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return AnalyticsPublicQuery{}, apperrors.Validation("invalid from date", map[string]any{"field": "from"})
		}
		from = parsed
	}
	if from.After(to) {
		return AnalyticsPublicQuery{}, apperrors.Validation("from must be before or equal to to", map[string]any{"field": "from"})
	}
	if to.Sub(from) > analyticsMaxSpanDays*24*time.Hour {
		return AnalyticsPublicQuery{}, apperrors.Validation("date range exceeds maximum span", map[string]any{"field": "to"})
	}
	limit := analyticsDefaultLimit
	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return AnalyticsPublicQuery{}, apperrors.Validation("invalid limit", map[string]any{"field": "limit"})
		}
		limit = parsed
	}
	if limit > analyticsMaxLimit {
		limit = analyticsMaxLimit
	}
	offset := 0
	if raw := strings.TrimSpace(values.Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return AnalyticsPublicQuery{}, apperrors.Validation("invalid offset", map[string]any{"field": "offset"})
		}
		offset = parsed
	}
	sortField := strings.TrimSpace(values.Get("sort"))
	sortDesc := false
	if strings.HasPrefix(sortField, "-") {
		sortDesc = true
		sortField = strings.TrimPrefix(sortField, "-")
	}
	if sortField != "" && sortAllowlist != nil {
		if _, ok := sortAllowlist[sortField]; !ok {
			return AnalyticsPublicQuery{}, apperrors.Validation("unsupported sort field", map[string]any{"field": "sort"})
		}
	}
	query := AnalyticsPublicQuery{
		From: from, To: to,
		DateDimension: coalesceDimension(values.Get("date_dimension")),
		Currency:      strings.ToUpper(strings.TrimSpace(values.Get("currency"))),
		TransportMode: strings.ToUpper(strings.TrimSpace(values.Get("transport_mode"))),
		EquipmentType: strings.ToUpper(strings.TrimSpace(values.Get("equipment_type"))),
		LaneKey:       strings.TrimSpace(values.Get("lane_key")),
		Limit:         limit, Offset: offset, Sort: sortField, SortDesc: sortDesc,
	}
	if raw := strings.TrimSpace(values.Get("carrier_company_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return AnalyticsPublicQuery{}, apperrors.Validation("invalid carrier_company_id", map[string]any{"field": "carrier_company_id"})
		}
		query.CarrierCompanyID = &id
	}
	return query, nil
}

func coalesceDimension(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return analyticsDateDimension
	}
	return strings.TrimSpace(raw)
}

func (s *AnalyticsPublicService) Overview(ctx context.Context, actor security.TrustedActor, values url.Values) (AnalyticsPublicOverviewResult, error) {
	if err := s.ensureEnabled(); err != nil {
		return AnalyticsPublicOverviewResult{}, err
	}
	if err := s.ensureBuyer(actor); err != nil {
		return AnalyticsPublicOverviewResult{}, err
	}
	query, err := ParseAnalyticsPublicQuery(values, nil)
	if err != nil {
		return AnalyticsPublicOverviewResult{}, err
	}
	periods, err := s.analytics.ListPeriodProjections(ctx, actor.TenantID, s.periodFilter(actor, query))
	if err != nil {
		return AnalyticsPublicOverviewResult{}, err
	}
	quality, mixed, currency := resolveCurrencyScope(extractPeriodCurrencies(periods), query.Currency)
	result := AnalyticsPublicOverviewResult{
		CurrencyCode: currency, Period: s.periodResult(query), DataQuality: quality, MixedCurrency: mixed,
		Freshness: s.loadFreshness(ctx, actor.TenantID),
	}
	if mixed {
		return result, nil
	}
	var planned, current, final, curVar, finVar decimal.Decimal
	orderCount, reconCount := 0, 0
	for _, p := range periods {
		if query.Currency != "" && p.CurrencyCode != query.Currency {
			continue
		}
		orderCount += p.OrderCount
		reconCount += p.ReconciliationOpenCount
		planned = planned.Add(decimalOrZero(p.PlannedTotal))
		current = current.Add(decimalOrZero(p.CurrentActualTotal))
		final = final.Add(decimalOrZero(p.FinalActualTotal))
		curVar = curVar.Add(decimalOrZero(p.CurrentVarianceTotal))
		finVar = finVar.Add(decimalOrZero(p.FinalVarianceTotal))
	}
	if currency != "" {
		result.Summary = &AnalyticsPublicOverviewSummary{
			PlannedTotal: planned, CurrentActualTotal: current, FinalActualTotal: final,
			CurrentVarianceTotal: curVar, FinalVarianceTotal: finVar,
			ReconciliationMismatchCount: reconCount, OrderCount: orderCount,
		}
	}
	laneItems, _ := s.analytics.ListLaneProjections(ctx, actor.TenantID, s.laneFilter(actor, query, 100, 0))
	sort.Slice(laneItems, func(i, j int) bool {
		return decimalOrZero(laneItems[i].CurrentActualTotal).GreaterThan(decimalOrZero(laneItems[j].CurrentActualTotal))
	})
	labels, _ := s.orderFacts.ListLaneLabels(ctx, actor.TenantID, actor.CompanyID)
	for i, lane := range laneItems {
		if i >= 5 {
			break
		}
		spend := decimalOrZero(lane.CurrentActualTotal)
		if lane.FinalActualTotal != nil {
			spend = *lane.FinalActualTotal
		}
		result.TopLanes = append(result.TopLanes, AnalyticsPublicOverviewTopLane{
			LaneKey: lane.LaneKey, LaneLabel: laneLabel(lane.LaneKey, labels[lane.LaneKey]),
			OrderCount: lane.OrderCount, SpendTotal: spend, Currency: lane.CurrencyCode,
		})
	}
	accessorials, _ := s.analytics.ListAccessorialProjections(ctx, actor.TenantID, s.accessorialFilter(actor, query, 500, 0))
	var accTotal decimal.Decimal
	accOrders := 0
	for _, row := range accessorials {
		accTotal = accTotal.Add(decimalOrZero(row.TotalAmount))
		accOrders += row.OrderCount
	}
	if len(accessorials) > 0 && currency != "" {
		result.Accessorial = &AnalyticsPublicOverviewAccessorial{TotalAmount: accTotal, OrderCount: accOrders, Currency: currency}
	}
	opps, _ := s.analytics.ListOpportunityProjections(ctx, actor.TenantID, s.opportunityFilter(actor, query, 3, 0))
	allOpps, _ := s.analytics.ListOpportunityProjections(ctx, actor.TenantID, s.opportunityFilter(actor, query, 5000, 0))
	result.Opportunities = opps
	result.OpportunityCount = len(allOpps)
	return result, nil
}

func (s *AnalyticsPublicService) ListLanes(ctx context.Context, actor security.TrustedActor, values url.Values) (AnalyticsPublicListResult, error) {
	return s.listProjections(ctx, actor, values, map[string]string{
		"spend_total": "spend_total", "order_count": "order_count", "variance_total": "variance_total", "lane_label": "lane_label",
	}, "lanes")
}

func (s *AnalyticsPublicService) ListCarriers(ctx context.Context, actor security.TrustedActor, values url.Values) (AnalyticsPublicListResult, error) {
	return s.listProjections(ctx, actor, values, map[string]string{"spend_total": "spend_total", "order_count": "order_count"}, "carriers")
}

func (s *AnalyticsPublicService) ListAccessorials(ctx context.Context, actor security.TrustedActor, values url.Values) (AnalyticsPublicListResult, error) {
	return s.listProjections(ctx, actor, values, map[string]string{
		"total_amount": "total_amount", "order_count": "order_count", "share_of_spend": "share_of_spend", "normalized_category": "normalized_category",
	}, "accessorials")
}

func (s *AnalyticsPublicService) ListOpportunities(ctx context.Context, actor security.TrustedActor, values url.Values) (AnalyticsPublicListResult, error) {
	return s.listProjections(ctx, actor, values, map[string]string{
		"estimated_delta": "estimated_delta", "calculated_at": "calculated_at", "type": "type",
	}, "opportunities")
}

func (s *AnalyticsPublicService) listProjections(
	ctx context.Context,
	actor security.TrustedActor,
	values url.Values,
	sortAllowlist map[string]string,
	kind string,
) (AnalyticsPublicListResult, error) {
	if err := s.ensureEnabled(); err != nil {
		return AnalyticsPublicListResult{}, err
	}
	if err := s.ensureBuyer(actor); err != nil {
		return AnalyticsPublicListResult{}, err
	}
	query, err := ParseAnalyticsPublicQuery(values, sortAllowlist)
	if err != nil {
		return AnalyticsPublicListResult{}, err
	}
	result := AnalyticsPublicListResult{
		Period: s.periodResult(query), Freshness: s.loadFreshness(ctx, actor.TenantID),
		Limit: query.Limit, Offset: query.Offset, DataQuality: domain.DataQualityAvailable,
	}
	switch kind {
	case "lanes":
		rows, err := s.analytics.ListLaneProjections(ctx, actor.TenantID, s.laneFilter(actor, query, query.Limit, query.Offset))
		if err != nil {
			return AnalyticsPublicListResult{}, err
		}
		allRows, _ := s.analytics.ListLaneProjections(ctx, actor.TenantID, s.laneFilter(actor, query, 5000, 0))
		result.DataQuality, result.MixedCurrency, result.CurrencyCode = laneCurrencyScope(allRows, query.Currency)
		labels, _ := s.orderFacts.ListLaneLabels(ctx, actor.TenantID, actor.CompanyID)
		benchmarks, _ := s.analytics.ListBenchmarkProjections(ctx, actor.TenantID, repository.AnalyticsBenchmarkListFilter{
			BuyerCompanyID: &actor.CompanyID, CurrencyCode: query.Currency, PeriodFrom: &query.From, PeriodTo: &query.To, Limit: 5000,
		})
		benchMap := map[string]domain.AnalyticsBenchmarkProjection{}
		for _, b := range benchmarks {
			benchMap[benchmarkRowKey(b)] = b
		}
		for _, row := range rows {
			if result.MixedCurrency {
				continue
			}
			var bench *AnalyticsPublicBenchmark
			if b, ok := benchMap[benchmarkRowKeyFromLane(row)]; ok {
				bench = toPublicBenchmark(&b)
			}
			result.Lanes = append(result.Lanes, AnalyticsPublicLaneItem{
				Projection: row, LaneLabel: laneLabel(row.LaneKey, labels[row.LaneKey]), Benchmark: bench,
			})
		}
		result.Total = len(allRows)
	case "carriers":
		rows, err := s.analytics.ListCarrierProjections(ctx, actor.TenantID, s.carrierFilter(actor, query, query.Limit, query.Offset))
		if err != nil {
			return AnalyticsPublicListResult{}, err
		}
		allRows, _ := s.analytics.ListCarrierProjections(ctx, actor.TenantID, s.carrierFilter(actor, query, 5000, 0))
		result.DataQuality, result.MixedCurrency, result.CurrencyCode = carrierCurrencyScope(allRows, query.Currency)
		names, _ := s.orderFacts.ListCarrierDisplayNames(ctx, actor.TenantID, actor.CompanyID)
		opps, _ := s.analytics.ListOpportunityProjections(ctx, actor.TenantID, repository.AnalyticsOpportunityListFilter{
			BuyerCompanyID: &actor.CompanyID, OpportunityType: domain.OpportunityTypeCarrierCostOutlier,
			CurrencyCode: query.Currency, PeriodFrom: &query.From, PeriodTo: &query.To, Limit: 5000,
		})
		deltaByCarrier, countByCarrier := map[string]decimal.Decimal{}, map[string]int{}
		for _, opp := range opps {
			if opp.CarrierCompanyID == nil {
				continue
			}
			key := opp.CarrierCompanyID.String()
			deltaByCarrier[key] = deltaByCarrier[key].Add(opp.EstimatedDelta)
			countByCarrier[key] = opp.SampleSize
		}
		for _, row := range rows {
			if result.MixedCurrency {
				continue
			}
			cid := row.CarrierCompanyID.String()
			result.Carriers = append(result.Carriers, AnalyticsPublicCarrierItem{
				Projection: row, CarrierName: names[row.CarrierCompanyID],
				ComparableOrderCount: countByCarrier[cid], LaneNormalizedDelta: deltaByCarrier[cid], DataQuality: result.DataQuality,
			})
		}
		result.Total = len(allRows)
	case "accessorials":
		rows, err := s.analytics.ListAccessorialProjections(ctx, actor.TenantID, s.accessorialFilter(actor, query, query.Limit, query.Offset))
		if err != nil {
			return AnalyticsPublicListResult{}, err
		}
		allRows, _ := s.analytics.ListAccessorialProjections(ctx, actor.TenantID, s.accessorialFilter(actor, query, 5000, 0))
		result.DataQuality, result.MixedCurrency, result.CurrencyCode = accessorialCurrencyScope(allRows, query.Currency)
		for _, row := range rows {
			if result.MixedCurrency {
				continue
			}
			result.Accessorials = append(result.Accessorials, AnalyticsPublicAccessorialItem{Projection: row})
		}
		result.Total = len(allRows)
	case "opportunities":
		rows, err := s.analytics.ListOpportunityProjections(ctx, actor.TenantID, s.opportunityFilter(actor, query, query.Limit, query.Offset))
		if err != nil {
			return AnalyticsPublicListResult{}, err
		}
		allRows, _ := s.analytics.ListOpportunityProjections(ctx, actor.TenantID, s.opportunityFilter(actor, query, 5000, 0))
		result.Opportunities = rows
		result.Total = len(allRows)
		if query.Currency == "" && len(allRows) > 0 {
			result.CurrencyCode = allRows[0].CurrencyCode
		} else {
			result.CurrencyCode = query.Currency
		}
	}
	return result, nil
}

func (s *AnalyticsPublicService) periodResult(q AnalyticsPublicQuery) AnalyticsPublicPeriod {
	return AnalyticsPublicPeriod{From: q.From.Format("2006-01-02"), To: q.To.Format("2006-01-02"), DateDimension: q.DateDimension}
}

func (s *AnalyticsPublicService) loadFreshness(ctx context.Context, tenantID uuid.UUID) AnalyticsPublicFreshness {
	freshness := AnalyticsPublicFreshness{ProjectionVersion: domain.AnalyticsProjectionVersion, BenchmarkCostBasis: analyticsPublicBenchmarkCostBasis}
	if s.state == nil {
		return freshness
	}
	state, err := s.state.Get(ctx, nil, domain.AnalyticsProjectionNamePeriod, tenantID)
	if err != nil || state == nil {
		return freshness
	}
	freshness.ProjectionVersion = state.ProjectionVersion
	freshness.CalculatedAt = state.CalculatedAt
	freshness.DataThrough = state.DataThrough
	return freshness
}

func (s *AnalyticsPublicService) periodFilter(actor security.TrustedActor, q AnalyticsPublicQuery) repository.AnalyticsPeriodListFilter {
	return repository.AnalyticsPeriodListFilter{BuyerCompanyID: &actor.CompanyID, CurrencyCode: q.Currency, PeriodFrom: &q.From, PeriodTo: &q.To, Limit: 5000}
}

func (s *AnalyticsPublicService) laneFilter(actor security.TrustedActor, q AnalyticsPublicQuery, limit, offset int) repository.AnalyticsLaneListFilter {
	return repository.AnalyticsLaneListFilter{
		BuyerCompanyID: &actor.CompanyID, CurrencyCode: q.Currency, LaneKey: q.LaneKey,
		TransportMode: q.TransportMode, EquipmentType: q.EquipmentType, PeriodFrom: &q.From, PeriodTo: &q.To, Limit: limit, Offset: offset,
	}
}

func (s *AnalyticsPublicService) carrierFilter(actor security.TrustedActor, q AnalyticsPublicQuery, limit, offset int) repository.AnalyticsCarrierListFilter {
	filter := repository.AnalyticsCarrierListFilter{
		BuyerCompanyID: &actor.CompanyID, CurrencyCode: q.Currency, PeriodFrom: &q.From, PeriodTo: &q.To, Limit: limit, Offset: offset,
	}
	if q.CarrierCompanyID != nil {
		filter.CarrierCompanyID = q.CarrierCompanyID
	}
	return filter
}

func (s *AnalyticsPublicService) accessorialFilter(actor security.TrustedActor, q AnalyticsPublicQuery, limit, offset int) repository.AnalyticsAccessorialListFilter {
	return repository.AnalyticsAccessorialListFilter{
		BuyerCompanyID: &actor.CompanyID, CurrencyCode: q.Currency, PeriodFrom: &q.From, PeriodTo: &q.To, Limit: limit, Offset: offset,
	}
}

func (s *AnalyticsPublicService) opportunityFilter(actor security.TrustedActor, q AnalyticsPublicQuery, limit, offset int) repository.AnalyticsOpportunityListFilter {
	return repository.AnalyticsOpportunityListFilter{
		BuyerCompanyID: &actor.CompanyID, CurrencyCode: q.Currency, PeriodFrom: &q.From, PeriodTo: &q.To, Limit: limit, Offset: offset,
	}
}

func extractPeriodCurrencies(periods []domain.AnalyticsPeriodProjection) []string {
	set := map[string]struct{}{}
	for _, p := range periods {
		set[p.CurrencyCode] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	return out
}

func resolveCurrencyScope(currencies []string, filter string) (quality string, mixed bool, out string) {
	if filter == "" && len(currencies) > 1 {
		return domain.DataQualityMixedCurrency, true, ""
	}
	if filter != "" {
		return domain.DataQualityAvailable, false, filter
	}
	if len(currencies) == 1 {
		return domain.DataQualityAvailable, false, currencies[0]
	}
	return domain.DataQualityAvailable, false, ""
}

func laneCurrencyScope(rows []domain.AnalyticsLanePeriodProjection, filter string) (string, bool, string) {
	set := map[string]struct{}{}
	for _, r := range rows {
		set[r.CurrencyCode] = struct{}{}
	}
	currencies := make([]string, 0, len(set))
	for c := range set {
		currencies = append(currencies, c)
	}
	return resolveCurrencyScope(currencies, filter)
}

func carrierCurrencyScope(rows []domain.AnalyticsCarrierPeriodProjection, filter string) (string, bool, string) {
	set := map[string]struct{}{}
	for _, r := range rows {
		set[r.CurrencyCode] = struct{}{}
	}
	currencies := make([]string, 0, len(set))
	for c := range set {
		currencies = append(currencies, c)
	}
	return resolveCurrencyScope(currencies, filter)
}

func accessorialCurrencyScope(rows []domain.AnalyticsAccessorialPeriodProjection, filter string) (string, bool, string) {
	set := map[string]struct{}{}
	for _, r := range rows {
		set[r.CurrencyCode] = struct{}{}
	}
	currencies := make([]string, 0, len(set))
	for c := range set {
		currencies = append(currencies, c)
	}
	return resolveCurrencyScope(currencies, filter)
}

func laneLabel(laneKey string, snapshot *string) string {
	if snapshot != nil && strings.TrimSpace(*snapshot) != "" {
		return strings.TrimSpace(*snapshot)
	}
	_, originCity, _, destCity, mode, equipment, ok := domain.ParseLaneKeyComponents(laneKey)
	if !ok {
		return laneKey
	}
	return domain.BuildLaneLabel(originCity, destCity, mode, equipment)
}

func toPublicBenchmark(b *domain.AnalyticsBenchmarkProjection) *AnalyticsPublicBenchmark {
	return &AnalyticsPublicBenchmark{
		SampleSize: b.SampleCount, Mean: b.MeanAmount, Median: b.MedianAmount, P25: b.P25Amount, P75: b.P75Amount,
		P90: b.P90Amount, Min: b.MinAmount, Max: b.MaxAmount, DataQuality: b.DataQuality, Currency: b.CurrencyCode,
	}
}

func benchmarkRowKey(b domain.AnalyticsBenchmarkProjection) string {
	return b.LaneKey + "|" + b.PeriodStart.Format("2006-01-02") + "|" + b.CurrencyCode
}

func benchmarkRowKeyFromLane(l domain.AnalyticsLanePeriodProjection) string {
	return l.LaneKey + "|" + l.PeriodStart.Format("2006-01-02") + "|" + l.CurrencyCode
}

func decimalOrZero(d *decimal.Decimal) decimal.Decimal {
	if d == nil {
		return decimal.Zero
	}
	return *d
}
