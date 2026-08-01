package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestShipmentServiceRejectsUserContextWithoutActorIDBeforeRepository(t *testing.T) {
	t.Parallel()
	called := false
	tenantID := uuid.New()
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			called = true
			return &domain.Shipment{Status: domain.ShipmentStatusCarrierAssigned, Version: 1}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.UpdateStatus(context.Background(), tenantID, uuid.New(), domain.UpdateShipmentStatusInput{
		Status: domain.ShipmentStatusAcceptedByCarrier,
	}, domain.StatusTransitionContext{
		ActorType:  domain.ActorTypeUser,
		Source:     domain.StatusHistorySourceShipmentService,
		OccurredAt: time.Now().UTC(),
	})
	if called {
		t.Fatal("repository must not be called when USER actor_id is missing")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestShipmentServiceRejectsUserContextWithZeroActorID(t *testing.T) {
	t.Parallel()
	zero := uuid.Nil
	svc := NewShipmentService(&mockShipmentStore{}, &mockDriverLookup{}, &mockVehicleLookup{})
	_, err := svc.Cancel(context.Background(), uuid.New(), uuid.New(), domain.CancelShipmentInput{}, domain.StatusTransitionContext{
		ActorType:  domain.ActorTypeUser,
		ActorID:    &zero,
		Source:     domain.StatusHistorySourceShipmentService,
		OccurredAt: time.Now().UTC(),
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestShipmentServiceAllowsExplicitSystemContextForServerSideCaller(t *testing.T) {
	t.Parallel()
	called := false
	tenantID := uuid.New()
	svc := NewShipmentService(&mockShipmentStore{
		getTransportOrderFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.TransportOrderSnapshot, error) {
			return &domain.TransportOrderSnapshot{
				Status:           domain.TransportOrderStatusAssigned,
				ShipperCompanyID: uuid.New(), ConsigneeCompanyID: uuid.New(),
				OriginLocationID: uuid.New(), DestinationLocationID: uuid.New(),
				TransportMode: "ROAD",
			}, nil
		},
		createFn: func(_ context.Context, params repository.CreateShipmentParams, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			if params.TenantID != tenantID {
				t.Fatalf("tenant=%s want %s", params.TenantID, tenantID)
			}
			return &domain.Shipment{Status: domain.ShipmentStatusCarrierAssigned}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.CreateFromTransportOrder(context.Background(), tenantID, domain.CreateShipmentFromOrderInput{
		ShipmentNumber: "SHP-SYS", TransportOrderID: uuid.New(), CarrierCompanyID: uuid.New(),
	}, domain.NewSystemTransitionContext(domain.StatusHistorySourceShipmentService, nil, time.Now().UTC()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("explicit SYSTEM context must reach repository for server-side callers")
	}
}

func TestShipmentServiceRejectsMissingTenantBeforeRepository(t *testing.T) {
	t.Parallel()
	called := false
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.Accept(context.Background(), uuid.Nil, uuid.New(), testUserTransition())
	if called {
		t.Fatal("repository must not be called without verified tenant")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestShipmentServiceCreateUsesVerifiedTenantForOrderLookup(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	orderID := uuid.New()
	var lookupTenant uuid.UUID
	svc := NewShipmentService(&mockShipmentStore{
		getTransportOrderFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.TransportOrderSnapshot, error) {
			lookupTenant = tenant
			if id != orderID || tenant != tenantID {
				return nil, apperrors.NotFound("transport order not found")
			}
			return &domain.TransportOrderSnapshot{
				Status:           domain.TransportOrderStatusAssigned,
				ShipperCompanyID: uuid.New(), ConsigneeCompanyID: uuid.New(),
				OriginLocationID: uuid.New(), DestinationLocationID: uuid.New(),
				TransportMode: "ROAD",
			}, nil
		},
		createFn: func(_ context.Context, params repository.CreateShipmentParams, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			if params.TenantID != tenantID {
				t.Fatalf("create tenant=%s want %s", params.TenantID, tenantID)
			}
			return &domain.Shipment{Status: domain.ShipmentStatusCarrierAssigned}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.CreateFromTransportOrder(context.Background(), tenantID, domain.CreateShipmentFromOrderInput{
		ShipmentNumber: "SHP-1", TransportOrderID: orderID, CarrierCompanyID: uuid.New(),
	}, testUserTransition())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lookupTenant != tenantID {
		t.Fatalf("order lookup tenant=%s want %s", lookupTenant, tenantID)
	}
}

func TestShipmentServiceCreateFromBidUsesVerifiedTenantForBidLookup(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	bidID := uuid.New()
	var lookupTenant uuid.UUID
	svc := NewShipmentService(&mockShipmentStore{
		getBidFn: func(_ context.Context, id, tenant uuid.UUID) (*domain.BidSnapshot, error) {
			lookupTenant = tenant
			if id != bidID || tenant != tenantID {
				return nil, apperrors.NotFound("bid not found")
			}
			return &domain.BidSnapshot{
				Status: domain.BidStatusAccepted, CarrierCompanyID: uuid.New(),
			}, nil
		},
		getTransportOrderFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.TransportOrderSnapshot, error) {
			return &domain.TransportOrderSnapshot{
				Status:           domain.TransportOrderStatusAssigned,
				ShipperCompanyID: uuid.New(), ConsigneeCompanyID: uuid.New(),
				OriginLocationID: uuid.New(), DestinationLocationID: uuid.New(),
				TransportMode: "ROAD",
			}, nil
		},
		createFn: func(_ context.Context, params repository.CreateShipmentParams, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			if params.TenantID != tenantID {
				t.Fatalf("create tenant=%s want %s", params.TenantID, tenantID)
			}
			return &domain.Shipment{Status: domain.ShipmentStatusCarrierAssigned}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.CreateFromBid(context.Background(), tenantID, domain.CreateShipmentFromBidInput{
		ShipmentNumber: "SHP-BID", BidID: bidID, TransportOrderID: uuid.New(),
	}, testUserTransition())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lookupTenant != tenantID {
		t.Fatalf("bid lookup tenant=%s want %s", lookupTenant, tenantID)
	}
}

func TestShipmentServiceForeignTransportOrderReturns404(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	called := false
	svc := NewShipmentService(&mockShipmentStore{
		getTransportOrderFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.TransportOrderSnapshot, error) {
			return nil, apperrors.NotFound("transport order not found")
		},
		createFn: func(context.Context, repository.CreateShipmentParams, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.CreateFromTransportOrder(context.Background(), tenantID, domain.CreateShipmentFromOrderInput{
		ShipmentNumber: "SHP-1", TransportOrderID: uuid.New(), CarrierCompanyID: uuid.New(),
	}, testUserTransition())
	if called {
		t.Fatal("create must not run when order lookup fails")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestShipmentServiceForeignBidReturns404(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	called := false
	svc := NewShipmentService(&mockShipmentStore{
		getBidFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BidSnapshot, error) {
			return nil, apperrors.NotFound("bid not found")
		},
		createFn: func(context.Context, repository.CreateShipmentParams, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.CreateFromBid(context.Background(), tenantID, domain.CreateShipmentFromBidInput{
		ShipmentNumber: "SHP-2", BidID: uuid.New(), TransportOrderID: uuid.New(),
	}, testUserTransition())
	if called {
		t.Fatal("create must not run when bid lookup fails")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestShipmentServiceAcceptForeignShipmentReturns404(t *testing.T) {
	t.Parallel()
	called := false
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return nil, apperrors.NotFound("shipment not found")
		},
		acceptFn: func(context.Context, uuid.UUID, uuid.UUID, string, int, domain.StatusTransitionContext) (*domain.Shipment, error) {
			called = true
			return nil, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.Accept(context.Background(), uuid.New(), uuid.New(), testUserTransition())
	if called {
		t.Fatal("accept repository must not run for foreign shipment")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestShipmentServiceUpdateStatusPassesVerifiedTenantToRepository(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	var repoTenant uuid.UUID
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: shipmentID, TenantID: tenantID,
				Status: domain.ShipmentStatusAcceptedByCarrier, Version: 3,
			}, nil
		},
		updateStatusFn: func(_ context.Context, id, tenant uuid.UUID, _, newStatus string, _, _ *time.Time, expectedVersion int, _ domain.StatusTransitionContext) (*domain.Shipment, error) {
			repoTenant = tenant
			if id != shipmentID || expectedVersion != 3 || newStatus != domain.ShipmentStatusVehicleAssigned {
				t.Fatalf("unexpected update args id=%s version=%d status=%s", id, expectedVersion, newStatus)
			}
			return &domain.Shipment{Status: newStatus, Version: 4}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.UpdateStatus(context.Background(), tenantID, shipmentID, domain.UpdateShipmentStatusInput{
		Status: domain.ShipmentStatusVehicleAssigned,
	}, testUserTransition())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repoTenant != tenantID {
		t.Fatalf("repo tenant=%s want %s", repoTenant, tenantID)
	}
}

func TestShipmentServiceCancelPassesVerifiedTenantToRepository(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	var repoTenant uuid.UUID
	svc := NewShipmentService(&mockShipmentStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Shipment, error) {
			return &domain.Shipment{
				ID: shipmentID, TenantID: tenantID,
				Status: domain.ShipmentStatusInTransit, Version: 2,
			}, nil
		},
		cancelFn: func(_ context.Context, id, tenant uuid.UUID, _ string, expectedVersion int, transition domain.StatusTransitionContext) (*domain.Shipment, error) {
			repoTenant = tenant
			if id != shipmentID || expectedVersion != 2 {
				t.Fatalf("unexpected cancel args id=%s version=%d", id, expectedVersion)
			}
			if transition.ReasonCode == nil || *transition.ReasonCode != "CUSTOMER_REQUEST" {
				t.Fatalf("reason_code=%v", transition.ReasonCode)
			}
			return &domain.Shipment{Status: domain.ShipmentStatusCancelled, Version: 3}, nil
		},
	}, &mockDriverLookup{}, &mockVehicleLookup{})

	_, err := svc.Cancel(context.Background(), tenantID, shipmentID, domain.CancelShipmentInput{
		Reason: "CUSTOMER_REQUEST",
	}, testUserTransition())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repoTenant != tenantID {
		t.Fatalf("repo tenant=%s want %s", repoTenant, tenantID)
	}
}
