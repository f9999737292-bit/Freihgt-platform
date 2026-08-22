package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	"github.com/freight-platform/freight-cost-service/internal/platform/respond"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

type VarianceHandler struct {
	derived  *service.DerivedProjectionService
	mappings *repository.ChargeCodeMappingRepository
}

func NewVarianceHandler(
	derived *service.DerivedProjectionService,
	mappings *repository.ChargeCodeMappingRepository,
) *VarianceHandler {
	return &VarianceHandler{derived: derived, mappings: mappings}
}

func (h *VarianceHandler) ReconcileTransportOrder(w http.ResponseWriter, r *http.Request) {
	tenantID, err := ParseTrustedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	transportOrderID, err := uuid.Parse(chi.URLParam(r, "transportOrderId"))
	if err != nil || transportOrderID == uuid.Nil {
		respond.Error(w, apperrors.Validation("invalid transport order id", map[string]any{"field": "transport_order_id"}))
		return
	}
	count, err := h.derived.ReconcileTransportOrder(r.Context(), tenantID, transportOrderID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"findings_detected": count,
		"auto_rebuild":      false,
		"auto_repair":       false,
	})
}

func (h *VarianceHandler) ReclassifyAttribution(w http.ResponseWriter, r *http.Request) {
	tenantID, err := ParseTrustedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	transportOrderID, err := uuid.Parse(chi.URLParam(r, "transportOrderId"))
	if err != nil || transportOrderID == uuid.Nil {
		respond.Error(w, apperrors.Validation("invalid transport order id", map[string]any{"field": "transport_order_id"}))
		return
	}
	inserted, err := h.derived.ReclassifyAttribution(r.Context(), tenantID, transportOrderID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"attribution_rows_inserted": inserted,
		"financial_amounts_changed": false,
	})
}

type chargeMappingRequest struct {
	MappingScope   string  `json:"mapping_scope"`
	TenantID       *string `json:"tenant_id"`
	SourceCode     string  `json:"source_code"`
	TargetCategory string  `json:"target_category"`
	EffectiveFrom  *string `json:"effective_from"`
	EffectiveTo    *string `json:"effective_to"`
}

func (h *VarianceHandler) PutChargeCodeMapping(w http.ResponseWriter, r *http.Request) {
	requestTenantID, err := ParseTrustedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var payload chargeMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respond.Error(w, apperrors.Validation("invalid json body", map[string]any{"field": "body"}))
		return
	}
	scope := strings.ToUpper(strings.TrimSpace(payload.MappingScope))
	if scope == domain.MappingScopePlatform {
		respond.Error(w, apperrors.Forbidden("platform mapping mutation is disabled until verified platform-admin authorization is available"))
		return
	}
	if scope != domain.MappingScopeTenant {
		respond.Error(w, apperrors.Validation("mapping_scope must be TENANT", map[string]any{"field": "mapping_scope"}))
		return
	}

	var tenantID *uuid.UUID
	if payload.TenantID != nil {
		parsed, parseErr := uuid.Parse(*payload.TenantID)
		if parseErr != nil {
			respond.Error(w, apperrors.Validation("invalid tenant_id", map[string]any{"field": "tenant_id"}))
			return
		}
		if parsed != requestTenantID {
			respond.Error(w, apperrors.Forbidden("cross-tenant mapping denied"))
			return
		}
		tenantID = &parsed
	} else {
		tenantID = &requestTenantID
	}

	effectiveFrom := time.Now().UTC()
	if payload.EffectiveFrom != nil && strings.TrimSpace(*payload.EffectiveFrom) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*payload.EffectiveFrom))
		if parseErr != nil {
			respond.Error(w, apperrors.Validation("invalid effective_from", map[string]any{"field": "effective_from"}))
			return
		}
		effectiveFrom = parsed.UTC()
	}
	var effectiveTo *time.Time
	if payload.EffectiveTo != nil && strings.TrimSpace(*payload.EffectiveTo) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*payload.EffectiveTo))
		if parseErr != nil {
			respond.Error(w, apperrors.Validation("invalid effective_to", map[string]any{"field": "effective_to"}))
			return
		}
		v := parsed.UTC()
		effectiveTo = &v
	}

	mapping, err := h.mappings.UpsertMapping(r.Context(), repository.UpsertChargeCodeMappingInput{
		MappingScope:   scope,
		TenantID:       tenantID,
		SourceCode:     payload.SourceCode,
		TargetCategory: payload.TargetCategory,
		EffectiveFrom:  effectiveFrom,
		EffectiveTo:    effectiveTo,
		ActorID:        "internal",
	})
	if err != nil {
		if repository.IsOverlapConstraintViolation(err) {
			respond.Error(w, apperrors.Validation("overlapping active mapping window", map[string]any{"field": "effective_from"}))
			return
		}
		if strings.Contains(err.Error(), "effective_to must be after") || strings.Contains(err.Error(), "invalid target category") {
			respond.Error(w, apperrors.Validation(err.Error(), map[string]any{"field": "mapping"}))
			return
		}
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"mapping_scope":                 mapping.MappingScope,
		"source_charge_code_normalized": mapping.SourceChargeCodeNormalized,
		"normalized_category":           mapping.NormalizedCategory,
		"mapping_version":               mapping.MappingVersion,
	})
}
