package controltower

import (
	"context"
	"time"

	"github.com/freight-platform/api-gateway/internal/controltower/risk"
	"github.com/freight-platform/api-gateway/internal/tracking"
)

func (s *Service) enrichShipmentsWithSlots(ctx context.Context, reqCtx RequestContext, rows []ControlTowerShipment) {
	if s.trackingClient == nil || len(rows) == 0 {
		return
	}
	ids := make([]string, 0, len(rows))
	index := make(map[string]int, len(rows))
	contextPayload := make(map[string]tracking.SlotLookupContext, len(rows))
	for i, row := range rows {
		ids = append(ids, row.ID)
		index[row.ID] = i
		ctxItem := tracking.SlotLookupContext{ShipmentStatus: row.Status}
		ctxItem.ActualPickupAt = tracking.FormatTimePtr(row.ActualPickupAt)
		ctxItem.ActualDeliveryAt = tracking.FormatTimePtr(row.ActualDeliveryAt)
		ctxItem.PickupETA = etaSnapshotFromRow(row, "pickup")
		ctxItem.DeliveryETA = etaSnapshotFromRow(row, "delivery")
		contextPayload[row.ID] = ctxItem
	}
	states, err := s.trackingClient.LookupSlots(ctx, reqCtx.TenantID, reqCtx.RequestID, ids, contextPayload)
	if err != nil {
		return
	}
	for id, summary := range states {
		i, ok := index[id]
		if !ok {
			continue
		}
		applySlotSummary(&rows[i], summary)
	}
}

func etaSnapshotFromRow(row ControlTowerShipment, phase string) *tracking.ETASnapshotIn {
	if phase == "delivery" {
		if row.ETAStatus == nil {
			return nil
		}
		snap := &tracking.ETASnapshotIn{
			Status:          *row.ETAStatus,
			FreshnessStatus: "",
			QualityStatus:   "",
		}
		if row.ETAFreshness != nil {
			snap.FreshnessStatus = *row.ETAFreshness
		}
		if row.ETAQuality != nil {
			snap.QualityStatus = *row.ETAQuality
		}
		snap.HasUsableETA = *row.ETAStatus == risk.ETAStatusAvailable || *row.ETAStatus == risk.ETAStatusStale
		if row.EstimatedDeliveryAt != nil {
			snap.EstimatedArrivalAt = tracking.FormatTimePtr(row.EstimatedDeliveryAt)
		}
		return snap
	}
	return nil
}

func applySlotSummary(row *ControlTowerShipment, summary tracking.ShipmentSlotSummary) {
	if summary.Pickup != nil {
		applySlotTarget(row, *summary.Pickup, "pickup")
	}
	if summary.Delivery != nil {
		applySlotTarget(row, *summary.Delivery, "delivery")
	}
}

func applySlotTarget(row *ControlTowerShipment, summary tracking.SlotTargetSummary, phase string) {
	if phase == "pickup" {
		status := summary.WindowStatus
		row.PickupSlotWindowStatus = &status
		row.PickupSlotWindowStart = summary.WindowStart
		row.PickupSlotWindowEnd = summary.WindowEnd
		row.PickupSlotStatus = summary.SlotStatus
		projection := summary.ArrivalProjection
		row.PickupSlotArrivalProjection = &projection
		row.PickupSlotProjectedLateSeconds = summary.ProjectedLateBySeconds
		row.PickupSlotMarginSeconds = summary.MarginSeconds
		return
	}
	status := summary.WindowStatus
	row.DeliverySlotWindowStatus = &status
	row.DeliverySlotWindowStart = summary.WindowStart
	row.DeliverySlotWindowEnd = summary.WindowEnd
	row.DeliverySlotStatus = summary.SlotStatus
	projection := summary.ArrivalProjection
	row.DeliverySlotArrivalProjection = &projection
	row.DeliverySlotProjectedLateSeconds = summary.ProjectedLateBySeconds
	row.DeliverySlotMarginSeconds = summary.MarginSeconds
}

func slotContextFromShipment(row ControlTowerShipment) *risk.SlotContext {
	ctx := &risk.SlotContext{}
	if row.PickupSlotWindowStatus != nil {
		ctx.Pickup = slotTargetContextFromRow(row, "pickup")
	}
	if row.DeliverySlotWindowStatus != nil {
		ctx.Delivery = slotTargetContextFromRow(row, "delivery")
	}
	if ctx.Pickup == nil && ctx.Delivery == nil {
		return nil
	}
	return ctx
}

func slotTargetContextFromRow(row ControlTowerShipment, phase string) *risk.SlotTargetContext {
	var windowStatus, projection *string
	var windowEnd *time.Time
	var projectedLate, margin *int64

	if phase == "pickup" {
		windowStatus = row.PickupSlotWindowStatus
		projection = row.PickupSlotArrivalProjection
		windowEnd = row.PickupSlotWindowEnd
		projectedLate = row.PickupSlotProjectedLateSeconds
		margin = row.PickupSlotMarginSeconds
	} else {
		windowStatus = row.DeliverySlotWindowStatus
		projection = row.DeliverySlotArrivalProjection
		windowEnd = row.DeliverySlotWindowEnd
		projectedLate = row.DeliverySlotProjectedLateSeconds
		margin = row.DeliverySlotMarginSeconds
	}
	if windowStatus == nil {
		return nil
	}
	target := &risk.SlotTargetContext{
		HasRealWindow:          *windowStatus == "available",
		ArrivalProjection:      "",
		WindowEnd:              windowEnd,
		ProjectedLateBySeconds: projectedLate,
		MarginSeconds:          margin,
	}
	if projection != nil {
		target.ArrivalProjection = *projection
	}
	if phase == "delivery" {
		if row.ETAStatus != nil {
			target.HasUsableETA = *row.ETAStatus == risk.ETAStatusAvailable || *row.ETAStatus == risk.ETAStatusStale
		}
		if row.ETAFreshness != nil {
			target.ETAFreshness = *row.ETAFreshness
		}
	}
	return target
}
