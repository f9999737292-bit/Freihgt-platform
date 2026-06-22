package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/platform/respond"
	"github.com/freight-platform/low-code-service/internal/service"
)

type FormTemplateHandler struct {
	service *service.FormTemplateService
}

func NewFormTemplateHandler(svc *service.FormTemplateService) *FormTemplateHandler {
	return &FormTemplateHandler{service: svc}
}

type listFormTemplatesResponse struct {
	Items []formTemplateSummaryResponse `json:"items"`
}

type formTemplateSummaryResponse struct {
	ID            string  `json:"id"`
	TenantID      string  `json:"tenant_id"`
	EntityType    string  `json:"entity_type"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	Version       int     `json:"version"`
	SectionsCount int     `json:"sections_count"`
	FieldsCount   int     `json:"fields_count"`
	PublishedAt   *string `json:"published_at,omitempty"`
}

type formTemplateDetailResponse struct {
	ID          string                  `json:"id"`
	TenantID    string                  `json:"tenant_id"`
	EntityType  string                  `json:"entity_type"`
	Code        string                  `json:"code"`
	Name        string                  `json:"name"`
	Status      string                  `json:"status"`
	Version     int                     `json:"version"`
	PublishedAt *string                 `json:"published_at,omitempty"`
	Sections    []formSectionResponse   `json:"sections"`
}

type formSectionResponse struct {
	ID        string               `json:"id"`
	Code      string               `json:"code"`
	Title     string               `json:"title"`
	SortOrder int                  `json:"sort_order"`
	Fields    []formFieldResponse  `json:"fields"`
}

type formFieldResponse struct {
	ID                 string          `json:"id"`
	Code               string          `json:"code"`
	Label              string          `json:"label"`
	FieldType          string          `json:"field_type"`
	Required           bool            `json:"required"`
	ReadOnly           bool            `json:"read_only"`
	SystemField        bool            `json:"system_field"`
	OptionsJSON        json.RawMessage `json:"options_json,omitempty"`
	ValidationRuleJSON json.RawMessage `json:"validation_rule_json,omitempty"`
	VisibilityRuleJSON json.RawMessage `json:"visibility_rule_json,omitempty"`
	SortOrder          int             `json:"sort_order"`
}

func (h *FormTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	entityType := r.URL.Query().Get("entity_type")
	items, err := h.service.ListPublished(r.Context(), tenantID, entityType)
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

func (h *FormTemplateHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	tenantID, err := parseTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	idOrCode := chi.URLParam(r, "id")
	if idOrCode == "" {
		respond.Error(w, apperrors.Validation("id is required", map[string]any{"field": "id"}))
		return
	}

	detail, err := h.service.GetPublished(r.Context(), tenantID, idOrCode)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toDetailResponse(*detail))
}

func parseTenantID(r *http.Request) (uuid.UUID, error) {
	raw := tenantIDFromRequest(r)
	if raw == "" {
		return uuid.Nil, apperrors.TenantRequired()
	}
	tenantID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid tenant id", map[string]any{"header": tenantHeader})
	}
	return tenantID, nil
}

func toSummaryResponse(item domain.FormTemplateSummary) formTemplateSummaryResponse {
	return formTemplateSummaryResponse{
		ID:            item.ID.String(),
		TenantID:      item.TenantID.String(),
		EntityType:    item.EntityType,
		Code:          item.Code,
		Name:          item.Name,
		Status:        item.Status,
		Version:       item.Version,
		SectionsCount: item.SectionsCount,
		FieldsCount:   item.FieldsCount,
		PublishedAt:   formatTime(item.PublishedAt),
	}
}

func toDetailResponse(detail domain.FormTemplateDetail) formTemplateDetailResponse {
	sections := make([]formSectionResponse, 0, len(detail.Sections))
	for _, section := range detail.Sections {
		fields := make([]formFieldResponse, 0, len(section.Fields))
		for _, field := range section.Fields {
			fields = append(fields, formFieldResponse{
				ID:                 field.ID.String(),
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
		sections = append(sections, formSectionResponse{
			ID:        section.ID.String(),
			Code:      section.Code,
			Title:     section.Title,
			SortOrder: section.SortOrder,
			Fields:    fields,
		})
	}

	return formTemplateDetailResponse{
		ID:          detail.ID.String(),
		TenantID:    detail.TenantID.String(),
		EntityType:  detail.EntityType,
		Code:        detail.Code,
		Name:        detail.Name,
		Status:      detail.Status,
		Version:     detail.Version,
		PublishedAt: formatTime(detail.PublishedAt),
		Sections:    sections,
	}
}

func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
