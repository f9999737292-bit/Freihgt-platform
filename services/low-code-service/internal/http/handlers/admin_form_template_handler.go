package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/platform/respond"
	"github.com/freight-platform/low-code-service/internal/service"
)

type AdminFormTemplateHandler struct {
	service *service.AdminFormTemplateService
}

func NewAdminFormTemplateHandler(svc *service.AdminFormTemplateService) *AdminFormTemplateHandler {
	return &AdminFormTemplateHandler{service: svc}
}

type createDraftFormTemplateResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Version int    `json:"version"`
}

type clonePublishedToDraftResponse struct {
	ID               string `json:"id"`
	SourceTemplateID string `json:"source_template_id"`
	Status           string `json:"status"`
	Version          int    `json:"version"`
	Code             string `json:"code"`
}

type draftFormTemplateRequest struct {
	EntityType  string                      `json:"entity_type"`
	Code        string                      `json:"code"`
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	Sections    []draftFormSectionRequest   `json:"sections"`
}

type draftFormSectionRequest struct {
	Code      string                   `json:"code"`
	Title     string                   `json:"title"`
	SortOrder int                      `json:"sort_order"`
	Fields    []draftFormFieldRequest  `json:"fields"`
}

type draftFormFieldRequest struct {
	Code               string          `json:"code"`
	Label              string          `json:"label"`
	FieldType          string          `json:"field_type"`
	Required           bool            `json:"required"`
	ReadOnly           bool            `json:"read_only"`
	SystemField        bool            `json:"system_field"`
	OptionsJSON        json.RawMessage `json:"options_json"`
	ValidationRuleJSON json.RawMessage `json:"validation_rule_json"`
	VisibilityRuleJSON json.RawMessage `json:"visibility_rule_json"`
	SortOrder          int             `json:"sort_order"`
}

type adminFormTemplateDetailResponse struct {
	ID          string                `json:"id"`
	TenantID    string                `json:"tenant_id"`
	EntityType  string                `json:"entity_type"`
	Code        string                `json:"code"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Status      string                `json:"status"`
	Version     int                   `json:"version"`
	PublishedAt *string               `json:"published_at,omitempty"`
	Sections    []formSectionResponse `json:"sections"`
}

func (h *AdminFormTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req draftFormTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid json body", map[string]any{"error": err.Error()}))
		return
	}

	result, err := h.service.CreateDraft(r.Context(), tenantID, toDraftInput(req), auditContextFromRequest(r))
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, createDraftFormTemplateResponse{
		ID:      result.ID.String(),
		Status:  result.Status,
		Version: result.Version,
	})
}

func (h *AdminFormTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	entityType := strings.TrimSpace(r.URL.Query().Get("entity_type"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	limit := parseLimit(r.URL.Query().Get("limit"), 50, 100)

	items, err := h.service.List(r.Context(), tenantID, entityType, status, limit)
	if err != nil {
		respond.Error(w, err)
		return
	}

	responseItems := make([]formTemplateSummaryResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, toSummaryResponse(item))
	}
	respond.JSON(w, http.StatusOK, listFormTemplatesResponse{Items: responseItems})
}

func (h *AdminFormTemplateHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	templateID, err := parseTemplateID(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, err)
		return
	}

	detail, err := h.service.GetByID(r.Context(), tenantID, templateID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toAdminDetailResponse(*detail))
}

func (h *AdminFormTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	templateID, err := parseTemplateID(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req draftFormTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid json body", map[string]any{"error": err.Error()}))
		return
	}

	if err := h.service.UpdateDraft(r.Context(), tenantID, templateID, toDraftInput(req), auditContextFromRequest(r)); err != nil {
		respond.Error(w, err)
		return
	}

	detail, err := h.service.GetByID(r.Context(), tenantID, templateID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toAdminDetailResponse(*detail))
}

func (h *AdminFormTemplateHandler) Publish(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	templateID, err := parseTemplateID(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, err)
		return
	}

	detail, err := h.service.PublishDraft(r.Context(), tenantID, templateID, auditContextFromRequest(r))
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toAdminDetailResponse(*detail))
}

func (h *AdminFormTemplateHandler) CloneToDraft(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	templateID, err := parseTemplateID(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, err)
		return
	}

	result, err := h.service.ClonePublishedToDraft(r.Context(), tenantID, templateID, auditContextFromRequest(r))
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, clonePublishedToDraftResponse{
		ID:               result.ID.String(),
		SourceTemplateID: result.SourceTemplateID.String(),
		Status:           result.Status,
		Version:          result.Version,
		Code:             result.Code,
	})
}

func parseTemplateID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	templateID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid template id", map[string]any{"field": "id"})
	}
	return templateID, nil
}

func parseLimit(raw string, defaultLimit int, maxLimit int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultLimit
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultLimit
	}
	if value > maxLimit {
		return maxLimit
	}
	return value
}

func toDraftInput(req draftFormTemplateRequest) domain.DraftFormTemplateInput {
	sections := make([]domain.DraftFormSectionInput, 0, len(req.Sections))
	for _, section := range req.Sections {
		fields := make([]domain.DraftFormFieldInput, 0, len(section.Fields))
		for _, field := range section.Fields {
			fields = append(fields, domain.DraftFormFieldInput{
				Code:               field.Code,
				Label:              field.Label,
				FieldType:          field.FieldType,
				Required:           field.Required,
				ReadOnly:           field.ReadOnly,
				SystemField:        field.SystemField,
				OptionsJSON:        field.OptionsJSON,
				ValidationRuleJSON: field.ValidationRuleJSON,
				VisibilityRuleJSON: field.VisibilityRuleJSON,
				SortOrder:          field.SortOrder,
			})
		}
		sections = append(sections, domain.DraftFormSectionInput{
			Code:      section.Code,
			Title:     section.Title,
			SortOrder: section.SortOrder,
			Fields:    fields,
		})
	}

	return domain.DraftFormTemplateInput{
		EntityType:  req.EntityType,
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Sections:    sections,
	}
}

func toAdminDetailResponse(detail domain.FormTemplateDetail) adminFormTemplateDetailResponse {
	base := toDetailResponse(detail)
	return adminFormTemplateDetailResponse{
		ID:          base.ID,
		TenantID:    base.TenantID,
		EntityType:  base.EntityType,
		Code:        base.Code,
		Name:        base.Name,
		Description: detail.Description,
		Status:      base.Status,
		Version:     base.Version,
		PublishedAt: formatTime(detail.PublishedAt),
		Sections:    base.Sections,
	}
}
