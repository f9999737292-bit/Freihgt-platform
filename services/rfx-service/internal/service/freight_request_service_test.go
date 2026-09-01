package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type mockFreightRequestListStore struct {
	listFn    func(ctx context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error)
	getByIDFn func(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error)
}

func (m *mockFreightRequestListStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockFreightRequestListStore) GetTransportOrder(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", nil
}
func (m *mockFreightRequestListStore) CreateFromTransportOrder(context.Context, domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
	return nil, nil
}
func (m *mockFreightRequestListStore) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.FreightRequest, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id, tenantID)
	}
	return nil, apperrors.NotFound("freight request not found")
}
func (m *mockFreightRequestListStore) List(ctx context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
	if m.listFn != nil {
		return m.listFn(ctx, filter)
	}
	return nil, 0, nil
}
func (m *mockFreightRequestListStore) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, string, string) (*domain.FreightRequest, error) {
	return nil, nil
}

func sampleFR(id, tenantID, shipperID uuid.UUID, status string) *domain.FreightRequest {
	now := time.Now().UTC()
	return &domain.FreightRequest{
		ID:                   id,
		TenantID:             tenantID,
		FreightRequestNumber: "FR-1",
		RequestType:          "SPOT",
		ShipperCompanyID:     shipperID,
		Status:               status,
		CreatedAt:            now,
		UpdatedAt:            now,
		Version:              1,
	}
}

func TestFreightRequestServiceBuyerListOwnCompanyScope(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	shipperA := uuid.New()
	shipperB := uuid.New()
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{
		listFn: func(_ context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
			if filter.ShipperCompanyID != nil && *filter.ShipperCompanyID == shipperB {
				if len(filter.ShipperCompanyIDs) != 1 || filter.ShipperCompanyIDs[0] != uuid.Nil {
					t.Fatalf("expected empty buyer scope for foreign shipper, got %#v", filter.ShipperCompanyIDs)
				}
				return nil, 0, nil
			}
			if len(filter.ShipperCompanyIDs) != 1 || filter.ShipperCompanyIDs[0] != shipperA {
				t.Fatalf("expected buyer scope shipperA, got %#v", filter.ShipperCompanyIDs)
			}
			return []domain.FreightRequest{*sampleFR(uuid.New(), tenantID, shipperA, domain.FreightRequestStatusDraft)}, 1, nil
		},
	}, buyerMembershipResolver(shipperA))
	items, total, err := svc.List(context.Background(), buyerTestActor(tenantID, userID, shipperA), domain.ListFreightRequestsFilter{})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("unexpected list result items=%v total=%d err=%v", items, total, err)
	}
	_, _, err = svc.List(context.Background(), buyerTestActor(tenantID, userID, shipperA), domain.ListFreightRequestsFilter{
		ShipperCompanyID: &shipperB,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFreightRequestServiceBuyerGetOwnAndForeignCompany(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	shipperA := uuid.New()
	shipperB := uuid.New()
	frID := uuid.New()
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.FreightRequest, error) {
			return sampleFR(id, tenant, shipperB, domain.FreightRequestStatusDraft), nil
		},
	}, buyerMembershipResolver(shipperA))
	_, err := svc.GetByID(context.Background(), buyerTestActor(tenantID, userID, shipperA), frID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestFreightRequestServiceBuyerGetOwnRequest(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	shipperA := uuid.New()
	frID := uuid.New()
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.FreightRequest, error) {
			return sampleFR(id, tenant, shipperA, domain.FreightRequestStatusDraft), nil
		},
	}, buyerMembershipResolver(shipperA))
	fr, err := svc.GetByID(context.Background(), buyerTestActor(tenantID, userID, shipperA), frID)
	if err != nil || fr == nil {
		t.Fatalf("expected own draft readable, got fr=%v err=%v", fr, err)
	}
}

func TestFreightRequestServiceCarrierListVisibleStatuses(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	carrierID := uuid.New()
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{
		listFn: func(_ context.Context, filter domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
			if filter.ShipperCompanyID != nil || len(filter.ShipperCompanyIDs) > 0 {
				t.Fatal("carrier list must not apply buyer shipper filters")
			}
			if filter.Status != nil {
				t.Fatalf("expected default statuses filter, got status=%v", filter.Status)
			}
			if len(filter.Statuses) != 2 {
				t.Fatalf("expected visible statuses, got %#v", filter.Statuses)
			}
			return []domain.FreightRequest{
				*sampleFR(uuid.New(), tenantID, uuid.New(), domain.FreightRequestStatusPublished),
				*sampleFR(uuid.New(), tenantID, uuid.New(), domain.FreightRequestStatusResponsesOpen),
			}, 2, nil
		},
	}, carrierMembershipResolver(carrierID))
	items, total, err := svc.List(context.Background(), carrierTestActor(tenantID, userID), domain.ListFreightRequestsFilter{})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("unexpected carrier list items=%v total=%d err=%v", items, total, err)
	}
}

func TestFreightRequestServiceCarrierListDraftStatusQueryEmpty(t *testing.T) {
	t.Parallel()
	called := false
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{
		listFn: func(context.Context, domain.ListFreightRequestsFilter) ([]domain.FreightRequest, int, error) {
			called = true
			return nil, 0, nil
		},
	}, carrierMembershipResolver(uuid.New()))
	status := domain.FreightRequestStatusDraft
	items, total, err := svc.List(context.Background(), carrierTestActor(uuid.New(), uuid.New()), domain.ListFreightRequestsFilter{Status: &status})
	if err != nil || total != 0 || len(items) != 0 || called {
		t.Fatalf("expected empty without repo call called=%v items=%v total=%d err=%v", called, items, total, err)
	}
}

func TestFreightRequestServiceCarrierGetPublishedAndResponsesOpen(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	carrierID := uuid.New()
	frID := uuid.New()
	for _, status := range []string{domain.FreightRequestStatusPublished, domain.FreightRequestStatusResponsesOpen} {
		status := status
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			svc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{
				getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.FreightRequest, error) {
					return sampleFR(id, tenant, uuid.New(), status), nil
				},
			}, carrierMembershipResolver(carrierID))
			fr, err := svc.GetByID(context.Background(), carrierTestActor(tenantID, userID), frID)
			if err != nil || fr == nil || fr.Status != status {
				t.Fatalf("expected readable %s, got fr=%v err=%v", status, fr, err)
			}
		})
	}
}

func TestFreightRequestServiceCarrierGetDraftReturnsNotFound(t *testing.T) {
	t.Parallel()
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.FreightRequest, error) {
			return sampleFR(id, tenant, uuid.New(), domain.FreightRequestStatusDraft), nil
		},
	}, carrierMembershipResolver(uuid.New()))
	_, err := svc.GetByID(context.Background(), carrierTestActor(uuid.New(), uuid.New()), uuid.New())
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestFreightRequestServiceCarrierGetForeignTenantNotFound(t *testing.T) {
	t.Parallel()
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return nil, apperrors.NotFound("freight request not found")
		},
	}, carrierMembershipResolver(uuid.New()))
	_, err := svc.GetByID(context.Background(), carrierTestActor(uuid.New(), uuid.New()), uuid.New())
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestFreightRequestServiceUnknownActorDenied(t *testing.T) {
	t.Parallel()
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{}, &mockActorResolver{kind: domain.ActorKindUnknown})
	_, _, err := svc.List(context.Background(), domain.ActorContext{TenantID: uuid.New(), UserID: uuid.New()}, domain.ListFreightRequestsFilter{})
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestFreightRequestServiceCarrierWithoutMembershipDenied(t *testing.T) {
	t.Parallel()
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{}, carrierMembershipResolver())
	_, _, err := svc.List(context.Background(), carrierTestActor(uuid.New(), uuid.New()), domain.ListFreightRequestsFilter{})
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestFreightRequestServiceCarrierPublishDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	shipperID := uuid.New()
	frID := uuid.New()
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.FreightRequest, error) {
			return sampleFR(id, tenant, shipperID, domain.FreightRequestStatusDraft), nil
		},
	}, carrierMembershipResolver(uuid.New()))
	_, err := svc.Publish(context.Background(), carrierTestActor(tenantID, uuid.New()), frID)
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestFreightRequestServiceCarrierReadToBidFlow(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	carrierID := uuid.New()
	frID := uuid.New()
	deadline := time.Now().UTC().Add(time.Hour)

	frSvc := NewFreightRequestServiceWithAuth(&mockFreightRequestListStore{
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{
				ID: id, TenantID: tenant, Status: domain.FreightRequestStatusPublished,
				ResponseDeadline: &deadline, ShipperCompanyID: uuid.New(),
			}, nil
		},
	}, carrierMembershipResolver(carrierID))
	fr, err := frSvc.GetByID(context.Background(), carrierTestActor(tenantID, userID), frID)
	if err != nil {
		t.Fatalf("carrier read failed: %v", err)
	}

	bidID := uuid.New()
	bidSvc := NewBidService(&mockBidStore{
		createFn: func(_ context.Context, in domain.CreateBidInput) (*domain.Bid, error) {
			if in.CarrierCompanyID != carrierID {
				t.Fatalf("expected carrier %s, got %s", carrierID, in.CarrierCompanyID)
			}
			return &domain.Bid{ID: bidID, Status: domain.BidStatusDraft, CarrierCompanyID: carrierID, FreightRequestID: frID}, nil
		},
		getByIDFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.Bid, error) {
			return &domain.Bid{ID: id, Status: domain.BidStatusDraft, CarrierCompanyID: carrierID, FreightRequestID: frID, TenantID: tenant}, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return fr, nil
		},
	}, &mockActorResolver{kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{carrierID}}, nil)

	_, err = bidSvc.CreateBid(context.Background(), carrierTestActor(tenantID, userID), frID, domain.CreateBidInput{
		TenantID: tenantID, BidNumber: "BID-1", Items: []domain.CreateBidItemInput{{BaseAmount: 100}},
	})
	if err != nil {
		t.Fatalf("create bid failed: %v", err)
	}
	_, err = bidSvc.SubmitBid(context.Background(), carrierTestActor(tenantID, userID), bidID)
	if err != nil {
		t.Fatalf("submit bid failed: %v", err)
	}
}

func TestFreightRequestServiceCarrierCreateFreightRequestDenied(t *testing.T) {
	t.Parallel()
	svc := NewFreightRequestServiceWithAuth(&mockFreightRequestStore{
		getTransportOrderFn: func(context.Context, uuid.UUID, uuid.UUID) (string, error) {
			return domain.TransportOrderStatusReadyForSourcing, nil
		},
		createFn: func(context.Context, domain.CreateFreightRequestFromOrderInput) (*domain.FreightRequest, error) {
			t.Fatal("create must not be called for carrier")
			return nil, nil
		},
	}, carrierMembershipResolver(uuid.New()))
	_, err := svc.CreateFromTransportOrder(context.Background(), carrierTestActor(uuid.New(), uuid.New()), domain.CreateFreightRequestFromOrderInput{
		TenantID: uuid.New(), TransportOrderID: uuid.New(), FreightRequestNumber: "FR-1",
		RequestType: "SPOT", ShipperCompanyID: uuid.New(),
	})
	assertAppErrorCode(t, err, apperrors.CodeForbidden)
}

func TestFreightRequestServiceCarrierAcceptBidDenied(t *testing.T) {
	t.Parallel()
	bidSvc := NewBidService(&mockBidStore{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Bid, error) {
			return &domain.Bid{Status: domain.BidStatusSubmitted, FreightRequestID: uuid.New()}, nil
		},
	}, &mockFreightRequestStoreForBid{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.FreightRequest, error) {
			return &domain.FreightRequest{Status: domain.FreightRequestStatusPublished, ShipperCompanyID: uuid.New()}, nil
		},
	}, &mockActorResolver{kind: domain.ActorKindCarrier, carrierIDs: []uuid.UUID{uuid.New()}}, nil)
	_, err := bidSvc.AcceptBid(context.Background(), carrierTestActor(uuid.New(), uuid.New()), uuid.New())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected forbidden for carrier accept, got %v", err)
	}
}
