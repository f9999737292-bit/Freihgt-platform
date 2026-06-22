package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/service"
)

type stubFormTemplateRepo struct {
	listItems  []domain.FormTemplateSummary
	listErr    error
	getDetail  *domain.FormTemplateDetail
	getErr     error
}

func (s *stubFormTemplateRepo) ListPublished(ctx context.Context, tenantID uuid.UUID, entityType string) ([]domain.FormTemplateSummary, error) {
	return s.listItems, s.listErr
}

func (s *stubFormTemplateRepo) GetPublishedByID(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.FormTemplateDetail, error) {
	return s.getDetail, s.getErr
}

func (s *stubFormTemplateRepo) GetPublishedByCode(ctx context.Context, tenantID uuid.UUID, code string) (*domain.FormTemplateDetail, error) {
	return s.getDetail, s.getErr
}

func TestListTenantRequired(t *testing.T) {
	handler := NewFormTemplateHandler(service.NewFormTemplateService(&stubFormTemplateRepo{}))
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/form-templates", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "TENANT_REQUIRED")
}

func TestListInvalidEntityType(t *testing.T) {
	handler := NewFormTemplateHandler(service.NewFormTemplateService(&stubFormTemplateRepo{}))
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/form-templates?entity_type=BAD", nil)
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestListReturnsPublishedOnlyFromService(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	stub := &stubFormTemplateRepo{
		listItems: []domain.FormTemplateSummary{
			{
				ID:         uuid.New(),
				TenantID:   tenantID,
				EntityType: "TRANSPORT_ORDER",
				Code:       "transport_order_default",
				Name:       "Transport Order Default Form",
				Status:     domain.PublishedStatus,
				Version:    1,
			},
		},
	}
	handler := NewFormTemplateHandler(service.NewFormTemplateService(stub))
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/form-templates", nil)
	req.Header.Set(tenantHeader, tenantID.String())
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Items []struct {
			Status string `json:"status"`
			Code   string `json:"code"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Status != domain.PublishedStatus {
		t.Fatalf("expected published template in response: %+v", payload.Items)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	handler := NewFormTemplateHandler(service.NewFormTemplateService(&stubFormTemplateRepo{
		getErr: apperrors.FormTemplateNotFound(),
	}))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", uuid.New().String())
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/form-templates/x", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()

	handler.GetByID(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "FORM_TEMPLATE_NOT_FOUND")
}

func assertErrorCode(t *testing.T, body []byte, code string) {
	t.Helper()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if payload.Error.Code != code {
		t.Fatalf("expected error code %s, got %s", code, payload.Error.Code)
	}
}
