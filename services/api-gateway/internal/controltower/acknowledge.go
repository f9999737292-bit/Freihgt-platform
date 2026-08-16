package controltower

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
)

func (s *Service) AcknowledgeCriticalEvent(
	ctx context.Context,
	reqCtx RequestContext,
	eventID string,
	rawBody []byte,
) (ControlTowerEventAcknowledgement, error) {
	if !ValidateCriticalEventID(strings.ToLower(strings.TrimSpace(eventID))) {
		return ControlTowerEventAcknowledgement{}, apperrors.Validation("invalid eventId", map[string]any{"field": "eventId"})
	}
	eventID = strings.ToLower(strings.TrimSpace(eventID))

	if err := validateAcknowledgeRequestBody(rawBody); err != nil {
		return ControlTowerEventAcknowledgement{}, err
	}

	if reqCtx.UserID == "" {
		return ControlTowerEventAcknowledgement{}, apperrors.Unauthorized("verified user context is required")
	}

	if !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		return ControlTowerEventAcknowledgement{}, apperrors.ControlTowerReadModelUnavailable("control tower read model is temporarily unavailable")
	}

	criticalEvents, err := s.buildTenantCriticalEvents(ctx, reqCtx)
	if err != nil {
		return ControlTowerEventAcknowledgement{}, err
	}

	event, ok := FindCriticalEventByID(criticalEvents, eventID)
	if !ok {
		return ControlTowerEventAcknowledgement{}, apperrors.NotFound("critical event not found")
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	remote, depErr := s.readModel.AcknowledgeCriticalEvent(rmCtx, controltowerreadmodel.AcknowledgeCriticalEventInput{
		TenantID:   reqCtx.TenantID,
		UserID:     reqCtx.UserID,
		RequestID:  reqCtx.RequestID,
		EventID:    event.ID,
		ShipmentID: event.ShipmentID,
		EventType:  event.Type,
		OccurredAt: event.OccurredAt,
		Source:     event.Source,
	})
	if depErr != nil {
		return ControlTowerEventAcknowledgement{}, apperrors.ControlTowerReadModelUnavailable("control tower read model is temporarily unavailable")
	}

	return mapRemoteAcknowledgement(*remote), nil
}

func (s *Service) buildTenantCriticalEvents(ctx context.Context, reqCtx RequestContext) ([]ControlTowerEvent, error) {
	now := time.Now().UTC()

	shipmentsRaw, _, err := s.client.FetchShipments(ctx, reqCtx)
	if err != nil {
		return nil, apperrors.ControlTowerShipmentsUnavailable("required shipment data is temporarily unavailable")
	}

	var (
		transportOrders []rawTransportOrder
		companies       []rawCompany
		documents       []rawDocument
	)

	transportOrders, _ = s.client.FetchTransportOrders(ctx, reqCtx)
	companies, _ = s.client.FetchCompanies(ctx, reqCtx)
	documents, _ = s.client.FetchDocuments(ctx, reqCtx)

	orderByID := map[string]rawTransportOrder{}
	for _, order := range transportOrders {
		orderByID[order.ID] = order
	}
	companyByID := map[string]rawCompany{}
	for _, company := range companies {
		companyByID[company.ID] = company
	}
	shipmentIDsWithDocs := shipmentDocumentIDs(documents)

	allRows := make([]ControlTowerShipment, 0, len(shipmentsRaw))
	for _, raw := range shipmentsRaw {
		allRows = append(allRows, s.mapShipment(raw, orderByID, companyByID, shipmentIDsWithDocs, now))
	}

	shipmentByID := make(map[string]ControlTowerShipment, len(allRows))
	for _, row := range allRows {
		shipmentByID[row.ID] = row
	}

	events := BuildCriticalEvents(allRows, shipmentIDsWithDocs, s.thresholds, now)
	s.mergeDriverCriticalEvents(ctx, reqCtx, &events, shipmentByID)
	return events, nil
}

func (s *Service) enrichCriticalEventAcknowledgements(
	ctx context.Context,
	reqCtx RequestContext,
	events *[]ControlTowerEvent,
) {
	if events == nil || len(*events) == 0 || !s.readModelCfg.Mode.Enabled() || s.readModel == nil {
		return
	}

	eventIDs := make([]string, 0, len(*events))
	for _, event := range *events {
		eventIDs = append(eventIDs, event.ID)
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()

	lookup, depErr := s.readModel.LookupAcknowledgements(rmCtx, reqCtx.TenantID, reqCtx.RequestID, eventIDs)
	if depErr != nil {
		return
	}

	for i := range *events {
		item, ok := lookup[(*events)[i].ID]
		if !ok {
			continue
		}
		(*events)[i].Acknowledgement = &ControlTowerEventAckSummary{
			AcknowledgedAt: item.AcknowledgedAt.UTC(),
			AcknowledgedBy: ControlTowerEventAcknowledgedBy{
				UserID: item.AcknowledgedByUserID,
			},
		}
	}
}

func validateAcknowledgeRequestBody(rawBody []byte) error {
	if len(rawBody) == 0 {
		return nil
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return apperrors.Validation("request body must be empty", map[string]any{"field": "body"})
	}
	if len(payload) > 0 {
		return apperrors.Validation("request body must be empty", map[string]any{"field": "body"})
	}
	return nil
}

func readAcknowledgeRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	return io.ReadAll(io.LimitReader(r.Body, 4096))
}

func mapRemoteAcknowledgement(remote controltowerreadmodel.RemoteAcknowledgement) ControlTowerEventAcknowledgement {
	source := remote.Source
	if source == "" {
		source = EventSourceControlTower
	}
	status := remote.Status
	if status == "" {
		status = WorkflowStatusAcknowledged
	}
	return ControlTowerEventAcknowledgement{
		EventID:        remote.EventID,
		ShipmentID:     remote.ShipmentID,
		EventType:      remote.EventType,
		OccurredAt:     remote.OccurredAt.UTC(),
		Source:         source,
		Status:         status,
		AcknowledgedAt: remote.AcknowledgedAt.UTC(),
		AcknowledgedBy: ControlTowerEventAcknowledgedBy{
			UserID: remote.AcknowledgedBy.UserID,
		},
	}
}
