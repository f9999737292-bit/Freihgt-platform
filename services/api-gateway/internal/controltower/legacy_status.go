package controltower

import (
	"context"
	"log/slog"

	"github.com/freight-platform/api-gateway/internal/controltower/legacyaggregate"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

func (s *Service) resolveLegacyStatusInput(
	ctx context.Context,
	reqCtx RequestContext,
	shipments []rawShipment,
	shipmentsTotal int,
) controltowerreadmodel.LegacyStatusInput {
	pageTotal, pageCounted, pageByStatus, pageLimited := BuildLegacyStatusSummary(shipments, shipmentsTotal)

	if s.legacyAggregate == nil {
		return controltowerreadmodel.LegacyStatusInput{
			TotalShipments:         pageTotal,
			CountedShipments:       pageCounted,
			ByStatus:               pageByStatus,
			LimitedDataset:         pageLimited,
			FullAggregateAvailable: false,
		}
	}

	mode := string(s.readModelCfg.Mode)
	aggCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.legacyAggregateTimeout)
	defer cancel()

	summary, depErr := s.legacyAggregate.FetchStatusSummary(aggCtx, mode, reqCtx.TenantID, reqCtx.RequestID)
	if depErr != nil {
		s.legacyMetrics.ObserveError("ERROR", string(depErr.Reason))
		fallbackReason := legacyaggregate.FallbackReasonFullLegacyUnavailable
		incomplete := depErr.Reason == legacyaggregate.ReasonIncomplete
		if incomplete {
			fallbackReason = legacyaggregate.FallbackReasonFullLegacyIncomplete
		}
		if s.legacyLog != nil {
			attrs := []any{
				slog.String("mode", mode),
				slog.String("request_id", reqCtx.RequestID),
				slog.String("reason", string(depErr.Reason)),
				slog.Int64("total_shipments", pageTotal),
				slog.Int64("counted_shipments", pageCounted),
			}
			if incomplete {
				s.legacyLog.Info("control_tower_legacy_aggregate_incomplete", attrs...)
			} else {
				s.legacyLog.Info("control_tower_legacy_aggregate_unavailable", attrs...)
			}
		}
		s.legacyMetrics.ObserveFallback(mode, legacyaggregate.FallbackLevelPageLimited, fallbackReason)
		return controltowerreadmodel.LegacyStatusInput{
			TotalShipments:          pageTotal,
			CountedShipments:        pageCounted,
			ByStatus:                pageByStatus,
			LimitedDataset:          pageLimited,
			FullAggregateAvailable:  false,
			FullAggregateIncomplete: incomplete,
		}
	}

	s.legacyMetrics.ObserveFallback(mode, legacyaggregate.FallbackLevelFullAggregate, "NONE")
	return controltowerreadmodel.LegacyStatusInput{
		TotalShipments:         summary.TotalShipments,
		CountedShipments:       summary.CountedShipments,
		ByStatus:               summary.ByStatus,
		LimitedDataset:         false,
		FullAggregateAvailable: true,
	}
}
