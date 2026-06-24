package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/repository"
	"github.com/freight-platform/low-code-service/internal/service"
)

type stubAdminFormTemplateRepo struct {
	createResult *repository.CreateDraftResult
	createErr    error
	listItems    []domain.FormTemplateSummary
	listErr      error
	getDetail    *domain.FormTemplateDetail
	getErr       error
	updateErr    error
	publishDetail *domain.FormTemplateDetail
	publishErr   error
	cloneResult  *repository.ClonePublishedToDraftResult
	cloneErr     error
	lastCreateInput repository.CreateDraftInput
	lastUpdateInput repository.UpdateDraftInput
	lastCloneTenantID uuid.UUID
	lastCloneSourceID uuid.UUID
	exportErr           error
	lastExportTenantID    uuid.UUID
	lastExportTemplateID  uuid.UUID
	lastExportAudit       domain.AuditContext
	lastExportSchemaVersion string
	exportRecorded        bool
	listByCodeItems       []domain.FormTemplateSummary
	listByCodeErr         error
	importPreviewRecorded bool
	lastImportPreviewAudit domain.AuditContext
	importPreviewErr      error
	importResult          *repository.ImportTemplateAsDraftResult
	importErr             error
	lastImportInput       repository.ImportTemplateAsDraftInput
	importCallCount       int
}

func (s *stubAdminFormTemplateRepo) CreateDraft(_ context.Context, input repository.CreateDraftInput) (*repository.CreateDraftResult, error) {
	s.lastCreateInput = input
	return s.createResult, s.createErr
}

func (s *stubAdminFormTemplateRepo) ListAdmin(_ context.Context, _ repository.AdminListFilter) ([]domain.FormTemplateSummary, error) {
	return s.listItems, s.listErr
}

func (s *stubAdminFormTemplateRepo) GetByID(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*domain.FormTemplateDetail, error) {
	return s.getDetail, s.getErr
}

func (s *stubAdminFormTemplateRepo) UpdateDraft(_ context.Context, input repository.UpdateDraftInput) error {
	s.lastUpdateInput = input
	return s.updateErr
}

func (s *stubAdminFormTemplateRepo) PublishDraft(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ domain.AuditContext) (*domain.FormTemplateDetail, error) {
	return s.publishDetail, s.publishErr
}

func (s *stubAdminFormTemplateRepo) ClonePublishedToDraft(_ context.Context, tenantID uuid.UUID, sourceTemplateID uuid.UUID, _ domain.AuditContext) (*repository.ClonePublishedToDraftResult, error) {
	s.lastCloneTenantID = tenantID
	s.lastCloneSourceID = sourceTemplateID
	return s.cloneResult, s.cloneErr
}

func (s *stubAdminFormTemplateRepo) RecordTemplateExport(_ context.Context, tenantID uuid.UUID, templateID uuid.UUID, _ domain.FormTemplateDetail, audit domain.AuditContext, schemaVersion string) error {
	s.exportRecorded = true
	s.lastExportTenantID = tenantID
	s.lastExportTemplateID = templateID
	s.lastExportAudit = audit
	s.lastExportSchemaVersion = schemaVersion
	return s.exportErr
}

func (s *stubAdminFormTemplateRepo) ListByEntityTypeAndCode(_ context.Context, _ uuid.UUID, _ string, _ string) ([]domain.FormTemplateSummary, error) {
	return s.listByCodeItems, s.listByCodeErr
}

func (s *stubAdminFormTemplateRepo) RecordTemplateImportPreview(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID, _ domain.TemplateImportPreviewInput, _ domain.TemplateImportPreviewResult, audit domain.AuditContext) error {
	s.importPreviewRecorded = true
	s.lastImportPreviewAudit = audit
	return s.importPreviewErr
}

func (s *stubAdminFormTemplateRepo) ImportTemplateAsDraft(_ context.Context, input repository.ImportTemplateAsDraftInput) (*repository.ImportTemplateAsDraftResult, error) {
	s.importCallCount++
	s.lastImportInput = input
	if s.importErr != nil {
		return nil, s.importErr
	}
	if s.importResult != nil {
		return s.importResult, nil
	}
	return &repository.ImportTemplateAsDraftResult{
		ID:            uuid.New(),
		Status:        domain.DraftStatus,
		Version:       2,
		Code:          input.DraftInput.Code,
		ReplacedDraft: input.ReplaceDraftID != nil,
	}, nil
}

func validDraftPayload() []byte {
	return []byte(`{
		"entity_type":"TRANSPORT_ORDER",
		"code":"transport_order_custom_v1",
		"name":"Transport Order Custom Form",
		"description":"Custom draft form",
		"sections":[{
			"code":"cargo",
			"title":"Cargo",
			"sort_order":100,
			"fields":[{
				"code":"cargo_class",
				"label":"Cargo class",
				"field_type":"SELECT",
				"sort_order":100,
				"options_json":{"options":["GENERAL","DANGEROUS","TEMPERATURE"]}
			}]
		}]
	}`)
}

func TestAdminCreateDraftTenantRequired(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates", bytes.NewReader(validDraftPayload()))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "TENANT_REQUIRED")
}

func TestAdminCreateDraftValidatesEntityType(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	body := bytes.NewReader([]byte(`{
		"entity_type":"BAD",
		"code":"transport_order_custom_v1",
		"name":"Custom",
		"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"x","label":"X","field_type":"TEXT"}]}]
	}`))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates", body)
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminCreateDraftValidatesFieldType(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	body := bytes.NewReader([]byte(`{
		"entity_type":"TRANSPORT_ORDER",
		"code":"transport_order_custom_v1",
		"name":"Custom",
		"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"x","label":"X","field_type":"BAD"}]}]
	}`))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates", body)
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminCreateDraftSuccess(t *testing.T) {
	templateID := uuid.New()
	stub := &stubAdminFormTemplateRepo{
		createResult: &repository.CreateDraftResult{
			ID:      templateID,
			Status:  domain.DraftStatus,
			Version: 1,
		},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates", bytes.NewReader(validDraftPayload()))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	req.Header.Set(userIDHeader, "00000000-0000-4000-8000-000000000001")
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(stub.lastCreateInput.Sections) != 1 || len(stub.lastCreateInput.Sections[0].Fields) != 1 {
		t.Fatalf("expected sections and fields passed to repo: %+v", stub.lastCreateInput.Sections)
	}
	if stub.lastCreateInput.Audit.ChangedByUserID == nil {
		t.Fatal("expected actor from X-User-ID")
	}
}

func TestAdminListReturnsDraft(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	stub := &stubAdminFormTemplateRepo{
		listItems: []domain.FormTemplateSummary{
			{
				ID:         uuid.New(),
				TenantID:   tenantID,
				EntityType: "TRANSPORT_ORDER",
				Code:       "transport_order_custom_v1",
				Name:       "Draft",
				Status:     domain.DraftStatus,
				Version:    1,
			},
		},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/admin/form-templates?status=DRAFT", nil)
	req.Header.Set(tenantHeader, tenantID.String())
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload struct {
		Items []struct {
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Status != domain.DraftStatus {
		t.Fatalf("expected draft item, got %+v", payload.Items)
	}
}

func TestPublicListDoesNotUseAdminRepo(t *testing.T) {
	publicStub := &stubFormTemplateRepo{
		listItems: []domain.FormTemplateSummary{
			{Status: domain.PublishedStatus, Code: "transport_order_default"},
		},
	}
	publicHandler := NewFormTemplateHandler(service.NewFormTemplateService(publicStub))
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/form-templates", nil)
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()

	publicHandler.List(rec, req)

	var payload struct {
		Items []struct {
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Status != domain.PublishedStatus {
		t.Fatalf("expected published only from public API")
	}
}

func TestAdminGetDraft(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	templateID := uuid.New()
	stub := &stubAdminFormTemplateRepo{
		getDetail: &domain.FormTemplateDetail{
			ID:         templateID,
			TenantID:   tenantID,
			EntityType: "TRANSPORT_ORDER",
			Code:       "transport_order_custom_v1",
			Name:       "Draft",
			Status:     domain.DraftStatus,
			Version:    1,
			Sections:   []domain.FormSection{},
		},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", templateID.String())
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/admin/form-templates/x", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.Header.Set(tenantHeader, tenantID.String())
	rec := httptest.NewRecorder()

	handler.GetByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateDraftSuccess(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	templateID := uuid.New()
	stub := &stubAdminFormTemplateRepo{
		getDetail: &domain.FormTemplateDetail{
			ID: templateID, TenantID: tenantID, Status: domain.DraftStatus, Sections: []domain.FormSection{},
		},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", templateID.String())
	req := httptest.NewRequest(http.MethodPut, "/v1/low-code/admin/form-templates/x", bytes.NewReader(validDraftPayload()))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.Header.Set(tenantHeader, tenantID.String())
	rec := httptest.NewRecorder()

	handler.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if stub.lastUpdateInput.TemplateID != templateID {
		t.Fatalf("expected update for template %s", templateID)
	}
}

func TestAdminUpdatePublishedBlocked(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{
		updateErr: apperrors.FormTemplateNotDraft(domain.PublishedStatus),
	}))
	templateID := uuid.New()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", templateID.String())
	req := httptest.NewRequest(http.MethodPut, "/v1/low-code/admin/form-templates/x", bytes.NewReader(validDraftPayload()))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()

	handler.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "FORM_TEMPLATE_NOT_DRAFT")
}

func TestAdminCreateDraftTenantIsolationInRepo(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	stub := &stubAdminFormTemplateRepo{
		createResult: &repository.CreateDraftResult{ID: uuid.New(), Status: domain.DraftStatus, Version: 1},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates", bytes.NewReader(validDraftPayload()))
	req.Header.Set(tenantHeader, tenantID.String())
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if stub.lastCreateInput.TenantID != tenantID {
		t.Fatalf("expected tenant %s, got %s", tenantID, stub.lastCreateInput.TenantID)
	}
}

func TestAdminListLimitMax(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{listItems: []domain.FormTemplateSummary{}}
	svc := service.NewAdminFormTemplateService(stub)
	items, err := svc.List(context.Background(), uuid.New(), "", "", 500)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if items == nil {
		t.Fatal("expected empty slice")
	}
}

func TestAdminCloneToDraftTenantRequired(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/x/clone-to-draft", nil)
	rec := httptest.NewRecorder()
	handler.CloneToDraft(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "TENANT_REQUIRED")
}

func TestAdminCloneToDraftPublishedSuccess(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	sourceID := uuid.New()
	draftID := uuid.New()
	stub := &stubAdminFormTemplateRepo{
		cloneResult: &repository.ClonePublishedToDraftResult{
			ID:               draftID,
			SourceTemplateID: sourceID,
			Status:           domain.DraftStatus,
			Version:          2,
			Code:             "transport_order_default",
		},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sourceID.String())
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/x/clone-to-draft", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.Header.Set(tenantHeader, tenantID.String())
	rec := httptest.NewRecorder()

	handler.CloneToDraft(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if stub.lastCloneTenantID != tenantID || stub.lastCloneSourceID != sourceID {
		t.Fatalf("expected clone tenant/source isolation")
	}
	var payload clonePublishedToDraftResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Status != domain.DraftStatus || payload.Version != 2 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestAdminCloneToDraftBlockedForDraftSource(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{
		cloneErr: apperrors.FormTemplateCloneSourceNotPublished(domain.DraftStatus),
	}))
	sourceID := uuid.New()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sourceID.String())
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/x/clone-to-draft", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()

	handler.CloneToDraft(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminCloneToDraftBlockedForArchivedSource(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{
		cloneErr: apperrors.FormTemplateCloneSourceNotPublished(domain.ArchivedStatus),
	}))
	sourceID := uuid.New()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sourceID.String())
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/x/clone-to-draft", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()

	handler.CloneToDraft(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminCloneToDraftTenantIsolation(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	sourceID := uuid.New()
	stub := &stubAdminFormTemplateRepo{
		cloneResult: &repository.ClonePublishedToDraftResult{
			ID: uuid.New(), SourceTemplateID: sourceID, Status: domain.DraftStatus, Version: 2, Code: "x",
		},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sourceID.String())
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/x/clone-to-draft", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.Header.Set(tenantHeader, tenantID.String())
	rec := httptest.NewRecorder()

	handler.CloneToDraft(rec, req)

	if stub.lastCloneTenantID != tenantID {
		t.Fatalf("expected tenant %s, got %s", tenantID, stub.lastCloneTenantID)
	}
}

func exportTemplateDetail(status string) *domain.FormTemplateDetail {
	sectionID := uuid.New()
	fieldID := uuid.New()
	templateID := uuid.MustParse("b1111111-1111-4111-8111-111111111102")
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	return &domain.FormTemplateDetail{
		ID:         templateID,
		TenantID:   tenantID,
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_default",
		Name:       "Transport Order Default",
		Status:     status,
		Version:    1,
		Sections: []domain.FormSection{
			{
				ID:        sectionID,
				Code:      "general",
				Title:     "General",
				SortOrder: 100,
				Fields: []domain.FormField{
					{
						ID:          fieldID,
						Code:        "cargo_class",
						Label:       "Cargo class",
						FieldType:   "SELECT",
						SortOrder:   100,
						OptionsJSON: json.RawMessage(`{"options":["GENERAL"]}`),
					},
				},
			},
		},
	}
}

func exportRequest(templateID uuid.UUID, tenantID uuid.UUID) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", templateID.String())
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/admin/form-templates/x/export", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.Header.Set(tenantHeader, tenantID.String())
	return req
}

func TestAdminExportTenantRequired(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	rec := httptest.NewRecorder()
	handler.Export(rec, httptest.NewRequest(http.MethodGet, "/v1/low-code/admin/form-templates/x/export", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "TENANT_REQUIRED")
}

func TestAdminExportDraftAllowed(t *testing.T) {
	detail := exportTemplateDetail(domain.DraftStatus)
	stub := &stubAdminFormTemplateRepo{getDetail: detail}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	rec := httptest.NewRecorder()
	handler.Export(rec, exportRequest(detail.ID, detail.TenantID))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload domain.TemplateExportEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.SchemaVersion != domain.TemplateExportSchemaVersion {
		t.Fatalf("schema_version = %q", payload.SchemaVersion)
	}
	if payload.Template.Status != domain.DraftStatus {
		t.Fatalf("expected draft status in export")
	}
	if !stub.exportRecorded {
		t.Fatal("expected export audit recorded")
	}
}

func TestAdminExportPublishedAllowed(t *testing.T) {
	detail := exportTemplateDetail(domain.PublishedStatus)
	stub := &stubAdminFormTemplateRepo{getDetail: detail}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	rec := httptest.NewRecorder()
	handler.Export(rec, exportRequest(detail.ID, detail.TenantID))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminExportWrongTenantRejected(t *testing.T) {
	detail := exportTemplateDetail(domain.PublishedStatus)
	stub := &stubAdminFormTemplateRepo{
		getErr: apperrors.FormTemplateNotFound(),
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	otherTenant := uuid.New()
	rec := httptest.NewRecorder()
	handler.Export(rec, exportRequest(detail.ID, otherTenant))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminExportMissingTemplateNotFound(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{getErr: apperrors.FormTemplateNotFound()}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()
	handler.Export(rec, exportRequest(uuid.New(), tenantID))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAdminExportPortableTemplateShape(t *testing.T) {
	detail := exportTemplateDetail(domain.PublishedStatus)
	sectionID := detail.Sections[0].ID
	fieldID := detail.Sections[0].Fields[0].ID
	stub := &stubAdminFormTemplateRepo{getDetail: detail}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	rec := httptest.NewRecorder()
	handler.Export(rec, exportRequest(detail.ID, detail.TenantID))

	body := rec.Body.String()
	if strings.Contains(body, sectionID.String()) || strings.Contains(body, fieldID.String()) {
		t.Fatalf("export must not include section/field DB ids: %s", body)
	}
	if strings.Contains(body, `"values"`) || strings.Contains(body, "audit") {
		t.Fatalf("export must not include custom values or audit events: %s", body)
	}

	var payload domain.TemplateExportEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Template.Code != "transport_order_default" {
		t.Fatalf("template.code = %q", payload.Template.Code)
	}
	if len(payload.Template.Sections) != 1 || len(payload.Template.Sections[0].Fields) != 1 {
		t.Fatalf("expected sections/fields in export: %+v", payload.Template)
	}
	if payload.Template.Sections[0].Fields[0].Code != "cargo_class" {
		t.Fatalf("expected field_code cargo_class")
	}
	if payload.Metadata.Checksum == "" {
		t.Fatal("expected checksum")
	}
	if stub.lastExportSchemaVersion != domain.TemplateExportSchemaVersion {
		t.Fatalf("audit schema version = %q", stub.lastExportSchemaVersion)
	}
}

func TestAdminExportDefaultOffCompatibility(t *testing.T) {
	detail := exportTemplateDetail(domain.PublishedStatus)
	stub := &stubAdminFormTemplateRepo{getDetail: detail}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	req := exportRequest(detail.ID, detail.TenantID)
	rec := httptest.NewRecorder()
	handler.Export(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 without X-User-ID, got %d", rec.Code)
	}
}

func validImportPreviewPayload() []byte {
	return []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"conflict_strategy":"NEW_VERSION",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Transport Order Default Form",
			"sections":[{
				"code":"cargo",
				"title":"Cargo",
				"sort_order":100,
				"fields":[{
					"code":"cargo_class",
					"label":"Cargo class",
					"field_type":"SELECT",
					"sort_order":100,
					"options_json":{"options":["GENERAL"]}
				}]
			}]
		}
	}`)
}

func TestAdminImportPreviewTenantRequired(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import-preview", bytes.NewReader(validImportPreviewPayload()))
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "TENANT_REQUIRED")
}

func TestAdminImportPreviewReadyWithPublishedExisting(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	templateID := uuid.MustParse("b1111111-1111-4111-8111-111111111102")
	stub := &stubAdminFormTemplateRepo{
		listByCodeItems: []domain.FormTemplateSummary{{
			ID: templateID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
			Code: "transport_order_default", Status: domain.PublishedStatus, Version: 1,
		}},
		getDetail: &domain.FormTemplateDetail{
			ID: templateID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER", Code: "transport_order_default",
			Status: domain.PublishedStatus, Version: 1,
			Sections: []domain.FormSection{{
				Code: "cargo", Title: "Cargo",
				Fields: []domain.FormField{{Code: "cargo_class", Label: "Cargo class", FieldType: "SELECT"}},
			}},
		},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import-preview", bytes.NewReader(validImportPreviewPayload()))
	req.Header.Set(tenantHeader, tenantID.String())
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload domain.TemplateImportPreviewResult
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.SchemaVersion != domain.TemplateExportSchemaVersion {
		t.Fatalf("schema_version = %q", payload.SchemaVersion)
	}
	if payload.Status != domain.ImportPreviewStatusReady {
		t.Fatalf("status = %q", payload.Status)
	}
	if payload.Summary.FieldsCount != 1 || payload.Summary.SectionsCount != 1 {
		t.Fatalf("summary = %+v", payload.Summary)
	}
	if !stub.importPreviewRecorded {
		t.Fatal("expected import preview audit")
	}
}

func TestAdminImportPreviewInvalidFieldType(t *testing.T) {
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"x","label":"X","field_type":"BAD"}]}]
		}
	}`)
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import-preview", bytes.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminImportPreviewFailIfExistsConflict(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{
		listByCodeItems: []domain.FormTemplateSummary{{
			Status: domain.PublishedStatus, Version: 1, Code: "transport_order_default", EntityType: "TRANSPORT_ORDER",
		}},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"conflict_strategy":"FAIL_IF_EXISTS",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"cargo_class","label":"Class","field_type":"SELECT"}]}]
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import-preview", bytes.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "FORM_TEMPLATE_CONFLICT")
}

func TestAdminImportPreviewUnsupportedSchemaVersion(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	body := []byte(`{
		"schema_version":"lowcode.template.export.v2",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"cargo_class","label":"Class","field_type":"SELECT"}]}]
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import-preview", bytes.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "UNSUPPORTED_SCHEMA_VERSION")
}

func TestAdminImportPreviewDefaultOffCompatibility(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{listByCodeItems: []domain.FormTemplateSummary{}}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import-preview", bytes.NewReader(validImportPreviewPayload()))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 without X-User-ID, got %d", rec.Code)
	}
}

func TestAdminImportTenantRequired(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import", bytes.NewReader(validImportPreviewPayload()))
	rec := httptest.NewRecorder()
	handler.Import(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "TENANT_REQUIRED")
}

func TestAdminImportCreatesDraft(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	templateID := uuid.MustParse("b1111111-1111-4111-8111-111111111102")
	stub := &stubAdminFormTemplateRepo{
		listByCodeItems: []domain.FormTemplateSummary{{
			ID: templateID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
			Code: "transport_order_default", Status: domain.PublishedStatus, Version: 1,
		}},
		getDetail: &domain.FormTemplateDetail{
			ID: templateID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER", Code: "transport_order_default",
			Status: domain.PublishedStatus, Version: 1,
			Sections: []domain.FormSection{{
				Code: "cargo", Title: "Cargo",
				Fields: []domain.FormField{{Code: "cargo_class", Label: "Cargo class", FieldType: "SELECT"}},
			}},
		},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import", bytes.NewReader(validImportPreviewPayload()))
	req.Header.Set(tenantHeader, tenantID.String())
	rec := httptest.NewRecorder()
	handler.Import(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload domain.TemplateImportExecuteResult
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Status != domain.DraftStatus {
		t.Fatalf("status = %q", payload.Status)
	}
	if payload.ImportSummary.ConflictStrategy != domain.ConflictStrategyNewVersion {
		t.Fatalf("conflict strategy = %q", payload.ImportSummary.ConflictStrategy)
	}
	if stub.lastImportInput.ReplaceDraftID != nil {
		t.Fatal("expected new draft import")
	}
}

func TestAdminImportReplaceExistingDraft(t *testing.T) {
	draftID := uuid.New()
	stub := &stubAdminFormTemplateRepo{
		listByCodeItems: []domain.FormTemplateSummary{{
			ID: draftID, Status: domain.DraftStatus, Version: 2, Code: "transport_order_default", EntityType: "TRANSPORT_ORDER",
		}},
		importResult: &repository.ImportTemplateAsDraftResult{
			ID: draftID, Status: domain.DraftStatus, Version: 2, Code: "transport_order_default", ReplacedDraft: true,
		},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"mode":"REPLACE_EXISTING_DRAFT",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Updated Draft",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"cargo_class","label":"Class","field_type":"SELECT"}]}]
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import", bytes.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()
	handler.Import(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if stub.lastImportInput.ReplaceDraftID == nil || *stub.lastImportInput.ReplaceDraftID != draftID {
		t.Fatal("expected replace draft id passed to repo")
	}
}

func TestAdminImportFailIfExistsConflict(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{
		listByCodeItems: []domain.FormTemplateSummary{{
			Status: domain.PublishedStatus, Version: 1, Code: "transport_order_default", EntityType: "TRANSPORT_ORDER",
		}},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"conflict_strategy":"FAIL_IF_EXISTS",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"cargo_class","label":"Class","field_type":"SELECT"}]}]
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import", bytes.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()
	handler.Import(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminImportDefaultOffCompatibility(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{listByCodeItems: []domain.FormTemplateSummary{}}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import", bytes.NewReader(validImportPreviewPayload()))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()
	handler.Import(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 without X-User-ID, got %d", rec.Code)
	}
}
