package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/compat"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	"github.com/freight-platform/network-optimizer-service/internal/platform/respond"
	"github.com/freight-platform/network-optimizer-service/internal/reference"
)

func (h *Handler) UseCatalog(catalog reference.Catalog) {
	h.catalog = catalog
}

func (h *Handler) EvaluateCargoEquipment(w http.ResponseWriter, r *http.Request) {
	h.evaluate(w, r, false)
}

func (h *Handler) EvaluateGroupage(w http.ResponseWriter, r *http.Request) {
	h.evaluate(w, r, true)
}

func (h *Handler) evaluate(w http.ResponseWriter, r *http.Request, group bool) {
	actor, err := actorFrom(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body struct {
		Equipment compat.Equipment  `json:"equipment"`
		Cargo     compat.Cargo      `json:"cargo"`
		Cargoes   []compat.Cargo    `json:"cargoes"`
		Access    compat.AccessNeed `json:"access"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", nil))
		return
	}
	ctx, err := h.catalog.Evaluation(r.Context(), actor.TenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var result compat.Result
	if group {
		result = compat.EvaluateGroupage(body.Equipment, body.Cargoes, body.Access, ctx)
	} else {
		result = compat.EvaluateCargoEquipment(body.Cargo, body.Equipment, body.Access, ctx)
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) ListCargoTypes(w http.ResponseWriter, r *http.Request) {
	h.listNamed(w, r, "CARGO")
}

func (h *Handler) ListEquipmentTypes(w http.ResponseWriter, r *http.Request) {
	h.listNamed(w, r, "EQUIPMENT_TYPE")
}

func (h *Handler) ListPalletTypes(w http.ResponseWriter, r *http.Request) {
	h.listNamed(w, r, "PALLET_TYPE")
}

func (h *Handler) ListPackagingTypes(w http.ResponseWriter, r *http.Request) {
	h.listNamed(w, r, "PACKAGING_TYPE")
}

func (h *Handler) listNamed(w http.ResponseWriter, r *http.Request, kind string) {
	actor, err := actorFrom(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	limit, offset := page(r)
	if kind == "CARGO" {
		items, err := h.catalog.ListCargo(r.Context(), actor.TenantID, limit, offset)
		if err != nil {
			respond.Error(w, err)
			return
		}
		respond.JSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
		return
	}
	items, err := h.catalog.ListNamed(r.Context(), kind, actor.TenantID, limit, offset)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (h *Handler) ListRuleSets(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items, err := h.catalog.ListRuleSets(r.Context(), actor.TenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateRuleSet(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	set, err := h.catalog.CreateRuleSet(r.Context(), actor.UserID, actor.TenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, set)
}

func (h *Handler) AddCompatibilityRule(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	setID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid rule set id", nil))
		return
	}
	var rule reference.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", nil))
		return
	}
	if strings.EqualFold(rule.Layer, "REGULATORY") {
		respond.Error(w, apperrors.Forbidden("tenant rules cannot be regulatory"))
		return
	}
	rule.Layer = "TENANT"
	if err := h.catalog.AddRule(r.Context(), actor.UserID, actor.TenantID, setID, rule); err != nil {
		writeCatalogError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, map[string]any{"status": "DRAFT"})
}

func (h *Handler) RemoveCompatibilityRule(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFrom(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	setID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid rule set id", nil))
		return
	}
	if err := h.catalog.RemoveRule(r.Context(), actor.UserID, actor.TenantID, setID, chi.URLParam(r, "ruleCode")); err != nil {
		writeCatalogError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ActivateRuleSet(w http.ResponseWriter, r *http.Request) {
	h.transitionRuleSet(w, r, true)
}

func (h *Handler) RetireRuleSet(w http.ResponseWriter, r *http.Request) {
	h.transitionRuleSet(w, r, false)
}

func (h *Handler) transitionRuleSet(w http.ResponseWriter, r *http.Request, activate bool) {
	actor, err := actorFrom(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	setID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid rule set id", nil))
		return
	}
	if activate {
		err = h.catalog.ActivateRuleSet(r.Context(), actor.UserID, actor.TenantID, setID)
	} else {
		err = h.catalog.RetireRuleSet(r.Context(), actor.UserID, actor.TenantID, setID)
	}
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"id": setID})
}

func writeCatalogError(w http.ResponseWriter, err error) {
	if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "immutable") {
		respond.Error(w, apperrors.NotFound("rule set not found"))
		return
	}
	respond.Error(w, apperrors.Validation(err.Error(), nil))
}

func page(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
