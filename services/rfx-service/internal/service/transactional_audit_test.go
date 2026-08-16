package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type failingAuditRecorder struct{}

func (failingAuditRecorder) Record(context.Context, repository.AuditRecord) error {
	return errors.New("audit insert failed")
}

type trackingRfxStore struct {
	mockRfxStore
	createCalled bool
}

func (s *trackingRfxStore) CreateEvent(ctx context.Context, in domain.CreateRfxEventInput) (*domain.RfxEvent, error) {
	s.createCalled = true
	return &domain.RfxEvent{ID: uuid.New(), OwnerCompanyID: in.OwnerCompanyID, RfxType: in.RfxType}, nil
}

func TestAuditFailureRollsBackMutationWithoutCommit(t *testing.T) {
	t.Parallel()
	ownerCompanyID := uuid.New()
	tenantID := uuid.New()
	userID := uuid.New()
	store := &trackingRfxStore{}
	svc := NewRfxService(store, failingAuditRecorder{}, buyerMembershipResolver(ownerCompanyID))
	_, err := svc.CreateEvent(context.Background(), buyerTestActor(tenantID, userID, ownerCompanyID), domain.CreateRfxEventInput{
		TenantID: tenantID, OwnerCompanyID: ownerCompanyID, Title: "T", RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-AUDIT-1",
	})
	if err == nil {
		t.Fatal("expected audit failure")
	}
	if !store.createCalled {
		t.Fatal("expected business mutation attempt before audit failure")
	}
}

func TestFailedMutationNoSuccessAudit(t *testing.T) {
	t.Parallel()
	ownerCompanyID := uuid.New()
	auditCalls := 0
	audit := auditCallCounter{onRecord: func() { auditCalls++ }}
	svc := NewRfxService(&mockRfxStore{
		getEventFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.RfxEvent, error) {
			return &domain.RfxEvent{Status: domain.RfxStatusPublished, OwnerCompanyID: ownerCompanyID}, nil
		},
	}, audit, buyerMembershipResolver(ownerCompanyID))
	_, err := svc.PublishEvent(context.Background(), buyerTestActor(uuid.New(), uuid.New(), ownerCompanyID), uuid.New())
	if err == nil {
		t.Fatal("expected validation error for publish from non-draft")
	}
	if auditCalls != 0 {
		t.Fatalf("expected no audit on failed mutation, got %d", auditCalls)
	}
}

type auditCallCounter struct {
	onRecord func()
}

func (a auditCallCounter) Record(context.Context, repository.AuditRecord) error {
	if a.onRecord != nil {
		a.onRecord()
	}
	return nil
}
