package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

// PODRequirementReader is the internal read for one delivery action.
type PODRequirementReader interface {
	GetPODRequirement(ctx context.Context, operatingTenantID, executionID, actionID uuid.UUID) (domain.PODRequirementView, error)
}

// PODRequirementHandler serves the frozen internal POD requirement read.
type PODRequirementHandler struct {
	reader PODRequirementReader
}

func NewPODRequirementHandler(reader PODRequirementReader) *PODRequirementHandler {
	return &PODRequirementHandler{reader: reader}
}

type podRequirementResponse struct {
	OperatingTenantID       uuid.UUID  `json:"operating_tenant_id"`
	ExecutionID             uuid.UUID  `json:"execution_id"`
	ExecutionActionID       uuid.UUID  `json:"execution_action_id"`
	ExecutionStopID         uuid.UUID  `json:"execution_stop_id"`
	ExecutionRevisionID     *uuid.UUID `json:"execution_revision_id"`
	ShipmentID              uuid.UUID  `json:"shipment_id"`
	CargoID                 uuid.UUID  `json:"cargo_id"`
	DecisionID              *uuid.UUID `json:"pod_requirement_decision_id"`
	DecisionVersion         *int       `json:"decision_version"`
	DecisionState           string     `json:"decision_state"`
	PodRequired             *bool      `json:"pod_required"`
	RequiredDocumentType    *string    `json:"required_document_type"`
	AcceptancePolicyID      *uuid.UUID `json:"acceptance_policy_id"`
	AcceptancePolicyVersion *int       `json:"acceptance_policy_version"`
	RequirementDecidedAt    *time.Time `json:"requirement_decided_at"`
	BusinessEffectiveAt     *time.Time `json:"business_effective_at"`
	SourcePolicyID          *uuid.UUID `json:"source_policy_id"`
	SourcePolicyVersion     *int       `json:"source_policy_version"`
	SourceRuleID            *string    `json:"source_rule_id"`
	DecisionActorClass      *string    `json:"decision_actor_class"`
}

func (h *PODRequirementHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	executionID, err := domain.ParseUUID(chi.URLParam(r, "executionId"), "execution_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	actionID, err := domain.ParseUUID(chi.URLParam(r, "actionId"), "action_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	view, err := h.reader.GetPODRequirement(r.Context(), tenantID, executionID, actionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, podRequirementResponse{
		OperatingTenantID:       view.OperatingTenantID,
		ExecutionID:             view.ExecutionID,
		ExecutionActionID:       view.ExecutionActionID,
		ExecutionStopID:         view.ExecutionStopID,
		ExecutionRevisionID:     view.ExecutionRevisionID,
		ShipmentID:              view.ShipmentID,
		CargoID:                 view.CargoID,
		DecisionID:              view.DecisionID,
		DecisionVersion:         view.DecisionVersion,
		DecisionState:           view.DecisionState,
		PodRequired:             view.PodRequired,
		RequiredDocumentType:    view.RequiredDocumentType,
		AcceptancePolicyID:      view.AcceptancePolicyID,
		AcceptancePolicyVersion: view.AcceptancePolicyVersion,
		RequirementDecidedAt:    view.RequirementDecidedAt,
		BusinessEffectiveAt:     view.BusinessEffectiveAt,
		SourcePolicyID:          view.SourcePolicyID,
		SourcePolicyVersion:     view.SourcePolicyVersion,
		SourceRuleID:            view.SourceRuleID,
		DecisionActorClass:      view.DecisionActorClass,
	})
}
