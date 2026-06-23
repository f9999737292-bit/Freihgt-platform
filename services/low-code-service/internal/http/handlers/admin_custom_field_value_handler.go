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
	TemplateCode      string                    `json:"template_code"`
	TargetTemplateID  string                    `json:"target_template_id"`
	AllowWarnings     bool                      `json:"allow_warnings"`
	ValidationContext *validationContextRequest `json:"validation_context,omitempty"`
}

type migrateCustomFieldValuesIncompatibleFieldResponse struct {
	FieldCode string `json:"field_code"`
	Reason    string `json:"reason"`
}

type migrateCustomFieldValuesToActiveResponse struct {
	Status                string                                            `json:"status"`
	TenantID              string                                            `json:"tenant_id"`
	EntityType            string                                            `json:"entity_type"`
	EntityID              string                                            `json:"entity_id"`
	ActiveTemplateID      string                                            `json:"active_template_id"`
	TargetTemplateID      string                                            `json:"target_template_id"`
	SourceTemplateID      string                                            `json:"source_template_id,omitempty"`
	MigratedCount         int                                               `json:"migrated_count"`
	SkippedCount          int                                               `json:"skipped_count"`
	SkippedFields         []string                                          `json:"skipped_fields"`
	CopiedFields          []string                                          `json:"copied_fields"`
	LegacyFields          []string                                          `json:"legacy_fields"`
	MissingRequiredFields []string                                          `json:"missing_required_fields"`
	IncompatibleFields    []migrateCustomFieldValuesIncompatibleFieldResponse `json:"incompatible_fields"`
	Warnings              []string                                          `json:"warnings"`
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

type batchMigrationPreviewSummaryResponse struct {
	Total    int `json:"total"`
	Safe     int `json:"safe"`
	Warnings int `json:"warnings"`
	Blocked  int `json:"blocked"`
}

type batchMigrationPreviewResponse struct {
	TenantID       string                                 `json:"tenant_id"`
	EntityType     string                                 `json:"entity_type"`
	TemplateCode   string                                 `json:"template_code"`
	TargetTemplate migrationPreviewTargetTemplateResponse `json:"target_template"`
	Summary        batchMigrationPreviewSummaryResponse   `json:"summary"`
	Items          []migrationPreviewItemResponse           `json:"items"`
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

	var targetTemplateID uuid.UUID
	if strings.TrimSpace(req.TargetTemplateID) != "" {
		targetTemplateID, err = uuid.Parse(req.TargetTemplateID)
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid target_template_id", map[string]any{"field": "target_template_id"}))
			return
		}
	}

	templateCode := strings.TrimSpace(req.TemplateCode)
	if templateCode == "" {
		templateCode = strings.TrimSpace(req.Code)
	}

	result, err := h.service.MigrateToActiveTemplate(r.Context(), domain.MigrateCustomFieldValuesToActiveInput{
		TenantID:          tenantID,
		EntityType:        req.EntityType,
		EntityID:          entityID,
		Code:              req.Code,
		TemplateCode:      templateCode,
		TargetTemplateID:  targetTemplateID,
		AllowWarnings:     req.AllowWarnings,
		ValidationContext: validationContextFromRequest(r, req.ValidationContext),
		Audit:             auditContextFromRequest(r),
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, buildMigrateToActiveResponse(tenantID, req.EntityType, entityID, result))
}

func buildMigrateToActiveResponse(tenantID uuid.UUID, entityType string, entityID uuid.UUID, result *domain.MigrateCustomFieldValuesToActiveResult) migrateCustomFieldValuesToActiveResponse {
	incompatible := make([]migrateCustomFieldValuesIncompatibleFieldResponse, 0, len(result.IncompatibleFields))
	for _, field := range result.IncompatibleFields {
		incompatible = append(incompatible, migrateCustomFieldValuesIncompatibleFieldResponse{
			FieldCode: field.FieldCode,
			Reason:    field.Reason,
		})
	}

	response := migrateCustomFieldValuesToActiveResponse{
		Status:                result.Status,
		TenantID:              tenantID.String(),
		EntityType:            entityType,
		EntityID:              entityID.String(),
		ActiveTemplateID:      result.ActiveTemplateID.String(),
		TargetTemplateID:      result.TargetTemplateID.String(),
		MigratedCount:         result.MigratedCount,
		SkippedCount:          result.SkippedCount,
		SkippedFields:         nonNilStringSlice(result.SkippedFields),
		CopiedFields:          nonNilStringSlice(result.CopiedFields),
		LegacyFields:          nonNilStringSlice(result.LegacyFields),
		MissingRequiredFields: nonNilStringSlice(result.MissingRequiredFields),
		IncompatibleFields:    incompatible,
		Warnings:              nonNilStringSlice(result.Warnings),
	}
	if result.SourceTemplateID != uuid.Nil {
		response.SourceTemplateID = result.SourceTemplateID.String()
	}
	return response
}

func (h *AdminCustomFieldValueHandler) MigrationPreview(w http.ResponseWriter, r *http.Request) {
	tenantID, input, err := parseMigrationPreviewRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	result, err := h.service.PreviewMigrationToActive(r.Context(), input)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, buildMigrationPreviewResponse(tenantID, result))
}

func (h *AdminCustomFieldValueHandler) BatchMigrationPreview(w http.ResponseWriter, r *http.Request) {
	tenantID, input, err := parseMigrationPreviewRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	result, err := h.service.PreviewMigrationToActive(r.Context(), input)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, buildBatchMigrationPreviewResponse(tenantID, input.TemplateCode, result))
}

func parseMigrationPreviewRequest(r *http.Request) (uuid.UUID, domain.MigrationPreviewInput, error) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		return uuid.Nil, domain.MigrationPreviewInput{}, err
	}

	var req migrationPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return uuid.Nil, domain.MigrationPreviewInput{}, apperrors.Validation("invalid request body", map[string]any{"error": err.Error()})
	}

	entityIDs := make([]uuid.UUID, 0, len(req.EntityIDs))
	for _, rawID := range req.EntityIDs {
		entityID, err := uuid.Parse(rawID)
		if err != nil {
			return uuid.Nil, domain.MigrationPreviewInput{}, apperrors.EntityIDInvalid(map[string]any{"field": "entity_id", "value": rawID})
		}
		entityIDs = append(entityIDs, entityID)
	}

	var targetTemplateID uuid.UUID
	if strings.TrimSpace(req.TargetTemplateID) != "" {
		targetTemplateID, err = uuid.Parse(req.TargetTemplateID)
		if err != nil {
			return uuid.Nil, domain.MigrationPreviewInput{}, apperrors.Validation("invalid target_template_id", map[string]any{"field": "target_template_id"})
		}
	}

	return tenantID, domain.MigrationPreviewInput{
		TenantID:         tenantID,
		EntityType:       req.EntityType,
		EntityIDs:        entityIDs,
		TemplateCode:     req.TemplateCode,
		TargetTemplateID: targetTemplateID,
	}, nil
}

func buildMigrationPreviewItemResponses(items []domain.MigrationPreviewItem) []migrationPreviewItemResponse {
	responseItems := make([]migrationPreviewItemResponse, 0, len(items))
	for _, item := range items {
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
		responseItems = append(responseItems, responseItem)
	}
	return responseItems
}

func buildMigrationPreviewResponse(tenantID uuid.UUID, result *domain.MigrationPreviewResult) migrationPreviewResponse {
	return migrationPreviewResponse{
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
		Items: buildMigrationPreviewItemResponses(result.Items),
	}
}

func buildBatchMigrationPreviewResponse(tenantID uuid.UUID, requestedTemplateCode string, result *domain.MigrationPreviewResult) batchMigrationPreviewResponse {
	templateCode := strings.TrimSpace(requestedTemplateCode)
	if templateCode == "" {
		templateCode = result.TargetTemplate.Code
	}

	return batchMigrationPreviewResponse{
		TenantID:     tenantID.String(),
		EntityType:   result.EntityType,
		TemplateCode: templateCode,
		TargetTemplate: migrationPreviewTargetTemplateResponse{
			ID:      result.TargetTemplate.ID.String(),
			Code:    result.TargetTemplate.Code,
			Version: result.TargetTemplate.Version,
		},
		Summary: batchMigrationPreviewSummaryResponse{
			Total:    result.Summary.EntitiesChecked,
			Safe:     result.Summary.SafeToMigrate,
			Warnings: result.Summary.Warnings,
			Blocked:  result.Summary.Blocked,
		},
		Items: buildMigrationPreviewItemResponses(result.Items),
	}
}

type migrationPreviewResponse struct {
	TenantID       string                                 `json:"tenant_id"`
	EntityType     string                                 `json:"entity_type"`
	TargetTemplate migrationPreviewTargetTemplateResponse `json:"target_template"`
	Summary        migrationPreviewSummaryResponse        `json:"summary"`
	Items          []migrationPreviewItemResponse         `json:"items"`
}

func nonNilStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
