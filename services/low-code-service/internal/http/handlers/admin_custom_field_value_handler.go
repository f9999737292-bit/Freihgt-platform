package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

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

type migrationPreviewRequest struct {
	EntityType       string   `json:"entity_type"`
	EntityIDs        []string `json:"entity_ids"`
	TemplateCode     string   `json:"template_code,omitempty"`
	TargetTemplateID string   `json:"target_template_id,omitempty"`
}

type migrationPreviewTargetTemplateResponse struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Version int    `json:"version"`
}

type migrationPreviewSummaryResponse struct {
	EntitiesChecked int `json:"entities_checked"`
	SafeToMigrate   int `json:"safe_to_migrate"`
	Warnings        int `json:"warnings"`
	Blocked         int `json:"blocked"`
}

type migrationPreviewIncompatibleFieldResponse struct {
	FieldCode string `json:"field_code"`
	Reason    string `json:"reason"`
}

type migrationPreviewItemResponse struct {
	EntityID              string                                      `json:"entity_id"`
	SourceTemplateID      string                                      `json:"source_template_id,omitempty"`
	TargetTemplateID      string                                      `json:"target_template_id"`
	Status                string                                      `json:"status"`
	CopiedFields          []string                                    `json:"copied_fields"`
	LegacyFields          []string                                    `json:"legacy_fields"`
	MissingRequiredFields []string                                    `json:"missing_required_fields"`
	IncompatibleFields    []migrationPreviewIncompatibleFieldResponse `json:"incompatible_fields"`
	Warnings              []string                                    `json:"warnings"`
}

type migrationPreviewResponse struct {
	TenantID       string                                 `json:"tenant_id"`
	EntityType     string                                 `json:"entity_type"`
	TargetTemplate migrationPreviewTargetTemplateResponse `json:"target_template"`
	Summary        migrationPreviewSummaryResponse        `json:"summary"`
	Items          []migrationPreviewItemResponse         `json:"items"`
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

func (h *AdminCustomFieldValueHandler) MigrationPreview(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req migrationPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"error": err.Error()}))
		return
	}

	entityIDs := make([]uuid.UUID, 0, len(req.EntityIDs))
	for _, rawID := range req.EntityIDs {
		entityID, err := uuid.Parse(rawID)
		if err != nil {
			respond.Error(w, apperrors.EntityIDInvalid(map[string]any{"field": "entity_id", "value": rawID}))
			return
		}
		entityIDs = append(entityIDs, entityID)
	}

	var targetTemplateID uuid.UUID
	if strings.TrimSpace(req.TargetTemplateID) != "" {
		targetTemplateID, err = uuid.Parse(req.TargetTemplateID)
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid target_template_id", map[string]any{"field": "target_template_id"}))
			return
		}
	}

	result, err := h.service.PreviewMigrationToActive(r.Context(), domain.MigrationPreviewInput{
		TenantID:         tenantID,
		EntityType:       req.EntityType,
		EntityIDs:        entityIDs,
		TemplateCode:     req.TemplateCode,
		TargetTemplateID: targetTemplateID,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]migrationPreviewItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		incompatible := make([]migrationPreviewIncompatibleFieldResponse, 0, len(item.IncompatibleFields))
		for _, field := range item.IncompatibleFields {
			incompatible = append(incompatible, migrationPreviewIncompatibleFieldResponse{
				FieldCode: field.FieldCode,
				Reason:    field.Reason,
			})
		}

		responseItem := migrationPreviewItemResponse{
			EntityID:              item.EntityID.String(),
			TargetTemplateID:      item.TargetTemplateID.String(),
			Status:                item.Status,
			CopiedFields:          nonNilStringSlice(item.CopiedFields),
			LegacyFields:          nonNilStringSlice(item.LegacyFields),
			MissingRequiredFields: nonNilStringSlice(item.MissingRequiredFields),
			IncompatibleFields:    incompatible,
			Warnings:              nonNilStringSlice(item.Warnings),
		}
		if item.SourceTemplateID != uuid.Nil {
			responseItem.SourceTemplateID = item.SourceTemplateID.String()
		}
		items = append(items, responseItem)
	}

	respond.JSON(w, http.StatusOK, migrationPreviewResponse{
		TenantID:   result.TenantID.String(),
		EntityType: result.EntityType,
		TargetTemplate: migrationPreviewTargetTemplateResponse{
			ID:      result.TargetTemplate.ID.String(),
			Code:    result.TargetTemplate.Code,
			Version: result.TargetTemplate.Version,
		},
		Summary: migrationPreviewSummaryResponse{
			EntitiesChecked: result.Summary.EntitiesChecked,
			SafeToMigrate:   result.Summary.SafeToMigrate,
			Warnings:        result.Summary.Warnings,
			Blocked:         result.Summary.Blocked,
		},
		Items: items,
	})
}

func nonNilStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
