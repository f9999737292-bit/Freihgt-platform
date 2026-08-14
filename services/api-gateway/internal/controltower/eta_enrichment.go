package controltower

import (
	"context"

	"github.com/freight-platform/api-gateway/internal/controltower/risk"
	"github.com/freight-platform/api-gateway/internal/tracking"
)

func (s *Service) enrichShipmentsWithETA(ctx context.Context, reqCtx RequestContext, rows []ControlTowerShipment) {
	if s.trackingClient == nil || len(rows) == 0 {
		return
	}
	ids := make([]string, 0, len(rows))
	index := make(map[string]int, len(rows))
	planned := make(map[string]tracking.ETALookupPlanned, len(rows))
	for i, row := range rows {
		ids = append(ids, row.ID)
		index[row.ID] = i
		p := tracking.ETALookupPlanned{ShipmentStatus: row.Status}
		p.PlannedPickupAt = tracking.FormatTimePtr(row.PlannedPickupAt)
		p.PlannedDeliveryAt = tracking.FormatTimePtr(row.PlannedDeliveryAt)
		p.ActualDeliveryAt = tracking.FormatTimePtr(row.ActualDeliveryAt)
		p.ActualPickupAt = tracking.FormatTimePtr(row.ActualPickupAt)
		planned[row.ID] = p
	}
	states, err := s.trackingClient.LookupDeliveryETA(ctx, reqCtx.TenantID, reqCtx.RequestID, ids, planned)
	if err != nil {
		return
	}
	for id, summary := range states {
		i, ok := index[id]
		if !ok {
			continue
		}
		applyETASummary(&rows[i], summary)
	}
}

func applyETASummary(row *ControlTowerShipment, summary tracking.ETATargetSummary) {
	status := summary.Status
	row.ETAStatus = &status
	row.ETAFreshness = &summary.FreshnessStatus
	quality := summary.QualityStatus
	row.ETAQuality = &quality
	row.EstimatedDeliveryAt = summary.EstimatedArrivalAt
	row.ProjectedDelaySeconds = summary.ProjectedDeviationSeconds
	projection := summary.ArrivalProjection
	row.ArrivalProjection = &projection
	if summary.AgeSeconds != nil {
		row.ETAAgeSeconds = summary.AgeSeconds
	}
	if summary.SourceObservedAt != nil {
		row.LastETAObservedAt = summary.SourceObservedAt
	}
}

func etaContextFromShipment(row ControlTowerShipment) *risk.ETAContext {
	if row.ETAStatus == nil {
		return nil
	}
	status := *row.ETAStatus
	usable := status == risk.ETAStatusAvailable || status == risk.ETAStatusStale
	ctx := &risk.ETAContext{
		HasUsableETA:              usable && row.EstimatedDeliveryAt != nil,
		Status:                    status,
		FreshnessStatus:           "",
		QualityStatus:             "",
		ArrivalProjection:         "",
		ProjectedDeviationSeconds: row.ProjectedDelaySeconds,
		EstimatedArrivalAt:        row.EstimatedDeliveryAt,
		AgeSeconds:                row.ETAAgeSeconds,
	}
	if row.ETAFreshness != nil {
		ctx.FreshnessStatus = *row.ETAFreshness
	}
	if row.ETAQuality != nil {
		ctx.QualityStatus = *row.ETAQuality
	}
	if row.ArrivalProjection != nil {
		ctx.ArrivalProjection = *row.ArrivalProjection
	}
	if status == risk.ETAStatusExpired || status == risk.ETAStatusUnavailable {
		ctx.HasUsableETA = false
	}
	if ctx.QualityStatus == "poor" {
		ctx.HasUsableETA = false
	}
	return ctx
}
