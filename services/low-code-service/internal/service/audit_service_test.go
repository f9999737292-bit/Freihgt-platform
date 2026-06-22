package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
)

type stubAuditServiceStore struct {
	lastFilter domain.ListAuditEventsFilter
}

func (s *stubAuditServiceStore) List(ctx context.Context, filter domain.ListAuditEventsFilter) ([]domain.ConfigurationAuditEntry, error) {
	s.lastFilter = filter
	return nil, nil
}

func TestAuditServiceEnforcesLimit(t *testing.T) {
	store := &stubAuditServiceStore{}
	svc := NewAuditService(store)
	_, err := svc.List(context.Background(), domain.ListAuditEventsFilter{
		TenantID: uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f"),
		Limit:    250,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.lastFilter.Limit != 100 {
		t.Fatalf("expected capped limit 100, got %d", store.lastFilter.Limit)
	}
}

func TestAuditServiceRequiresTenant(t *testing.T) {
	svc := NewAuditService(&stubAuditServiceStore{})
	_, err := svc.List(context.Background(), domain.ListAuditEventsFilter{})
	if err == nil {
		t.Fatalf("expected tenant required error")
	}
}
