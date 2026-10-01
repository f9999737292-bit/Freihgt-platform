package executionproj

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/lowcode"
)

const (
	callerName = "network-optimizer-service"
	path       = "/internal/v1/transport-executions/from-route-plan-activation"
)

// Command is the shipment-service projection contract. NLO sends it and does not write TMS tables.
type Command struct {
	ActivationID            uuid.UUID  `json:"activation_id"`
	ActivationVersion       int        `json:"activation_version"`
	ActivationStatus        string     `json:"activation_status"`
	RoutePlanID             uuid.UUID  `json:"route_plan_id"`
	RoutePlanVersion        int        `json:"route_plan_version"`
	PlanningMode            string     `json:"planning_mode"`
	OperatingTenantID       uuid.UUID  `json:"operating_tenant_id"`
	ContextShipmentID       *uuid.UUID `json:"context_shipment_id"`
	ContextShipmentTenantID *uuid.UUID `json:"context_shipment_tenant_id"`
	ContextShipmentVersion  *int       `json:"context_shipment_version"`
	CarrierCompanyID        uuid.UUID  `json:"carrier_company_id"`
	VehicleID               *uuid.UUID `json:"vehicle_id"`
	DriverID                *uuid.UUID `json:"driver_id"`
	EvaluationFingerprint   string     `json:"evaluation_fingerprint"`
	SupersedesRoutePlanID   *uuid.UUID `json:"supersedes_route_plan_id"`
	SupersedesActivationID  *uuid.UUID `json:"supersedes_activation_id"`
	ExecutionSubjects       []Subject  `json:"execution_subjects"`
	Stops                   []Stop     `json:"stops"`
	Actions                 []Action   `json:"actions"`
}

type Subject struct {
	RouteSubjectType         string     `json:"route_subject_type"`
	RouteSubjectID           uuid.UUID  `json:"route_subject_id"`
	RouteSubjectVersion      int        `json:"route_subject_version"`
	ExecutionShipmentID      *uuid.UUID `json:"execution_shipment_id"`
	ShipmentTenantID         *uuid.UUID `json:"shipment_tenant_id"`
	ExecutionShipmentVersion *int       `json:"execution_shipment_version"`
	CargoID                  *uuid.UUID `json:"cargo_id"`
	CargoVersion             *int       `json:"cargo_version"`
}

type Stop struct {
	RoutePlanStopID        uuid.UUID  `json:"route_plan_stop_id"`
	Ordinal                int        `json:"ordinal"`
	StopRole               string     `json:"stop_role"`
	PointKind              string     `json:"point_kind"`
	LocationID             *uuid.UUID `json:"location_id"`
	Latitude               float64    `json:"latitude"`
	Longitude              float64    `json:"longitude"`
	PlannedArrival         *time.Time `json:"planned_arrival"`
	PlannedDeparture       *time.Time `json:"planned_departure"`
	ServiceDurationSeconds *int       `json:"service_duration_seconds"`
}

type Action struct {
	RoutePlanActionID        uuid.UUID  `json:"route_plan_action_id"`
	RoutePlanStopID          uuid.UUID  `json:"route_plan_stop_id"`
	ActionOrdinal            int        `json:"action_ordinal"`
	ActionType               string     `json:"action_type"`
	RouteSubjectType         string     `json:"route_subject_type"`
	RouteSubjectID           uuid.UUID  `json:"route_subject_id"`
	ExecutionShipmentID      *uuid.UUID `json:"execution_shipment_id"`
	ShipmentTenantID         *uuid.UUID `json:"shipment_tenant_id"`
	ExecutionShipmentVersion *int       `json:"execution_shipment_version"`
	CargoID                  *uuid.UUID `json:"cargo_id"`
	CargoVersion             *int       `json:"cargo_version"`
	EvidenceState            string     `json:"evidence_state"`
	EvidenceStateVersion     *int       `json:"evidence_state_version"`
}

// Ack is the committed TMS correlation. Created is not part of the public correlation.
type Ack struct {
	OperatingTenantID uuid.UUID `json:"operating_tenant_id"`
	ActivationID      uuid.UUID `json:"activation_id"`
	RoutePlanID       uuid.UUID `json:"route_plan_id"`
	ExecutionID       uuid.UUID `json:"execution_id"`
	RevisionID        uuid.UUID `json:"execution_revision_id"`
}

// Error classifies a TMS call. Temporary failures leave the activation pending.
type Error struct {
	Temporary bool
	Reason    string
}

func (e *Error) Error() string {
	if e == nil {
		return "execution projection failed"
	}
	if e.Reason != "" {
		return e.Reason
	}
	if e.Temporary {
		return "execution projection unavailable"
	}
	return "execution projection rejected"
}

type Client interface {
	Project(context.Context, uuid.UUID, Command) (Ack, error)
}

type HTTPClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTP(baseURL, token string) *HTTPClient {
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (h *HTTPClient) Project(ctx context.Context, tenant uuid.UUID, cmd Command) (Ack, error) {
	if h == nil || h.baseURL == "" || h.token == "" {
		return Ack{}, &Error{Temporary: true, Reason: "execution projection unavailable"}
	}
	raw, err := json.Marshal(cmd)
	if err != nil {
		return Ack{}, &Error{Temporary: false, Reason: "execution projection request is invalid"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return Ack{}, &Error{Temporary: true, Reason: "execution projection unavailable"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(lowcode.HeaderTenantID, tenant.String())
	req.Header.Set(internalauth.HeaderName, h.token)
	req.Header.Set("X-Internal-Service-Name", callerName)
	resp, err := h.client.Do(req)
	if err != nil {
		return Ack{}, &Error{Temporary: true, Reason: "execution projection unavailable"}
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Ack{}, &Error{Temporary: true, Reason: "execution projection unavailable"}
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		var ack Ack
		if err := json.Unmarshal(payload, &ack); err != nil {
			return Ack{}, &Error{Temporary: true, Reason: "execution projection unavailable"}
		}
		return ack, nil
	}
	reason := errorReason(payload)
	switch resp.StatusCode {
	case http.StatusBadRequest, http.StatusConflict, http.StatusForbidden, http.StatusNotFound:
		return Ack{}, &Error{Temporary: false, Reason: reason}
	default:
		return Ack{}, &Error{Temporary: true, Reason: reason}
	}
}

func errorReason(payload []byte) string {
	var body struct {
		Error struct {
			Details map[string]any `json:"details"`
			Message string         `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return "execution projection rejected"
	}
	if reason, ok := body.Error.Details["reason"].(string); ok && reason != "" {
		return reason
	}
	if body.Error.Message != "" {
		return body.Error.Message
	}
	return "execution projection rejected"
}
