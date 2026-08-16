package controltower

import (
	"context"

	"github.com/freight-platform/api-gateway/internal/controltower/risk"
	"github.com/freight-platform/api-gateway/internal/tracking"
)

func (s *Service) enrichShipmentsWithTracking(ctx context.Context, reqCtx RequestContext, rows []ControlTowerShipment) {
	if s.trackingClient == nil || len(rows) == 0 {
		return
	}
	ids := make([]string, 0, len(rows))
	index := make(map[string]int, len(rows))
	for i, row := range rows {
		ids = append(ids, row.ID)
		index[row.ID] = i
	}
	states, err := s.trackingClient.LookupStates(ctx, reqCtx.TenantID, reqCtx.RequestID, ids)
	if err != nil {
		return
	}
	for id, summary := range states {
		i, ok := index[id]
		if !ok {
			continue
		}
		applyTrackingSummary(&rows[i], summary)
	}
}

func applyTrackingSummary(row *ControlTowerShipment, summary tracking.Summary) {
	status := summary.TrackingStatus
	row.TrackingStatus = &status
	freshness := summary.Freshness.Status
	row.TrackingFreshness = &freshness
	quality := summary.Quality.Status
	row.TrackingQuality = &quality
	if summary.Freshness.AgeSeconds != nil {
		row.TelemetryAgeSeconds = summary.Freshness.AgeSeconds
	}
	if summary.LastRecordedAt != nil {
		row.LastPositionRecordedAt = summary.LastRecordedAt
	}
	if summary.Provider != nil {
		row.TrackingProvider = summary.Provider
	}
	if summary.LastKnownPosition != nil {
		lat := summary.LastKnownPosition.Latitude
		lon := summary.LastKnownPosition.Longitude
		row.LastKnownLatitude = &lat
		row.LastKnownLongitude = &lon
	}
}

func telemetryContextFromShipment(row ControlTowerShipment) *risk.TelemetryContext {
	if row.TrackingStatus == nil {
		return nil
	}
	status := *row.TrackingStatus
	hasBinding := status != risk.TrackingStatusNotConfigured
	ctx := &risk.TelemetryContext{
		HasBinding:       hasBinding,
		TrackingStatus:   status,
		LastRecordedAt:   row.LastPositionRecordedAt,
		FreshnessStatus:  "",
		QualityStatus:    "",
		TelemetryAgeSecs: row.TelemetryAgeSeconds,
	}
	if row.TrackingFreshness != nil {
		ctx.FreshnessStatus = *row.TrackingFreshness
	}
	if row.TrackingQuality != nil {
		ctx.QualityStatus = *row.TrackingQuality
	}
	return ctx
}
