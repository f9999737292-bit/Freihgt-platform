package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/platform/respond"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

func (h *Handler) EvaluateRoutePlan(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		h.finish(w, r, "route_plan_evaluate", uuid.Nil, err)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		h.finishStatus(w, r, "route_plan_evaluate", uuid.Nil, apperrors.Validation("request body is invalid", nil), http.StatusBadRequest)
		return
	}
	key, err := requiredIdempotency(r)
	if err != nil {
		h.finishStatus(w, r, "route_plan_evaluate", uuid.Nil, err, http.StatusBadRequest)
		return
	}
	cmd, err := decodeRoutePlan(raw)
	if err != nil {
		h.finishStatus(w, r, "route_plan_evaluate", uuid.Nil, err, http.StatusBadRequest)
		return
	}
	result, err := h.svc.EvaluateRoutePlan(r.Context(), actor, key, cmd)
	if err != nil {
		h.finishRoutePlan(w, r, "route_plan_evaluate", err)
		return
	}
	respond.Bytes(w, result.Status, result.Body)
}

func (h *Handler) GetRoutePlan(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		h.finish(w, r, "route_plan_get", uuid.Nil, err)
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		h.finishStatus(w, r, "route_plan_get", uuid.Nil, err, http.StatusBadRequest)
		return
	}
	result, err := h.svc.GetRoutePlan(r.Context(), actor, id)
	if err != nil {
		h.finish(w, r, "route_plan_get", id, err)
		return
	}
	respond.Bytes(w, result.Status, result.Body)
}

func (h *Handler) AcceptRoutePlan(w http.ResponseWriter, r *http.Request) {
	h.decideRoutePlan(w, r, "route_plan_accept", func(actor service.Actor, id uuid.UUID, key string, raw []byte) (service.Result, error) {
		return h.svc.AcceptRoutePlan(r.Context(), actor, key, id, raw)
	})
}

func (h *Handler) ActivateRoutePlan(w http.ResponseWriter, r *http.Request) {
	h.decideRoutePlan(w, r, "route_plan_activate", func(actor service.Actor, id uuid.UUID, key string, raw []byte) (service.Result, error) {
		return h.svc.ActivateRoutePlan(r.Context(), actor, key, id, raw)
	})
}

func (h *Handler) decideRoutePlan(w http.ResponseWriter, r *http.Request, operation string, decide func(service.Actor, uuid.UUID, string, []byte) (service.Result, error)) {
	actor, err := actorFrom(r)
	if err != nil {
		h.finish(w, r, operation, uuid.Nil, err)
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		h.finishStatus(w, r, operation, uuid.Nil, err, http.StatusBadRequest)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		h.finishStatus(w, r, operation, id, apperrors.Validation("request body is invalid", nil), http.StatusBadRequest)
		return
	}
	key, err := requiredIdempotency(r)
	if err != nil {
		h.finishStatus(w, r, operation, id, err, http.StatusBadRequest)
		return
	}
	result, err := decide(actor, id, key, raw)
	if err != nil {
		h.finishRoutePlan(w, r, operation, err)
		return
	}
	respond.Bytes(w, result.Status, result.Body)
}

func (h *Handler) finishRoutePlan(w http.ResponseWriter, r *http.Request, operation string, err error) {
	status := http.StatusBadRequest
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case apperrors.CodeNotFound:
			status = http.StatusNotFound
		case apperrors.CodeConflict:
			status = http.StatusConflict
		case apperrors.CodeValidation:
			if reason, _ := appErr.Details["reason"].(string); reason != "" {
				status = http.StatusUnprocessableEntity
			}
		default:
			status = http.StatusInternalServerError
		}
	}
	h.finishStatus(w, r, operation, uuid.Nil, err, status)
}

func requiredIdempotency(r *http.Request) (string, error) {
	if _, ok := r.Header["Idempotency-Key"]; !ok {
		return "", apperrors.Validation("Idempotency-Key is required", nil)
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		return "", apperrors.Validation("Idempotency-Key must be 1..128 characters", nil)
	}
	return key, nil
}

func decodeRoutePlan(raw []byte) (service.RoutePlanCommand, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return service.RoutePlanCommand{}, apperrors.Validation("request body is invalid", nil)
	}
	var body struct {
		PlanningMode     string      `json:"planning_mode"`
		ShipmentID       *uuid.UUID  `json:"shipment_id"`
		CapacityID       *uuid.UUID  `json:"capacity_id"`
		CandidateLoadIDs []uuid.UUID `json:"candidate_load_ids"`
	}
	for _, key := range []string{"service_duration", "service_duration_seconds", "pickup_duration_seconds", "delivery_duration_seconds"} {
		if _, ok := keys[key]; ok {
			return service.RoutePlanCommand{}, apperrors.Validation("caller supplied service duration is not allowed", nil)
		}
	}
	if _, ok := keys["start"]; ok {
		return service.RoutePlanCommand{}, apperrors.Validation("caller supplied start position is not allowed", nil)
	}
	if err := decode(raw, &body); err != nil {
		return service.RoutePlanCommand{}, err
	}
	cmd := service.RoutePlanCommand{
		PlanningMode: body.PlanningMode, ShipmentID: body.ShipmentID, CapacityID: body.CapacityID,
		CandidateLoadIDs: body.CandidateLoadIDs, Raw: append([]byte(nil), raw...),
	}
	_ = routeplan.ModeCurrentTrip
	return cmd, nil
}
