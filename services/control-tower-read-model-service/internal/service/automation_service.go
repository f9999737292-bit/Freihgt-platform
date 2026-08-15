package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type AutomationService struct {
	repo      *repository.AutomationRepository
	evaluator *RuleEvaluator
}

func NewAutomationService(repo *repository.AutomationRepository) *AutomationService {
	return &AutomationService{repo: repo, evaluator: NewRuleEvaluator()}
}

type DryRunResult struct {
	Matched           bool                       `json:"matched"`
	MatchedConditions []domain.MatchedCondition  `json:"matchedConditions"`
	SelectedPlaybook  *uuid.UUID                 `json:"selectedPlaybookId,omitempty"`
	PlaybookVersion   int                        `json:"playbookVersion,omitempty"`
}

type EvaluateOutcome struct {
	Matches         []domain.RuleMatch                  `json:"matches"`
	Recommendations []domain.AutomationRecommendation   `json:"recommendations,omitempty"`
	Deduplicated    int                                 `json:"deduplicated"`
}

func (s *AutomationService) DryRunRule(ctx context.Context, tenantID, ruleID uuid.UUID, trigger domain.AutomationTrigger) (DryRunResult, error) {
	rule, err := s.repo.GetRule(ctx, tenantID, ruleID)
	if err != nil {
		return DryRunResult{}, err
	}
	ctxData := domain.BuildContextFromTrigger(trigger)
	ok, results := domain.EvaluateConditionGroup(rule.Conditions, ctxData)
	out := DryRunResult{Matched: ok && rule.TriggerType == trigger.TriggerType, MatchedConditions: results}
	if out.Matched && rule.PlaybookID != nil {
		out.SelectedPlaybook = rule.PlaybookID
		versions, _ := s.repo.GetActivePlaybookVersions(ctx, tenantID, []uuid.UUID{*rule.PlaybookID})
		if pv, ok := versions[*rule.PlaybookID]; ok {
			out.PlaybookVersion = pv.Version
		}
	}
	return out, nil
}

func (s *AutomationService) EvaluateTrigger(ctx context.Context, tenantID uuid.UUID, trigger domain.AutomationTrigger, persist bool) (EvaluateOutcome, error) {
	if err := domain.ValidateTriggerType(trigger.TriggerType); err != nil {
		return EvaluateOutcome{}, err
	}
	if trigger.TriggerID == "" {
		trigger.TriggerID = fmt.Sprintf("%s:%s", trigger.TriggerType, uuid.NewString())
	}
	rules, err := s.repo.ListActiveRulesByTrigger(ctx, tenantID, trigger.TriggerType)
	if err != nil {
		return EvaluateOutcome{}, err
	}
	playbookIDs := make([]uuid.UUID, 0)
	for _, rule := range rules {
		if rule.PlaybookID != nil {
			playbookIDs = append(playbookIDs, *rule.PlaybookID)
		}
	}
	versions, err := s.repo.GetActivePlaybookVersions(ctx, tenantID, playbookIDs)
	if err != nil {
		return EvaluateOutcome{}, err
	}
	ctxData := domain.BuildContextFromTrigger(trigger)
	matches := s.evaluator.EvaluateRules(trigger, ctxData, rules, versions)
	out := EvaluateOutcome{Matches: matches}
	if !persist {
		return out, nil
	}
	for _, match := range matches {
		rule := match.Rule
		if rule.ExecutionMode == domain.ExecutionModeObserve {
			continue
		}
		if match.SelectedPlaybookID == nil || match.PlaybookVersionID == uuid.Nil {
			continue
		}
		key := buildIdempotencyKey(tenantID, rule, trigger)
		rec := domain.AutomationRecommendation{
			TenantID:          tenantID,
			RuleID:            rule.ID,
			RuleVersion:       rule.Version,
			PlaybookID:        *match.SelectedPlaybookID,
			PlaybookVersion:   match.PlaybookVersion,
			PlaybookVersionID: match.PlaybookVersionID,
			TriggerID:         trigger.TriggerID,
			TriggerType:       trigger.TriggerType,
			CorrelationID:     trigger.CorrelationID,
			CausationID:       trigger.CausationID,
			ShipmentID:        trigger.ShipmentID,
			WorkItemType:      trigger.WorkItemType,
			WorkItemID:        trigger.WorkItemID,
			CaseID:            trigger.CaseID,
			RiskID:            trigger.RiskID,
			ExceptionID:       trigger.ExceptionID,
			MatchExplanation:  match.MatchedConditions,
			IdempotencyKey:    key,
		}
		createdRec, created, err := s.repo.CreateRecommendation(ctx, rec)
		if err != nil {
			continue
		}
		if created {
			out.Recommendations = append(out.Recommendations, createdRec)
		} else {
			out.Deduplicated++
		}
	}
	return out, nil
}

func buildIdempotencyKey(tenantID uuid.UUID, rule domain.AutomationRule, trigger domain.AutomationTrigger) string {
	stateVersion := strings.TrimSpace(trigger.Attributes.StateVersion)
	parts := []string{
		tenantID.String(),
		rule.ID.String(),
		fmt.Sprintf("v%d", rule.Version),
		trigger.TriggerType,
		trigger.TriggerID,
	}
	if trigger.ShipmentID != nil {
		parts = append(parts, trigger.ShipmentID.String())
	}
	if trigger.RiskID != "" {
		parts = append(parts, trigger.RiskID)
	}
	if trigger.ExceptionID != "" {
		parts = append(parts, trigger.ExceptionID)
	}
	if trigger.WorkItemID != "" {
		parts = append(parts, trigger.WorkItemType, trigger.WorkItemID)
	}
	if trigger.CaseID != nil {
		parts = append(parts, trigger.CaseID.String())
	}
	if stateVersion != "" {
		parts = append(parts, stateVersion)
	}
	key := strings.Join(parts, "|")
	if len(key) > 256 {
		sum := sha256.Sum256([]byte(key))
		key = "sha256:" + hex.EncodeToString(sum[:])
	}
	return key
}
