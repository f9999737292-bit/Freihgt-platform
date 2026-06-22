package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

type mockFormTemplateRepo struct {
	listItems []domain.FormTemplateSummary
	listErr   error
	getDetail *domain.FormTemplateDetail
	getErr    error
	lastTenant uuid.UUID
	lastEntity string
}

func (m *mockFormTemplateRepo) ListPublished(ctx context.Context, tenantID uuid.UUID, entityType string) ([]domain.FormTemplateSummary, error) {
	m.lastTenant = tenantID
	m.lastEntity = entityType
	return m.listItems, m.listErr
}

func (m *mockFormTemplateRepo) GetPublishedByID(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.FormTemplateDetail, error) {
	m.lastTenant = tenantID
	return m.getDetail, m.getErr
}

func (m *mockFormTemplateRepo) GetPublishedByCode(ctx context.Context, tenantID uuid.UUID, code string) (*domain.FormTemplateDetail, error) {
	m.lastTenant = tenantID
	return m.getDetail, m.getErr
}

func TestListPublishedInvalidEntityType(t *testing.T) {
	svc := NewFormTemplateService(&mockFormTemplateRepo{})
	_, err := svc.ListPublished(context.Background(), uuid.New(), "INVALID")
	if err == nil {
		t.Fatal("expected validation error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected VALIDATION_ERROR, got %v", err)
	}
}

func TestListPublishedFiltersByTenant(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	repo := &mockFormTemplateRepo{
		listItems: []domain.FormTemplateSummary{
			{ID: uuid.New(), TenantID: tenantID, EntityType: "TRANSPORT_ORDER", Code: "transport_order_default", Status: domain.PublishedStatus},
		},
	}
	svc := NewFormTemplateService(repo)
	items, err := svc.ListPublished(context.Background(), tenantID, "TRANSPORT_ORDER")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if repo.lastTenant != tenantID {
		t.Fatalf("tenant not forwarded to repository")
	}
}
