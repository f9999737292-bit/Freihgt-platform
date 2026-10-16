package domain

import (
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const (
	PODDecisionStateKnown   = "KNOWN"
	PODDecisionStateUnknown = "UNKNOWN"

	PODRequiredDocumentType = "POD"

	PODReasonActionNotDelivery = "ACTION_NOT_DELIVERY"
)

// PODRequirementSubject is the canonical delivery action addressed by one read.
type PODRequirementSubject struct {
	OperatingTenantID uuid.UUID
	ExecutionID       uuid.UUID
	ExecutionActionID uuid.UUID
	ExecutionStopID   uuid.UUID
	ShipmentID        uuid.UUID
	CargoID           uuid.UUID
	ActionType        string
}

// PODRequirementDecision is one immutable stored version. A nil decision means no row.
type PODRequirementDecision struct {
	ID                      uuid.UUID
	DecisionVersion         int
	ExecutionRevisionID     *uuid.UUID
	DecisionState           string
	PodRequired             *bool
	RequiredDocumentType    *string
	AcceptancePolicyID      *uuid.UUID
	AcceptancePolicyVersion *int
	RequirementDecidedAt    *time.Time
	BusinessEffectiveAt     *time.Time
	SourcePolicyID          *uuid.UUID
	SourcePolicyVersion     *int
	SourceRuleID            *string
	DecisionActorClass      *string
	CreatedAt               time.Time
	SupersedesDecisionID    *uuid.UUID
}

// PODRequirementLoad is the repository result for one tenant-scoped action.
type PODRequirementLoad struct {
	Subject  PODRequirementSubject
	Decision *PODRequirementDecision
}

// PODRequirementView is the internal read body. Pointers stay nil for unknown facts.
type PODRequirementView struct {
	OperatingTenantID       uuid.UUID
	ExecutionID             uuid.UUID
	ExecutionActionID       uuid.UUID
	ExecutionStopID         uuid.UUID
	ExecutionRevisionID     *uuid.UUID
	ShipmentID              uuid.UUID
	CargoID                 uuid.UUID
	DecisionID              *uuid.UUID
	DecisionVersion         *int
	DecisionState           string
	PodRequired             *bool
	RequiredDocumentType    *string
	AcceptancePolicyID      *uuid.UUID
	AcceptancePolicyVersion *int
	RequirementDecidedAt    *time.Time
	BusinessEffectiveAt     *time.Time
	SourcePolicyID          *uuid.UUID
	SourcePolicyVersion     *int
	SourceRuleID            *string
	DecisionActorClass      *string
}

// SyntheticPODRequirement is the no-row result. It does not invent a stored decision.
func SyntheticPODRequirement(subject PODRequirementSubject) PODRequirementView {
	return PODRequirementView{
		OperatingTenantID: subject.OperatingTenantID,
		ExecutionID:       subject.ExecutionID,
		ExecutionActionID: subject.ExecutionActionID,
		ExecutionStopID:   subject.ExecutionStopID,
		ShipmentID:        subject.ShipmentID,
		CargoID:           subject.CargoID,
		DecisionState:     PODDecisionStateUnknown,
	}
}

// ViewFromPODDecision copies a stored version without turning UNKNOWN into false.
func ViewFromPODDecision(subject PODRequirementSubject, decision PODRequirementDecision) (PODRequirementView, error) {
	view := SyntheticPODRequirement(subject)
	id := decision.ID
	version := decision.DecisionVersion
	view.DecisionID = &id
	view.DecisionVersion = &version
	view.ExecutionRevisionID = decision.ExecutionRevisionID
	view.BusinessEffectiveAt = decision.BusinessEffectiveAt
	view.SourcePolicyID = decision.SourcePolicyID
	view.SourcePolicyVersion = decision.SourcePolicyVersion
	view.SourceRuleID = decision.SourceRuleID
	view.DecisionActorClass = decision.DecisionActorClass
	switch decision.DecisionState {
	case PODDecisionStateUnknown:
		if decision.PodRequired != nil || decision.RequiredDocumentType != nil || decision.AcceptancePolicyID != nil || decision.AcceptancePolicyVersion != nil || decision.RequirementDecidedAt != nil {
			return PODRequirementView{}, apperrors.Unavailable("stored pod requirement decision is invalid", nil)
		}
		view.DecisionState = PODDecisionStateUnknown
		return view, nil
	case PODDecisionStateKnown:
		if decision.PodRequired == nil || decision.RequirementDecidedAt == nil {
			return PODRequirementView{}, apperrors.Unavailable("stored pod requirement decision is invalid", nil)
		}
		view.DecisionState = PODDecisionStateKnown
		view.RequirementDecidedAt = decision.RequirementDecidedAt
		if *decision.PodRequired {
			if decision.RequiredDocumentType == nil || *decision.RequiredDocumentType != PODRequiredDocumentType || decision.AcceptancePolicyID == nil || decision.AcceptancePolicyVersion == nil {
				return PODRequirementView{}, apperrors.Unavailable("stored pod requirement decision is invalid", nil)
			}
			required := true
			docType := PODRequiredDocumentType
			view.PodRequired = &required
			view.RequiredDocumentType = &docType
			view.AcceptancePolicyID = decision.AcceptancePolicyID
			view.AcceptancePolicyVersion = decision.AcceptancePolicyVersion
			return view, nil
		}
		if decision.RequiredDocumentType != nil || decision.AcceptancePolicyID != nil || decision.AcceptancePolicyVersion != nil {
			return PODRequirementView{}, apperrors.Unavailable("stored pod requirement decision is invalid", nil)
		}
		required := false
		view.PodRequired = &required
		return view, nil
	default:
		return PODRequirementView{}, apperrors.Unavailable("stored pod requirement decision is invalid", nil)
	}
}
