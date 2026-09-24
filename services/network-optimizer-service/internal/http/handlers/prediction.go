package handlers

import (
	"bytes"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

type predictionVersionBody struct {
	Version int `json:"version"`
}

func (h *Handler) GeneratePrediction(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "generate_prediction", func(actor service.Actor, raw []byte, key, hash string) (service.Result, error) {
		id, err := parseID(chi.URLParam(r, "shipmentId"))
		if err != nil {
			return service.Result{}, err
		}
		if len(bytes.TrimSpace(raw)) > 0 {
			if err := decode(raw, &struct{}{}); err != nil {
				return service.Result{}, err
			}
		}
		return h.svc.GeneratePrediction(r.Context(), actor, id, key, hash)
	})
}

func (h *Handler) RefreshPrediction(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "refresh_prediction", func(actor service.Actor, raw []byte, key, hash string) (service.Result, error) {
		id, version, err := predictionCommand(chi.URLParam(r, "id"), raw)
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.RefreshPrediction(r.Context(), actor, id, version, key, hash)
	})
}

func (h *Handler) ActivatePrediction(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "activate_prediction", func(actor service.Actor, raw []byte, key, hash string) (service.Result, error) {
		id, version, err := predictionCommand(chi.URLParam(r, "id"), raw)
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.ActivatePrediction(r.Context(), actor, id, version, key, hash)
	})
}

func (h *Handler) GetPrediction(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "get_prediction", func(actor service.Actor) (service.Result, error) {
		id, err := parseID(chi.URLParam(r, "id"))
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.GetPrediction(r.Context(), actor, id)
	})
}

func (h *Handler) ListPredictions(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "list_predictions", func(actor service.Actor) (service.Result, error) {
		limit, offset, err := pageQuery(r)
		if err != nil {
			return service.Result{}, err
		}
		return h.svc.ListPredictions(r.Context(), actor, limit, offset)
	})
}

func predictionCommand(rawID string, raw []byte) (uuid.UUID, int, error) {
	id, err := parseID(rawID)
	if err != nil {
		return uuid.Nil, 0, err
	}
	var body predictionVersionBody
	if err := decode(raw, &body); err != nil {
		return uuid.Nil, 0, err
	}
	if body.Version < 1 {
		return uuid.Nil, 0, apperrors.Validation("version must be >= 1", map[string]any{"field": "version"})
	}
	return id, body.Version, nil
}
