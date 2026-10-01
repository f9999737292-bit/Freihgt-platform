package domain

import (
	"encoding/json"
	"strings"
)

const (
	RouteShipmentStatus = "shipment_status"
	RouteExecution      = "execution"
	RouteApproach       = "approach"
	RouteDisposition    = "disposition"
	RouteUnknown        = "unknown"

	EventDeliveryPartiallyRejected = "shipment.delivery.partially_rejected"
	EventDeliveryRejected          = "shipment.delivery.rejected"
	EventCargoDispositionPending   = "shipment.cargo.disposition_pending"
	EventCargoReturnAuthorized     = "shipment.cargo.return_authorized"
	EventCargoRedirectAuthorized   = "shipment.cargo.redirect_authorized"
	EventCargoReturnCompleted      = "shipment.cargo.return_completed"
	EventCargoRedirectCompleted    = "shipment.cargo.redirect_completed"

	EventExecutionPlanCreated    = "shipment.execution_plan.created"
	EventExecutionPlanSuperseded = "shipment.execution_plan.superseded"
	EventRouteStopCurrent        = "shipment.route_stop.current"
	EventRouteStopArrived        = "shipment.route_stop.arrived"
	EventRouteStopServiceStarted = "shipment.route_stop.service_started"
	EventRouteStopCompleted      = "shipment.route_stop.completed"
	EventRouteStopOverridden     = "shipment.route_stop.sequence_overridden"
	EventStopApproaching         = "tracking.stop.approaching"
)

func ClassifyEventPayload(payload []byte) (kind, eventType string) {
	var probe struct {
		EventType      string `json:"event_type"`
		EventTypeCamel string `json:"eventType"`
	}
	if err := json.Unmarshal(payload, &probe); err != nil {
		return RouteUnknown, ""
	}
	eventType = strings.TrimSpace(probe.EventType)
	if eventType == "" {
		eventType = strings.TrimSpace(probe.EventTypeCamel)
	}
	switch {
	case eventType == "":
		return RouteUnknown, ""
	case IsShipmentStatusEventType(eventType):
		return RouteShipmentStatus, eventType
	case IsDispositionEventType(eventType):
		return RouteDisposition, eventType
	case IsExecutionProgressEventType(eventType):
		return RouteExecution, eventType
	case eventType == EventStopApproaching:
		return RouteApproach, eventType
	default:
		return RouteUnknown, eventType
	}
}

func IsShipmentStatusEventType(eventType string) bool {
	_, ok := allowedEventTypes[strings.TrimSpace(eventType)]
	return ok
}

func IsDispositionEventType(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case EventDeliveryPartiallyRejected, EventDeliveryRejected, EventCargoDispositionPending,
		EventCargoReturnAuthorized, EventCargoRedirectAuthorized, EventCargoReturnCompleted, EventCargoRedirectCompleted:
		return true
	default:
		return false
	}
}

func IsExecutionProgressEventType(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case EventExecutionPlanCreated, EventExecutionPlanSuperseded,
		EventRouteStopCurrent, EventRouteStopArrived, EventRouteStopServiceStarted,
		EventRouteStopCompleted, EventRouteStopOverridden:
		return true
	default:
		return false
	}
}

func PayloadHasForbiddenExecutionField(payload []byte) bool {
	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return true
	}
	return walkForbidden(decoded)
}

func walkForbidden(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if forbiddenExecutionField(key) {
				return true
			}
			if walkForbidden(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if walkForbidden(child) {
				return true
			}
		}
	}
	return false
}

func forbiddenExecutionField(key string) bool {
	switch strings.TrimSpace(key) {
	case "shipment_tenant_id", "shipmentTenantId", "price", "rate", "capacity_snapshot", "capacitySnapshot",
		"raw_load_opportunity", "rawLoadOpportunity", "leg_geometry", "legGeometry",
		"latitude", "longitude", "route_subject_type", "routeSubjectType":
		return true
	default:
		return false
	}
}
