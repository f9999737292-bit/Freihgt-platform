package controltower

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

func (s *Service) ListCases(ctx context.Context, reqCtx RequestContext, r *http.Request) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	q := r.URL.Query()
	filter := controltowerreadmodel.CasesFilter{
		Status: q.Get("status"), Severity: q.Get("severity"),
		OwnerUserID: q.Get("ownerUserId"), ShipmentID: q.Get("shipmentId"),
		Search: q.Get("search"), Preset: q.Get("preset"), SlaState: q.Get("slaState"),
		Page: parseIntQuery(q.Get("page"), 1), Limit: parseIntQuery(q.Get("limit"), 50),
		MyCases: q.Get("myCases") == "true", Unassigned: q.Get("unassigned") == "true",
		IncludeClosed: q.Get("includeClosed") == "true",
		HasSlaBreach: q.Get("hasSlaBreach") == "true", HasSlaWarning: q.Get("hasSlaWarning") == "true",
		OverdueActions: q.Get("overdueActions") == "true" || q.Get("hasOverdueActions") == "true",
		HasOpenActions: q.Get("hasOpenActions") == "true",
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ListCases(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, filter)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return payload, nil
}

func (s *Service) GetCase(ctx context.Context, reqCtx RequestContext, caseID string) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.GetCase(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, caseID)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return enrichCasePayload(ctx, s, reqCtx, payload)
}

func (s *Service) ProxyCaseMutation(ctx context.Context, reqCtx RequestContext, method, caseID, suffix string, raw []byte) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	path := controltowerreadmodel.CasePath(caseID, suffix)
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ProxyCaseJSON(rmCtx, method, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, path, raw)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	if len(payload) == 0 {
		return nil, nil
	}
	return enrichCasePayload(ctx, s, reqCtx, payload)
}

func (s *Service) ProxyCaseCreate(ctx context.Context, reqCtx RequestContext, raw []byte) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ProxyCaseJSON(rmCtx, http.MethodPost, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, "/internal/v1/control-tower/cases", raw)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return enrichCasePayload(ctx, s, reqCtx, payload)
}

func (s *Service) GetCaseKPI(ctx context.Context, reqCtx RequestContext) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ProxyCaseJSON(rmCtx, http.MethodGet, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, controltowerreadmodel.CaseKPIPath, nil)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return payload, nil
}

func enrichCasePayload(ctx context.Context, s *Service, reqCtx RequestContext, payload json.RawMessage) (json.RawMessage, error) {
	userNames, _ := s.loadTenantUserNames(ctx, reqCtx)
	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		return payload, nil
	}
	if ownerID, ok := doc["ownerUserId"].(string); ok {
		if name, ok := userNames[ownerID]; ok {
			doc["ownerDisplayName"] = name
		}
	}
	if createdBy, ok := doc["createdByUserId"].(string); ok {
		if name, ok := userNames[createdBy]; ok {
			doc["createdByDisplayName"] = name
		}
	}
	if participants, ok := doc["participants"].([]any); ok {
		for _, p := range participants {
			if pm, ok := p.(map[string]any); ok {
				if uid, ok := pm["userId"].(string); ok {
					if name, ok := userNames[uid]; ok {
						pm["displayName"] = name
					}
				}
			}
		}
	}
	shipmentsRaw, _, _ := s.client.FetchShipments(ctx, reqCtx)
	shipmentByID := map[string]rawShipment{}
	orderByID := map[string]rawTransportOrder{}
	for _, sh := range shipmentsRaw {
		shipmentByID[sh.ID] = sh
	}
	if orders, err := s.client.FetchTransportOrders(ctx, reqCtx); err == nil {
		for _, o := range orders {
			orderByID[o.ID] = o
		}
	}
	if links, ok := doc["links"].([]any); ok {
		linkedShipments := make([]map[string]any, 0)
		linkedOrders := make([]map[string]any, 0)
		for _, link := range links {
			lm, ok := link.(map[string]any)
			if !ok {
				continue
			}
			entityType, _ := lm["entityType"].(string)
			entityID, _ := lm["entityId"].(string)
			switch entityType {
			case "shipment":
				if sh, found := shipmentByID[entityID]; found {
					linkedShipments = append(linkedShipments, mapShipmentSummary(sh))
				} else {
					linkedShipments = append(linkedShipments, map[string]any{"id": entityID})
				}
			case "transport_order":
				if ord, found := orderByID[entityID]; found {
					linkedOrders = append(linkedOrders, map[string]any{
						"id": ord.ID, "reference": ord.OrderNumber,
					})
				} else {
					linkedOrders = append(linkedOrders, map[string]any{"id": entityID})
				}
			}
		}
		if len(linkedShipments) > 0 {
			doc["linkedShipments"] = linkedShipments
		}
		if len(linkedOrders) > 0 {
			doc["linkedTransportOrders"] = linkedOrders
		}
	}
	if caseID, ok := doc["id"].(string); ok && caseID != "" {
		s.enrichCaseAutomation(ctx, reqCtx, caseID, doc)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return payload, nil
	}
	return out, nil
}

func mapShipmentSummary(sh rawShipment) map[string]any {
	item := map[string]any{
		"id": sh.ID, "reference": sh.ShipmentNumber, "status": sh.Status,
	}
	if sh.PlannedPickupAt != nil {
		item["plannedPickupAt"] = sh.PlannedPickupAt.UTC().Format(time.RFC3339)
	}
	if sh.PlannedDeliveryAt != nil {
		item["plannedDeliveryAt"] = sh.PlannedDeliveryAt.UTC().Format(time.RFC3339)
	}
	if sh.ActualPickupAt != nil {
		item["actualPickupAt"] = sh.ActualPickupAt.UTC().Format(time.RFC3339)
	}
	if sh.ActualDeliveryAt != nil {
		item["actualDeliveryAt"] = sh.ActualDeliveryAt.UTC().Format(time.RFC3339)
	}
	if sh.DriverID != nil {
		item["driverId"] = *sh.DriverID
	}
	if sh.VehicleID != nil {
		item["vehicleId"] = *sh.VehicleID
	}
	if sh.CarrierCompanyID != nil {
		item["carrierCompanyId"] = *sh.CarrierCompanyID
	}
	item["originLocationId"] = sh.OriginLocationID
	item["destinationLocationId"] = sh.DestinationLocationID
	return item
}

func parseIntQuery(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 1 {
		return fallback
	}
	return n
}
