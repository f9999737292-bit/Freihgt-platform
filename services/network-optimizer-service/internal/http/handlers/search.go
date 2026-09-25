package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/platform/respond"
	"github.com/freight-platform/network-optimizer-service/internal/service"
)

func (h *Handler) SearchNextLoad(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		h.finish(w, r, "next_load_search", uuid.Nil, err)
		return
	}
	var body struct {
		CapacityID     uuid.UUID                   `json:"capacity_id"`
		Policy         domain.NextLoadSearchPolicy `json:"policy"`
		CandidateLimit *int                        `json:"candidate_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.finish(w, r, "next_load_search", uuid.Nil, apperrors.Validation("request body is invalid", nil))
		return
	}
	result, err := h.svc.SearchNextLoad(r.Context(), actor, service.SearchCommand{
		CapacityID: body.CapacityID, Policy: body.Policy, CandidateLimit: body.CandidateLimit,
	})
	if err != nil {
		h.finish(w, r, "next_load_search", uuid.Nil, err)
		return
	}
	h.log.Info("next-load search",
		slog.String("search_id", result.AggregateID.String()),
		slog.String("capacity_id", body.CapacityID.String()),
		slog.String("routing_provider", "configured"),
	)
	respond.Bytes(w, result.Status, result.Body)
}
