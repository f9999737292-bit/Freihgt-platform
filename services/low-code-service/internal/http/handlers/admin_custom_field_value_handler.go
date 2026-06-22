package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/platform/respond"
	"github.com/freight-platform/low-code-service/internal/service"
)

type AdminCustomFieldValueHandler struct {
	service *service.CustomFieldValueService
}

func NewAdminCustomFieldValueHandler(svc *service.CustomFieldValueService) *AdminCustomFieldValueHandler {
	return &AdminCustomFieldValueHandler{service: svc}
}

type migrateCustomFieldValuesToActiveRequest struct {
	EntityType        string                    `json:"entity_type"`
	EntityID          string                    `json:"entity_id"`
	Code              string                    `json:"code"`
	ValidationContext *validationContextRequest `json:"validation_context,omitempty"`
}

type migrateCustomFieldValuesToActiveResponse struct {
	Status           string   `json:"status"`
	ActiveTemplateID string   `json:"active_template_id"`
	MigratedCount    int      `json:"migrated_count"`
	SkippedCount     int      `json:"skipped_count"`
	SkippedFields    []string `json:"skipped_fields"`
}

func (h *AdminCustomFieldValueHandler) MigrateToActive(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req migrateCustomFieldValuesToActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"error": err.Error()}))
		return
	}

	entityID, err := uuid.Parse(req.EntityID)
	if err != nil {
		respond.Error(w, apperrors.EntityIDInvalid(map[string]any{"field": "entity_id", "value": req.EntityID}))
		return
	}

	result, err := h.service.MigrateToActiveTemplate(r.Context(), domain.MigrateCustomFieldValuesToActiveInput{
		TenantID:          tenantID,
		EntityType:        req.EntityType,
		EntityID:          entityID,
		Code:              req.Code,
		ValidationContext: validationContextFromRequest(r, req.ValidationContext),
		Audit:             auditContextFromRequest(r),
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	skipped := result.SkippedFields
	if skipped == nil {
		skipped = []string{}
	}

	respond.JSON(w, http.StatusOK, migrateCustomFieldValuesToActiveResponse{
		Status:           "ok",
		ActiveTemplateID: result.ActiveTemplateID.String(),
		MigratedCount:    result.MigratedCount,
		SkippedCount:     result.SkippedCount,
		SkippedFields:    skipped,
	})
}
