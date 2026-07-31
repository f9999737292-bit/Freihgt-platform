package shipmentevents

import (
	"context"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/config"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/sla"
)

type Service struct {
	client     *DownstreamClient
	thresholds sla.Thresholds
	maxFetch   int
}

func NewService(cfg config.Config, client *DownstreamClient) *Service {
	return &Service{
		client: client,
		thresholds: sla.Thresholds{
			AtRiskMinutes:        cfg.ControlTower.AtRiskMinutes,
			CriticalDelayMinutes: cfg.ControlTower.CriticalDelayMinutes,
			StaleWarningMinutes:  cfg.ControlTower.StaleWarningMinutes,
			StaleCriticalMinutes: cfg.ControlTower.StaleCriticalMinutes,
		},
		maxFetch: cfg.ControlTower.MaxDownstreamFetchLimit,
	}
}

func (s *Service) GetEvents(ctx context.Context, reqCtx RequestContext, shipmentID string, query ListQuery) (EventsResponse, error) {
	fetchResult, err := s.client.FetchShipment(ctx, reqCtx, shipmentID)
	if err != nil {
		if isUnavailableError(err) {
			return EventsResponse{}, apperrors.ShipmentEventsShipmentUnavailable("shipment data source is temporarily unavailable")
		}
		return EventsResponse{}, apperrors.Internal("failed to load shipment", err)
	}
	if fetchResult.NotFound || fetchResult.Shipment == nil {
		return EventsResponse{}, apperrors.NotFound("shipment not found")
	}
	shipment := *fetchResult.Shipment
	if !strings.EqualFold(strings.TrimSpace(shipment.TenantID), strings.TrimSpace(reqCtx.TenantID)) {
		return EventsResponse{}, apperrors.NotFound("shipment not found")
	}

	now := time.Now().UTC()
	freshness := DataFreshness{
		ShipmentLoaded:        true,
		ShipmentEventsLoaded:  false,
		DocumentsLoaded:       false,
		BillingLoaded:         false,
		TechnicalEventsLoaded: false,
		Partial:               true,
		Warnings: []string{
			WarningShipmentHistoryDerived,
			WarningBillingEventsUnavailable,
		},
	}

	events := buildDerivedShipmentEvents(shipment)

	if slaEvent := buildSLAEvent(shipment, s.thresholds, now); slaEvent != nil {
		events = append(events, *slaEvent)
	}

	documents, docsLimited, docErr := s.client.FetchDocuments(ctx, reqCtx, shipmentID)
	if docErr != nil {
		freshness.Warnings = appendUnique(freshness.Warnings, WarningDocumentEventsUnavailable)
	} else {
		freshness.DocumentsLoaded = true
		events = append(events, buildDocumentEvents(shipment, documents)...)
		if docsLimited {
			freshness.Warnings = appendUnique(freshness.Warnings, WarningTimelineLimitedDataset)
		}
	}

	events = dedupeEvents(events)
	events = filterEvents(events, query)
	sortEvents(events, query.Order)
	filters := buildFilterOptions(events)
	page := paginateEvents(events, query.Page, query.Limit)

	return EventsResponse{
		Shipment: ShipmentSummary{
			ID:     shipment.ID,
			Number: shipment.ShipmentNumber,
			Status: shipment.Status,
		},
		GeneratedAt:   now,
		DataFreshness: freshness,
		Timeline:      page,
		Filters:       filters,
	}, nil
}

func isUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "service returned 5")
}

func appendUnique(values []string, code string) []string {
	for _, existing := range values {
		if existing == code {
			return values
		}
	}
	return append(values, code)
}
