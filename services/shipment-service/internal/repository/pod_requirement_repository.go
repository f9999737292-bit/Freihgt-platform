package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

// PODRequirementRepository reads one delivery action and its latest decision version.
// It does not insert a decision when the row is missing.
type PODRequirementRepository struct {
	db QueryRower
}

func NewPODRequirementRepository(db QueryRower) *PODRequirementRepository {
	return &PODRequirementRepository{db: db}
}

func (r *PODRequirementRepository) LoadPODRequirement(ctx context.Context, operatingTenantID, executionID, actionID uuid.UUID) (domain.PODRequirementLoad, error) {
	subject, err := r.loadSubject(ctx, operatingTenantID, executionID, actionID)
	if err != nil {
		return domain.PODRequirementLoad{}, err
	}
	decision, err := r.loadLatestDecision(ctx, operatingTenantID, actionID)
	if err != nil {
		return domain.PODRequirementLoad{}, err
	}
	return domain.PODRequirementLoad{Subject: subject, Decision: decision}, nil
}

func (r *PODRequirementRepository) loadSubject(ctx context.Context, operatingTenantID, executionID, actionID uuid.UUID) (domain.PODRequirementSubject, error) {
	const query = `
		SELECT e.operating_tenant_id, e.id, a.id, s.id, a.shipment_id, a.cargo_id, a.action_type
		FROM transport.transport_execution_actions AS a
		JOIN transport.transport_execution_stops AS s ON s.id = a.execution_stop_id
		JOIN transport.transport_executions AS e ON e.id = s.execution_id
		WHERE e.id = $1
		  AND e.operating_tenant_id = $2
		  AND a.id = $3
	`
	var subject domain.PODRequirementSubject
	err := r.db.QueryRow(ctx, query, executionID, operatingTenantID, actionID).Scan(
		&subject.OperatingTenantID,
		&subject.ExecutionID,
		&subject.ExecutionActionID,
		&subject.ExecutionStopID,
		&subject.ShipmentID,
		&subject.CargoID,
		&subject.ActionType,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PODRequirementSubject{}, apperrors.NotFound("pod requirement subject was not found")
	}
	if err != nil {
		return domain.PODRequirementSubject{}, apperrors.Unavailable("pod requirement store is unavailable", err)
	}
	return subject, nil
}

func (r *PODRequirementRepository) loadLatestDecision(ctx context.Context, operatingTenantID, actionID uuid.UUID) (*domain.PODRequirementDecision, error) {
	const query = `
		SELECT id, decision_version, execution_revision_id, decision_state, pod_required,
			required_document_type, acceptance_policy_id, acceptance_policy_version,
			requirement_decided_at, business_effective_at, source_policy_id, source_policy_version,
			source_rule_id, decision_actor_class, created_at, supersedes_decision_id
		FROM transport.transport_execution_pod_requirement_decisions
		WHERE operating_tenant_id = $1
		  AND execution_action_id = $2
		ORDER BY decision_version DESC
		LIMIT 1
	`
	var decision domain.PODRequirementDecision
	err := r.db.QueryRow(ctx, query, operatingTenantID, actionID).Scan(
		&decision.ID,
		&decision.DecisionVersion,
		&decision.ExecutionRevisionID,
		&decision.DecisionState,
		&decision.PodRequired,
		&decision.RequiredDocumentType,
		&decision.AcceptancePolicyID,
		&decision.AcceptancePolicyVersion,
		&decision.RequirementDecidedAt,
		&decision.BusinessEffectiveAt,
		&decision.SourcePolicyID,
		&decision.SourcePolicyVersion,
		&decision.SourceRuleID,
		&decision.DecisionActorClass,
		&decision.CreatedAt,
		&decision.SupersedesDecisionID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, apperrors.Unavailable("pod requirement store is unavailable", err)
	}
	return &decision, nil
}
