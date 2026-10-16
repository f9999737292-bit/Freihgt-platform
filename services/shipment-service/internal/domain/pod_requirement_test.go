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
	policy := uuid.New()
	policyVersion := 3
	source := uuid.New()
	sourceVersion := 1
	required, err := ViewFromPODDecision(subject, knownDecision(true, &policy, &policyVersion, &source, &sourceVersion))
	if err != nil {
		t.Fatal(err)
	}
	if required.DecisionState != PODDecisionStateKnown || required.PodRequired == nil || !*required.PodRequired || required.RequiredDocumentType == nil || *required.RequiredDocumentType != PODRequiredDocumentType {
		t.Fatalf("known required = %+v", required)
	}
	if required.AcceptancePolicyID == nil || *required.AcceptancePolicyID != policy || required.SourcePolicyID == nil || *required.SourcePolicyID != source {
		t.Fatal("known required dropped pins")
	}
	notRequired, err := ViewFromPODDecision(subject, knownDecision(false, nil, nil, &source, &sourceVersion))
	if err != nil {
		t.Fatal(err)
	}
	if notRequired.PodRequired == nil || *notRequired.PodRequired || notRequired.RequiredDocumentType != nil || notRequired.AcceptancePolicyID != nil {
		t.Fatalf("known not required = %+v", notRequired)
	}
}

func TestKnownRequiredWithoutAcceptancePinIsInvalid(t *testing.T) {
	subject := testPODSubject()
	source := uuid.New()
	sourceVersion := 1
	decision := knownDecision(true, nil, nil, &source, &sourceVersion)
	if _, err := ViewFromPODDecision(subject, decision); err == nil {
		t.Fatal("expected invalid known required")
	}
}

func TestKnownWithoutProvenanceIsUnavailable(t *testing.T) {
	subject := testPODSubject()
	source := uuid.New()
	sourceVersion := 1
	policy := uuid.New()
	policyVersion := 1
	base := knownDecision(true, &policy, &policyVersion, &source, &sourceVersion)
	missingPolicy := base
	missingPolicy.SourcePolicyID = nil
	missingPolicy.SourcePolicyVersion = nil
	missingRevision := base
	missingRevision.ExecutionRevisionID = nil
	missingActor := base
	missingActor.DecisionActorClass = nil
	for _, decision := range []PODRequirementDecision{missingPolicy, missingRevision, missingActor} {
		if _, err := ViewFromPODDecision(subject, decision); err == nil {
			t.Fatalf("accepted malformed known decision %+v", decision)
		}
	}
}

func knownDecision(required bool, acceptance *uuid.UUID, acceptanceVersion *int, source *uuid.UUID, sourceVersion *int) PODRequirementDecision {
	decided := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	revision := uuid.New()
	actor := PODDecisionActorSystem
	decision := PODRequirementDecision{
		ID: uuid.New(), DecisionVersion: 1, DecisionState: PODDecisionStateKnown,
		ExecutionRevisionID: &revision, PodRequired: boolPtr(required), RequirementDecidedAt: &decided,
		SourcePolicyID: source, SourcePolicyVersion: sourceVersion, DecisionActorClass: &actor,
	}
	if required {
		docType := PODRequiredDocumentType
		decision.RequiredDocumentType = &docType
		decision.AcceptancePolicyID = acceptance
		decision.AcceptancePolicyVersion = acceptanceVersion
	}
	return decision
}

func testPODSubject() PODRequirementSubject {
	return PODRequirementSubject{
		OperatingTenantID: uuid.New(), ExecutionID: uuid.New(), ExecutionActionID: uuid.New(),
		ExecutionStopID: uuid.New(), ShipmentID: uuid.New(), CargoID: uuid.New(), ActionType: ActionTypeDelivery,
	}
}

func boolPtr(v bool) *bool { return &v }

func strPtr(v string) *string { return &v }
