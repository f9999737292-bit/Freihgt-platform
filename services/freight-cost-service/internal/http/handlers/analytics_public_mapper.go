package handlers

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	"github.com/freight-platform/freight-cost-service/internal/http/dto"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

func toOverviewResponse(result service.AnalyticsPublicOverviewResult) dto.AnalyticsOverviewResponse {
	resp := dto.AnalyticsOverviewResponse{
		CurrencyCode:  result.CurrencyCode,
		Period:        toPeriodDTO(result.Period),
		DataQuality:   result.DataQuality,
		MixedCurrency: result.MixedCurrency,
		Freshness:     toFreshnessDTO(result.Freshness),
	}
	if result.Summary != nil {
		resp.Summary = &dto.AnalyticsOverviewSummaryDTO{
			PlannedTotal:                moneyStringPtr(result.Summary.PlannedTotal),
			CurrentActualTotal:          moneyStringPtr(result.Summary.CurrentActualTotal),
			FinalActualTotal:            moneyStringPtr(result.Summary.FinalActualTotal),
			CurrentVarianceTotal:        moneyStringPtr(result.Summary.CurrentVarianceTotal),
			FinalVarianceTotal:          moneyStringPtr(result.Summary.FinalVarianceTotal),
			ReconciliationMismatchCount: result.Summary.ReconciliationMismatchCount,
			OrderCount:                  result.Summary.OrderCount,
		}
	}
	for _, lane := range result.TopLanes {
		resp.TopLanes = append(resp.TopLanes, dto.AnalyticsOverviewTopLaneDTO{
			LaneKey: lane.LaneKey, LaneLabel: lane.LaneLabel, OrderCount: lane.OrderCount,
			SpendTotal: dto.MoneyValue(lane.SpendTotal, lane.Currency),
		})
	}
	if result.Accessorial != nil {
		resp.Accessorial = &dto.AnalyticsOverviewAccessorialDTO{
			TotalAmount: dto.MoneyValue(result.Accessorial.TotalAmount, result.Accessorial.Currency),
			OrderCount:  result.Accessorial.OrderCount,
		}
	}
	if len(result.Opportunities) > 0 || result.OpportunityCount > 0 {
		items := make([]dto.AnalyticsOpportunityItemDTO, 0, len(result.Opportunities))
		for _, opp := range result.Opportunities {
			items = append(items, dto.ToOpportunityItem(opp))
		}
		resp.Opportunities = &dto.AnalyticsOverviewOpportunitySummaryDTO{Count: result.OpportunityCount, TopItems: items}
	}
	return resp
}

func toListEnvelope(result service.AnalyticsPublicListResult) dto.AnalyticsListEnvelope {
	env := dto.AnalyticsListEnvelope{
		CurrencyCode:  result.CurrencyCode,
		Period:        toPeriodDTO(result.Period),
		DataQuality:   result.DataQuality,
		MixedCurrency: result.MixedCurrency,
		Freshness:     toFreshnessDTO(result.Freshness),
		Total:         result.Total,
		Limit:         result.Limit,
		Offset:        result.Offset,
	}
	switch {
	case len(result.Lanes) > 0:
		items := make([]dto.AnalyticsLaneItemDTO, 0, len(result.Lanes))
		for _, row := range result.Lanes {
			p := row.Projection
			variance := decimalOrZero(p.FinalVarianceTotal)
			if p.CurrentVarianceTotal != nil {
				variance = *p.CurrentVarianceTotal
			}
			originCountry, originCity, destCountry, destCity, _, _, _ := domain.ParseLaneKeyComponents(p.LaneKey)
			benchDTO := dto.AnalyticsBenchmarkDTO{DataQuality: domain.DataQualityNotAvailable}
			if row.Benchmark != nil {
				benchDTO = toBenchmarkDTO(row.Benchmark)
			}
			items = append(items, dto.AnalyticsLaneItemDTO{
				LaneKey: p.LaneKey, LaneLabel: row.LaneLabel,
				OriginCountry: originCountry, OriginCity: originCity, DestinationCountry: destCountry, DestinationCity: destCity,
				TransportMode: p.TransportMode, EquipmentType: p.EquipmentType,
				OrderCount: p.OrderCount, CarrierCount: p.CarrierCount,
				PlannedTotal: dto.MoneyPtr(p.PlannedTotal, p.CurrencyCode),
				CurrentActualTotal: dto.MoneyPtr(p.CurrentActualTotal, p.CurrencyCode),
				FinalActualTotal: dto.MoneyPtr(p.FinalActualTotal, p.CurrencyCode),
				VarianceTotal: dto.MoneyValue(variance, p.CurrencyCode),
				Benchmark: benchDTO,
			})
		}
		env.Items = items
	case len(result.Carriers) > 0:
		items := make([]dto.AnalyticsCarrierItemDTO, 0, len(result.Carriers))
		for _, row := range result.Carriers {
			p := row.Projection
			items = append(items, dto.AnalyticsCarrierItemDTO{
				CarrierCompanyID: p.CarrierCompanyID.String(), CarrierName: row.CarrierName,
				OrderCount: p.OrderCount, LaneCount: p.LaneCount,
				PlannedTotal: dto.MoneyPtr(p.PlannedTotal, p.CurrencyCode),
				CurrentActualTotal: dto.MoneyPtr(p.CurrentActualTotal, p.CurrencyCode),
				FinalActualTotal: dto.MoneyPtr(p.FinalActualTotal, p.CurrencyCode),
				VarianceTotal: dto.MoneyValue(decimalOrZero(p.FinalVarianceTotal), p.CurrencyCode),
				ComparableOrderCount: row.ComparableOrderCount,
				LaneNormalizedDelta: dto.MoneyValue(row.LaneNormalizedDelta, p.CurrencyCode),
				DataQuality: row.DataQuality,
			})
		}
		env.Items = items
	case len(result.Accessorials) > 0:
		items := make([]dto.AnalyticsAccessorialItemDTO, 0, len(result.Accessorials))
		for _, row := range result.Accessorials {
			p := row.Projection
			items = append(items, dto.AnalyticsAccessorialItemDTO{
				NormalizedCategory: p.NormalizedCategory,
				TotalAmount:        dto.MoneyPtr(p.TotalAmount, p.CurrencyCode),
				OrderCount:         p.OrderCount,
				LineCount:          p.LineCount,
				ShareOfSpend:       dto.RatioString(p.ShareOfSpend),
				AccessorialRate:    dto.RatioString(p.AccessorialOrderRate),
				DataQuality:        domain.DataQualityAvailable,
			})
		}
		env.Items = items
	default:
		items := make([]dto.AnalyticsOpportunityItemDTO, 0, len(result.Opportunities))
		for _, opp := range result.Opportunities {
			items = append(items, dto.ToOpportunityItem(opp))
		}
		env.Items = items
	}
	return env
}

func toPeriodDTO(period service.AnalyticsPublicPeriod) dto.AnalyticsPeriodDTO {
	return dto.AnalyticsPeriodDTO{From: period.From, To: period.To, DateDimension: period.DateDimension}
}

func toFreshnessDTO(f service.AnalyticsPublicFreshness) dto.AnalyticsFreshnessDTO {
	return dto.AnalyticsFreshnessDTO{
		CalculatedAt:       timePtrRFC3339(f.CalculatedAt),
		DataThrough:        timePtrRFC3339(f.DataThrough),
		ProjectionVersion:  f.ProjectionVersion,
		BenchmarkCostBasis: f.BenchmarkCostBasis,
	}
}

func toBenchmarkDTO(b *service.AnalyticsPublicBenchmark) dto.AnalyticsBenchmarkDTO {
	return dto.AnalyticsBenchmarkDTO{
		SampleSize: b.SampleSize,
		Mean: dto.MoneyPtr(b.Mean, b.Currency), Median: dto.MoneyPtr(b.Median, b.Currency),
		P25: dto.MoneyPtr(b.P25, b.Currency), P75: dto.MoneyPtr(b.P75, b.Currency), P90: dto.MoneyPtr(b.P90, b.Currency),
		Min: dto.MoneyPtr(b.Min, b.Currency), Max: dto.MoneyPtr(b.Max, b.Currency),
		DataQuality: b.DataQuality,
	}
}

func moneyStringPtr(d decimal.Decimal) *string {
	s := d.Round(domain.MoneyScale).StringFixed(domain.MoneyScale)
	return &s
}

func timePtrRFC3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func decimalOrZero(d *decimal.Decimal) decimal.Decimal {
	if d == nil {
		return decimal.Zero
	}
	return *d
}
