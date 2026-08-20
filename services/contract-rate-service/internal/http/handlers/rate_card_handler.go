package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
	"github.com/freight-platform/contract-rate-service/internal/platform/respond"
	"github.com/freight-platform/contract-rate-service/internal/service"
)

type RateCardHandler struct {
	svc    *service.RateCardService
	actors *ActorResolver
}

func NewRateCardHandler(svc *service.RateCardService, actors *ActorResolver) *RateCardHandler {
	return &RateCardHandler{svc: svc, actors: actors}
}

func (h *RateCardHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	contractID, err := parsePathUUID(chi.URLParam(r, "contractId"), "contract_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	created, err := h.svc.Create(r.Context(), domain.CreateRateCardInput{
		TenantID: actor.TenantID, ContractID: contractID, Name: req.Name, Description: req.Description, Actor: actor,
	}, CorrelationID(r))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, mapRateCard(created))
}

func (h *RateCardHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	contractID, err := parsePathUUID(chi.URLParam(r, "contractId"), "contract_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	items, err := h.svc.ListByContract(r.Context(), actor.TenantID, contractID, actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]any, 0, len(items))
	for i := range items {
		out = append(out, mapRateCard(&items[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *RateCardHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	rateCardID, err := parsePathUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, err := h.svc.Get(r.Context(), actor.TenantID, rateCardID, actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapRateCard(item))
}

func (h *RateCardHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	rateCardID, err := parsePathUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		ValidFrom string  `json:"valid_from"`
		ValidTo   *string `json:"valid_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	validFrom, err := parseDate(req.ValidFrom, "valid_from")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var validTo *time.Time
	if req.ValidTo != nil {
		vt, err := parseDate(*req.ValidTo, "valid_to")
		if err != nil {
			respond.Error(w, err)
			return
		}
		validTo = &vt
	}
	created, err := h.svc.CreateDraftVersion(r.Context(), domain.CreateRateVersionInput{
		TenantID: actor.TenantID, RateCardID: rateCardID, ValidFrom: validFrom, ValidTo: validTo, Actor: actor,
	}, CorrelationID(r))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, mapRateVersion(created))
}

func (h *RateCardHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	rateCardID, err := parsePathUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	items, err := h.svc.ListVersions(r.Context(), actor.TenantID, rateCardID, actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]any, 0, len(items))
	for i := range items {
		out = append(out, mapRateVersion(&items[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *RateCardHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	versionID, err := parsePathUUID(chi.URLParam(r, "versionId"), "version_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, err := h.svc.GetVersion(r.Context(), actor.TenantID, versionID, actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapRateVersion(item))
}

func (h *RateCardHandler) PatchVersion(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	versionID, err := parsePathUUID(chi.URLParam(r, "versionId"), "version_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		ValidFrom *string `json:"valid_from"`
		ValidTo   *string `json:"valid_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	patch := domain.UpdateRateVersionInput{Actor: actor}
	if req.ValidFrom != nil {
		vf, err := parseDate(*req.ValidFrom, "valid_from")
		if err != nil {
			respond.Error(w, err)
			return
		}
		patch.ValidFrom = &vf
	}
	if req.ValidTo != nil {
		vt, err := parseDate(*req.ValidTo, "valid_to")
		if err != nil {
			respond.Error(w, err)
			return
		}
		patch.ValidTo = &vt
	}
	updated, err := h.svc.UpdateDraftVersion(r.Context(), actor.TenantID, versionID, patch, CorrelationID(r))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapRateVersion(updated))
}

func (h *RateCardHandler) DiscardVersion(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	versionID, err := parsePathUUID(chi.URLParam(r, "versionId"), "version_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.svc.DiscardDraftVersion(r.Context(), actor.TenantID, versionID, actor, CorrelationID(r)); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mapRateCard(c *domain.RateCard) map[string]any {
	return map[string]any{
		"id": c.ID, "tenant_id": c.TenantID, "contract_id": c.ContractID,
		"name": c.Name, "description": c.Description, "created_at": c.CreatedAt, "updated_at": c.UpdatedAt, "version": c.Version,
	}
}

func mapRateVersion(v *domain.RateCardVersion) map[string]any {
	return map[string]any{
		"id": v.ID, "tenant_id": v.TenantID, "rate_card_id": v.RateCardID, "version_number": v.VersionNumber,
		"valid_from": v.ValidFrom.Format("2006-01-02"), "valid_to": datePtr(v.ValidTo), "status": v.Status,
		"supersedes_version_id": v.SupersedesVersionID, "created_at": v.CreatedAt, "version": v.Version,
	}
}

func parsePathUUID(raw, field string) (uuid.UUID, error) {
	return domain.ParseUUID(raw, field)
}

func parseDate(raw, field string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, apperrors.Validation("invalid date format, expected YYYY-MM-DD", map[string]any{"field": field})
	}
	return parsed.UTC(), nil
}

func datePtr(v *time.Time) any {
	if v == nil {
		return nil
	}
	return v.Format("2006-01-02")
}

func domainValidation(message string) error {
	return apperrors.Validation(message, map[string]any{})
}
