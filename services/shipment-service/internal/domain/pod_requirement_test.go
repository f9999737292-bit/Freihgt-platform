package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSyntheticPODRequirementStaysUnknown(t *testing.T) {
	subject := testPODSubject()
	view := SyntheticPODRequirement(subject)
	if view.DecisionState != PODDecisionStateUnknown || view.PodRequired != nil || view.DecisionID != nil || view.DecisionVersion != nil {
		t.Fatalf("synthetic view = %+v", view)
	}
	if view.RequiredDocumentType != nil || view.AcceptancePolicyID != nil || view.RequirementDecidedAt != nil {
		t.Fatalf("synthetic view filled requirement facts %+v", view)
	}
	if view.ShipmentID != subject.ShipmentID || view.CargoID != subject.CargoID || view.ExecutionStopID != subject.ExecutionStopID {
		t.Fatal("synthetic view dropped canonical identity")
	}
}

func TestStoredUnknownDoesNotBecomeFalse(t *testing.T) {
	subject := testPODSubject()
	id := uuid.New()
	version := 2
	revision := uuid.New()
	view, err := ViewFromPODDecision(subject, PODRequirementDecision{
		ID: id, DecisionVersion: version, ExecutionRevisionID: &revision, DecisionState: PODDecisionStateUnknown,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.DecisionState != PODDecisionStateUnknown || view.PodRequired != nil || view.DecisionID == nil || *view.DecisionID != id || view.DecisionVersion == nil || *view.DecisionVersion != version {
		t.Fatalf("stored unknown = %+v", view)
	}
	if view.ExecutionRevisionID == nil || *view.ExecutionRevisionID != revision {
		t.Fatal("stored unknown dropped revision pin")
	}
}

func TestStoredKnownRequirementShapes(t *testing.T) {
	subject := testPODSubject()
	decided := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	policy := uuid.New()
	policyVersion := 3
	required, err := ViewFromPODDecision(subject, PODRequirementDecision{
		ID: uuid.New(), DecisionVersion: 1, DecisionState: PODDecisionStateKnown,
		PodRequired: boolPtr(true), RequiredDocumentType: strPtr(PODRequiredDocumentType),
		AcceptancePolicyID: &policy, AcceptancePolicyVersion: &policyVersion, RequirementDecidedAt: &decided,
	})
	if err != nil {
		t.Fatal(err)
	}
	if required.DecisionState != PODDecisionStateKnown || required.PodRequired == nil || !*required.PodRequired || required.RequiredDocumentType == nil || *required.RequiredDocumentType != PODRequiredDocumentType {
		t.Fatalf("known required = %+v", required)
	}
	if required.AcceptancePolicyID == nil || *required.AcceptancePolicyID != policy || required.AcceptancePolicyVersion == nil || *required.AcceptancePolicyVersion != policyVersion {
		t.Fatal("known required dropped acceptance pin")
	}
	notRequired, err := ViewFromPODDecision(subject, PODRequirementDecision{
		ID: uuid.New(), DecisionVersion: 1, DecisionState: PODDecisionStateKnown,
		PodRequired: boolPtr(false), RequirementDecidedAt: &decided,
	})
	if err != nil {
		t.Fatal(err)
	}
	if notRequired.PodRequired == nil || *notRequired.PodRequired || notRequired.RequiredDocumentType != nil || notRequired.AcceptancePolicyID != nil {
		t.Fatalf("known not required = %+v", notRequired)
	}
}

func TestKnownRequiredWithoutAcceptancePinIsInvalid(t *testing.T) {
	subject := testPODSubject()
	decided := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	_, err := ViewFromPODDecision(subject, PODRequirementDecision{
		ID: uuid.New(), DecisionVersion: 1, DecisionState: PODDecisionStateKnown,
		PodRequired: boolPtr(true), RequiredDocumentType: strPtr(PODRequiredDocumentType), RequirementDecidedAt: &decided,
	})
	if err == nil {
		t.Fatal("expected invalid known required")
	}
}

func testPODSubject() PODRequirementSubject {
	return PODRequirementSubject{
		OperatingTenantID: uuid.New(), ExecutionID: uuid.New(), ExecutionActionID: uuid.New(),
		ExecutionStopID: uuid.New(), ShipmentID: uuid.New(), CargoID: uuid.New(), ActionType: ActionTypeDelivery,
	}
}

func boolPtr(v bool) *bool { return &v }

func strPtr(v string) *string { return &v }
