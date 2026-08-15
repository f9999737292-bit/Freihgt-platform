package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain/tender"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

// CarrierPerformanceProvider supplies carrier KPI inputs for scoring.
// Future: Control Tower aggregated performance via service boundary.
type CarrierPerformanceProvider interface {
	CarrierKPI(ctx context.Context, tenantID, carrierCompanyID uuid.UUID) (kpiScore float64, reliabilityScore float64, err error)
}

type StaticCarrierPerformanceProvider struct {
	ByCarrier map[string]struct {
		KPI         float64
		Reliability float64
	}
}

func (p StaticCarrierPerformanceProvider) CarrierKPI(_ context.Context, _ uuid.UUID, carrierCompanyID uuid.UUID) (float64, float64, error) {
	if p.ByCarrier == nil {
		return 0, 0, nil
	}
	if v, ok := p.ByCarrier[carrierCompanyID.String()]; ok {
		return v.KPI, v.Reliability, nil
	}
	return 0, 0, nil
}

type RunEvaluationInput struct {
	TenantID                  uuid.UUID
	RfxEventID                uuid.UUID
	ScoringTemplateVersionID  uuid.UUID
	QualificationRules        tender.QualificationRules
	RequiredVolume            float64
}

type RunEvaluationResult struct {
	EvaluationID       uuid.UUID
	Qualification      []tender.QualificationResult
	Scores             []tender.CarrierScoreResult
	ScoringSnapshot    tender.ScoringTemplateSnapshot
}

type CreateAllocationScenarioInput struct {
	TenantID     uuid.UUID
	EvaluationID uuid.UUID
	Name         string
	Config       tender.AllocationConfig
	QuotaTargets []tender.QuotaTarget
	QuotaPolicy  tender.QuotaBalancePolicy
	ActualShares map[string]float64
}

type TenderEvaluationStore interface {
	GetScoringTemplateVersion(ctx context.Context, id, tenantID uuid.UUID) (*tender.ScoringTemplateSnapshot, error)
	CreateScoringTemplate(ctx context.Context, tenantID uuid.UUID, code, name string, factors []tender.ScoringFactorWeight, createdBy *uuid.UUID) (templateID, versionID uuid.UUID, err error)
	CreateEvaluation(ctx context.Context, in RunEvaluationInput, snapshot tender.ScoringTemplateSnapshot, qual []tender.QualificationResult, scores []tender.CarrierScoreResult) (uuid.UUID, error)
	ListEvaluationCandidates(ctx context.Context, rfxEventID, tenantID uuid.UUID) ([]tender.BidCandidate, error)
	LoadEvaluationForAllocation(ctx context.Context, evaluationID, tenantID uuid.UUID) (tender.AllocationOutcome, []tender.CarrierScoreResult, []tender.BidCandidate, error)
	SaveAllocationScenario(ctx context.Context, in CreateAllocationScenarioInput, outcome tender.AllocationOutcome, positions []tender.QuotaPosition) (uuid.UUID, error)
	LoadScenarioLines(ctx context.Context, scenarioID, tenantID uuid.UUID) ([]tender.AllocationLine, tender.ScoringTemplateSnapshot, error)
	CreateAwardProposal(ctx context.Context, tenantID, rfxEventID, evaluationID, scenarioID uuid.UUID, lines []tender.AllocationLine, snapshot tender.ScoringTemplateSnapshot, createdBy *uuid.UUID, idempotencyKey *string) (uuid.UUID, error)
	SubmitAwardProposal(ctx context.Context, proposalID, tenantID uuid.UUID) error
	ApproveAwardProposal(ctx context.Context, proposalID, tenantID uuid.UUID, approvedBy uuid.UUID) error
	RejectAwardProposal(ctx context.Context, proposalID, tenantID uuid.UUID, rejectedBy uuid.UUID) error
	FinalizeAward(ctx context.Context, proposalID, tenantID uuid.UUID, finalizedBy uuid.UUID, idempotencyKey *string) (uuid.UUID, error)
	GetAwardProposal(ctx context.Context, proposalID, tenantID uuid.UUID) (status string, rfxEventID uuid.UUID, err error)
}

type EvaluationService struct {
	store       TenderEvaluationStore
	performance CarrierPerformanceProvider
}

func NewEvaluationService(store TenderEvaluationStore, performance CarrierPerformanceProvider) *EvaluationService {
	if performance == nil {
		performance = StaticCarrierPerformanceProvider{}
	}
	return &EvaluationService{store: store, performance: performance}
}

func (s *EvaluationService) CreateScoringTemplate(ctx context.Context, tenantID uuid.UUID, code, name string, factors []tender.ScoringFactorWeight, createdBy *uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	return s.store.CreateScoringTemplate(ctx, tenantID, code, name, factors, createdBy)
}

func (s *EvaluationService) RunEvaluation(ctx context.Context, in RunEvaluationInput) (*RunEvaluationResult, error) {
	if in.TenantID == uuid.Nil || in.RfxEventID == uuid.Nil || in.ScoringTemplateVersionID == uuid.Nil {
		return nil, apperrors.Validation("tenant_id, rfx_event_id and scoring_template_version_id are required", map[string]any{})
	}
	snapshot, err := s.store.GetScoringTemplateVersion(ctx, in.ScoringTemplateVersionID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if err := tender.ValidateScoringTemplate(snapshot.Factors); err != nil {
		return nil, apperrors.Validation(err.Error(), map[string]any{"field": "scoring_template"})
	}

	candidates, err := s.store.ListEvaluationCandidates(ctx, in.RfxEventID, in.TenantID)
	if err != nil {
		return nil, err
	}
	for i := range candidates {
		carrierID, err := uuid.Parse(candidates[i].CarrierCompanyID)
		if err != nil {
			continue
		}
		kpi, rel, err := s.performance.CarrierKPI(ctx, in.TenantID, carrierID)
		if err != nil {
			return nil, err
		}
		if candidates[i].CarrierKPIInput == 0 && kpi > 0 {
			candidates[i].CarrierKPIInput = kpi
		}
		if candidates[i].ReliabilityInput == 0 && rel > 0 {
			candidates[i].ReliabilityInput = rel
		}
	}

	qual := tender.EvaluateQualification(in.QualificationRules, candidates)
	qualified := tender.QualifiedCarrierSet(qual)
	filtered := tender.FilterQualifiedCandidates(qualified, candidates)
	scores, err := tender.ScoreCandidates(*snapshot, filtered, in.RequiredVolume)
	if err != nil {
		return nil, apperrors.Validation(err.Error(), map[string]any{"field": "scoring"})
	}

	evalID, err := s.store.CreateEvaluation(ctx, in, *snapshot, qual, scores)
	if err != nil {
		return nil, err
	}
	return &RunEvaluationResult{
		EvaluationID:    evalID,
		Qualification:   qual,
		Scores:          scores,
		ScoringSnapshot: *snapshot,
	}, nil
}

func (s *EvaluationService) RunAllocationScenario(ctx context.Context, in CreateAllocationScenarioInput) (uuid.UUID, tender.AllocationOutcome, []tender.QuotaPosition, error) {
	if in.TenantID == uuid.Nil || in.EvaluationID == uuid.Nil {
		return uuid.Nil, tender.AllocationOutcome{}, nil, apperrors.Validation("tenant_id and evaluation_id are required", map[string]any{})
	}
	// Repository reloads scores/candidates for evaluation — service layer applies quota + allocation engine.
	outcome, scores, candidates, err := s.store.LoadEvaluationForAllocation(ctx, in.EvaluationID, in.TenantID)
	if err != nil {
		return uuid.Nil, tender.AllocationOutcome{}, nil, err
	}
	_ = outcome
	positions, err := tender.ComputeQuotaPositions(in.QuotaPolicy, in.QuotaTargets, in.ActualShares)
	if err != nil {
		return uuid.Nil, tender.AllocationOutcome{}, nil, apperrors.Validation(err.Error(), map[string]any{"field": "quota_targets"})
	}
	qualified := make(map[string]struct{}, len(scores))
	for _, sc := range scores {
		qualified[sc.CarrierCompanyID] = struct{}{}
	}
	adj := tender.BalanceAdjustmentsForAllocation(positions, qualified)
	computed := tender.ComputeAllocation(in.Config, scores, candidates, adj)
	if computed.Status == tender.AllocationStatusComputed {
		computed.Lines = tender.ApplyQualificationOverrideBalance(computed.Lines, qualified)
	}
	scenarioID, err := s.store.SaveAllocationScenario(ctx, in, computed, positions)
	if err != nil {
		return uuid.Nil, tender.AllocationOutcome{}, nil, err
	}
	return scenarioID, computed, positions, nil
}

func (s *EvaluationService) CreateAwardProposal(ctx context.Context, tenantID, rfxEventID, evaluationID, scenarioID uuid.UUID, createdBy *uuid.UUID, idempotencyKey *string) (uuid.UUID, error) {
	lines, snapshot, err := s.store.LoadScenarioLines(ctx, scenarioID, tenantID)
	if err != nil {
		return uuid.Nil, err
	}
	return s.store.CreateAwardProposal(ctx, tenantID, rfxEventID, evaluationID, scenarioID, lines, snapshot, createdBy, idempotencyKey)
}

func (s *EvaluationService) SubmitAwardProposal(ctx context.Context, proposalID, tenantID uuid.UUID) error {
	return s.store.SubmitAwardProposal(ctx, proposalID, tenantID)
}

func (s *EvaluationService) ApproveAwardProposal(ctx context.Context, proposalID, tenantID, approvedBy uuid.UUID) error {
	return s.store.ApproveAwardProposal(ctx, proposalID, tenantID, approvedBy)
}

func (s *EvaluationService) RejectAwardProposal(ctx context.Context, proposalID, tenantID, rejectedBy uuid.UUID) error {
	return s.store.RejectAwardProposal(ctx, proposalID, tenantID, rejectedBy)
}

func (s *EvaluationService) FinalizeAward(ctx context.Context, proposalID, tenantID, finalizedBy uuid.UUID, idempotencyKey *string) (uuid.UUID, error) {
	status, _, err := s.store.GetAwardProposal(ctx, proposalID, tenantID)
	if err != nil {
		return uuid.Nil, err
	}
	if err := tender.ValidateFinalizeAward(status); err != nil {
		return uuid.Nil, apperrors.Validation(err.Error(), map[string]any{"field": "status"})
	}
	return s.store.FinalizeAward(ctx, proposalID, tenantID, finalizedBy, idempotencyKey)
}
