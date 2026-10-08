package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type analyticsSourceStub struct {
	tenant uuid.UUID
	snap   domain.OperationsAnalyticsSourceSnapshot
	err    error
}

func (s *analyticsSourceStub) OperationsFoundation(_ context.Context, tenantID uuid.UUID) (domain.OperationsAnalyticsSourceSnapshot, error) {
	s.tenant = tenantID
	if s.err != nil {
		return domain.OperationsAnalyticsSourceSnapshot{}, s.err
	}
	return s.snap, nil
}

func TestOperationsAnalyticsSourceServiceUsesVerifiedTenant(t *testing.T) {
	tenant := uuid.New()
	stub := &analyticsSourceStub{snap: domain.OperationsAnalyticsSourceSnapshot{
		TenantID: uuid.New(), ShipmentTotal: 4, OnTimeDeliveryDenominator: 2, OnTimeDeliveryNumerator: 1, ReturnCaseCount: 3, RedirectCaseCount: 1,
	}}
	svc := NewOperationsAnalyticsSourceService(stub)
	snap, err := svc.OperationsFoundation(context.Background(), tenant)
	if err != nil {
		t.Fatal(err)
	}
	if stub.tenant != tenant || snap.TenantID != tenant {
		t.Fatalf("tenant %s snap %s", stub.tenant, snap.TenantID)
	}
	if snap.ShipmentTotal != 4 || snap.OnTimeDeliveryNumerator != 1 || snap.ReturnCaseCount != 3 {
		t.Fatalf("%+v", snap)
	}
}

func TestOperationsAnalyticsSourceServiceRejectsInconsistentSnapshot(t *testing.T) {
	stub := &analyticsSourceStub{snap: domain.OperationsAnalyticsSourceSnapshot{
		ShipmentTotal: 1, OnTimeDeliveryDenominator: 2, OnTimeDeliveryNumerator: 2,
	}}
	_, err := NewOperationsAnalyticsSourceService(stub).OperationsFoundation(context.Background(), uuid.New())
	app, ok := err.(*apperrors.AppError)
	if !ok || app.Code != apperrors.CodeInternal {
		t.Fatalf("%v", err)
	}
}

func TestOperationsAnalyticsSourceServiceRejectsPickupNumeratorAboveDenominator(t *testing.T) {
	stub := &analyticsSourceStub{snap: domain.OperationsAnalyticsSourceSnapshot{
		ShipmentTotal: 2, OnTimePickupDenominator: 1, OnTimePickupNumerator: 2,
	}}
	_, err := NewOperationsAnalyticsSourceService(stub).OperationsFoundation(context.Background(), uuid.New())
	app, ok := err.(*apperrors.AppError)
	if !ok || app.Code != apperrors.CodeInternal {
		t.Fatalf("%v", err)
	}
}

func TestOperationsAnalyticsSourceServiceRejectsCarrierNumeratorAboveDenominator(t *testing.T) {
	stub := &analyticsSourceStub{snap: domain.OperationsAnalyticsSourceSnapshot{
		ShipmentTotal: 1,
		Carriers: []domain.OperationsAnalyticsCarrierSource{{
			CarrierCompanyID: uuid.New(), OnTimeDeliveryDenominator: 1, OnTimeDeliveryNumerator: 2,
		}},
	}}
	_, err := NewOperationsAnalyticsSourceService(stub).OperationsFoundation(context.Background(), uuid.New())
	app, ok := err.(*apperrors.AppError)
	if !ok || app.Code != apperrors.CodeInternal {
		t.Fatalf("%v", err)
	}
}

func TestOperationsAnalyticsSourceServiceRejectsNilCarrier(t *testing.T) {
	stub := &analyticsSourceStub{snap: domain.OperationsAnalyticsSourceSnapshot{
		ShipmentTotal: 1,
		Carriers:      []domain.OperationsAnalyticsCarrierSource{{CarrierCompanyID: uuid.Nil}},
	}}
	_, err := NewOperationsAnalyticsSourceService(stub).OperationsFoundation(context.Background(), uuid.New())
	app, ok := err.(*apperrors.AppError)
	if !ok || app.Code != apperrors.CodeInternal {
		t.Fatalf("%v", err)
	}
}

func TestOperationsAnalyticsSourceServiceRejectsDuplicateCarrier(t *testing.T) {
	carrier := uuid.New()
	stub := &analyticsSourceStub{snap: domain.OperationsAnalyticsSourceSnapshot{
		ShipmentTotal: 2,
		Carriers: []domain.OperationsAnalyticsCarrierSource{
			{CarrierCompanyID: carrier},
			{CarrierCompanyID: carrier},
		},
	}}
	_, err := NewOperationsAnalyticsSourceService(stub).OperationsFoundation(context.Background(), uuid.New())
	app, ok := err.(*apperrors.AppError)
	if !ok || app.Code != apperrors.CodeInternal {
		t.Fatalf("%v", err)
	}
}

func TestOperationsAnalyticsSourceServiceRequiresTenant(t *testing.T) {
	_, err := NewOperationsAnalyticsSourceService(&analyticsSourceStub{}).OperationsFoundation(context.Background(), uuid.Nil)
	app, ok := err.(*apperrors.AppError)
	if !ok || app.Code != apperrors.CodeUnauthorized {
		t.Fatalf("%v", err)
	}
}
