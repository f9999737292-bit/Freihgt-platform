package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type fakePODStore struct {
	load domain.PODRequirementLoad
	err  error
}

func (f fakePODStore) LoadPODRequirement(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.PODRequirementLoad, error) {
	return f.load, f.err
}

func TestGetPODRequirementMissingRowIsUnknown(t *testing.T) {
	subject := domain.PODRequirementSubject{
		OperatingTenantID: uuid.New(), ExecutionID: uuid.New(), ExecutionActionID: uuid.New(),
		ExecutionStopID: uuid.New(), ShipmentID: uuid.New(), CargoID: uuid.New(), ActionType: domain.ActionTypeDelivery,
	}
	svc := NewPODRequirementService(fakePODStore{load: domain.PODRequirementLoad{Subject: subject}})
	view, err := svc.GetPODRequirement(context.Background(), subject.OperatingTenantID, subject.ExecutionID, subject.ExecutionActionID)
	if err != nil {
		t.Fatal(err)
	}
	if view.DecisionState != domain.PODDecisionStateUnknown || view.PodRequired != nil || view.DecisionID != nil {
		t.Fatalf("view = %+v", view)
	}
}

func TestGetPODRequirementPickupConflicts(t *testing.T) {
	subject := domain.PODRequirementSubject{ActionType: domain.ActionTypePickup, ExecutionActionID: uuid.New()}
	svc := NewPODRequirementService(fakePODStore{load: domain.PODRequirementLoad{Subject: subject}})
	_, err := svc.GetPODRequirement(context.Background(), uuid.New(), uuid.New(), subject.ExecutionActionID)
	var app *apperrors.AppError
	if err == nil || !asApp(err, &app) || app.Code != apperrors.CodeConflict || app.Message != domain.PODReasonActionNotDelivery {
		t.Fatalf("err = %v", err)
	}
}

func TestGetPODRequirementStoredUnknownKeepsIdentity(t *testing.T) {
	subject := domain.PODRequirementSubject{ActionType: domain.ActionTypeDelivery, ShipmentID: uuid.New(), CargoID: uuid.New()}
	id := uuid.New()
	svc := NewPODRequirementService(fakePODStore{load: domain.PODRequirementLoad{Subject: subject, Decision: &domain.PODRequirementDecision{
		ID: id, DecisionVersion: 4, DecisionState: domain.PODDecisionStateUnknown, CreatedAt: time.Now().UTC(),
	}}})
	view, err := svc.GetPODRequirement(context.Background(), uuid.New(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if view.PodRequired != nil || view.DecisionID == nil || *view.DecisionID != id || view.DecisionVersion == nil || *view.DecisionVersion != 4 {
		t.Fatalf("view = %+v", view)
	}
}

func asApp(err error, target **apperrors.AppError) bool {
	app, ok := err.(*apperrors.AppError)
	if ok {
		*target = app
	}
	return ok
}
