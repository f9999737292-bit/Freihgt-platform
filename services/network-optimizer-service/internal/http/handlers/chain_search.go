package handlers

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chainapi"
	"github.com/freight-platform/network-optimizer-service/internal/platform/respond"
)

func (h *Handler) UseChainAPI(api *chainapi.API) {
	h.chain = api
}

func (h *Handler) PostChainSearch(w http.ResponseWriter, r *http.Request) {
	if h.chain == nil {
		writeChainError(w, chainapiUnavailable())
		return
	}
	actor, err := actorFrom(r)
	if err != nil {
		h.finish(w, r, "chain_search_post", uuid.Nil, err)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeChainError(w, &chainapi.StatusError{Status: http.StatusBadRequest, Code: "VALIDATION_ERROR", Message: "request body is invalid", Details: map[string]any{}})
		return
	}
	_, supplied := r.Header["Idempotency-Key"]
	result, err := h.chain.Post(r.Context(), actor, raw, r.Header.Get("Idempotency-Key"), supplied)
	if err != nil {
		writeChainError(w, err)
		return
	}
	respond.JSON(w, result.Status, result.Body)
}

func (h *Handler) GetChainSearch(w http.ResponseWriter, r *http.Request) {
	h.chainRead(w, r, "chain_search_get", func(actorID uuid.UUID) (any, error) {
		actor, err := actorFrom(r)
		if err != nil {
			return nil, err
		}
		_ = actorID
		id, err := parseID(chi.URLParam(r, "searchId"))
		if err != nil {
			return nil, &chainapi.StatusError{Status: http.StatusBadRequest, Code: "VALIDATION_ERROR", Message: "searchId must be a uuid", Details: map[string]any{}}
		}
		return h.chain.Get(r.Context(), actor, id)
	})
}

func (h *Handler) GetChainCandidates(w http.ResponseWriter, r *http.Request) {
	h.chainRead(w, r, "chain_search_candidates", func(uuid.UUID) (any, error) {
		actor, err := actorFrom(r)
		if err != nil {
			return nil, err
		}
		id, err := parseID(chi.URLParam(r, "searchId"))
		if err != nil {
			return nil, &chainapi.StatusError{Status: http.StatusBadRequest, Code: "VALIDATION_ERROR", Message: "searchId must be a uuid", Details: map[string]any{}}
		}
		return h.chain.Candidates(r.Context(), actor, id)
	})
}

func (h *Handler) chainRead(w http.ResponseWriter, r *http.Request, operation string, fn func(uuid.UUID) (any, error)) {
	if h.chain == nil {
		writeChainError(w, chainapiUnavailable())
		return
	}
	body, err := fn(uuid.Nil)
	if err != nil {
		var status *chainapi.StatusError
		if errors.As(err, &status) {
			writeChainError(w, err)
			return
		}
		h.finish(w, r, operation, uuid.Nil, err)
		return
	}
	respond.JSON(w, http.StatusOK, body)
}

func chainapiUnavailable() error {
	return &chainapi.StatusError{Status: http.StatusServiceUnavailable, Code: "SERVICE_UNAVAILABLE", Message: "chain search dependency unavailable", Details: map[string]any{}}
}

func writeChainError(w http.ResponseWriter, err error) {
	var status *chainapi.StatusError
	if !errors.As(err, &status) || status == nil {
		status = &chainapi.StatusError{Status: http.StatusServiceUnavailable, Code: "SERVICE_UNAVAILABLE", Message: "chain search dependency unavailable", Details: map[string]any{}}
	}
	if status.Details == nil {
		status.Details = map[string]any{}
	}
	for key, value := range status.Headers {
		w.Header().Set(key, value)
	}
	respond.JSON(w, status.Status, map[string]any{
		"error": map[string]any{"code": status.Code, "message": status.Message, "details": status.Details},
	})
}
