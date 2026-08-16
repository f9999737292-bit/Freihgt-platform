package service

import (
	"sort"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

type RuleEvaluator struct{}

func NewRuleEvaluator() *RuleEvaluator {
	return &RuleEvaluator{}
}

func (e *RuleEvaluator) EvaluateRules(trigger domain.AutomationTrigger, ctx domain.AutomationContext, rules []domain.AutomationRule, playbookVersions map[uuid.UUID]domain.PlaybookVersion) []domain.RuleMatch {
	matches := make([]domain.RuleMatch, 0)
	for _, rule := range rules {
		if rule.Status != domain.RuleStatusActive {
			continue
		}
		if rule.TriggerType != trigger.TriggerType {
			continue
		}
		ok, condResults := domain.EvaluateConditionGroup(rule.Conditions, ctx)
		if !ok {
			continue
		}
		match := domain.RuleMatch{
			Rule:              rule,
			Matched:           true,
			MatchedConditions: filterMatched(condResults),
		}
		if rule.PlaybookID != nil {
			match.SelectedPlaybookID = rule.PlaybookID
			if pv, ok := playbookVersions[*rule.PlaybookID]; ok {
				match.PlaybookVersion = pv.Version
				match.PlaybookVersionID = pv.ID
			}
		}
		matches = append(matches, match)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Rule.Priority != matches[j].Rule.Priority {
			return matches[i].Rule.Priority > matches[j].Rule.Priority
		}
		return matches[i].Rule.ID.String() < matches[j].Rule.ID.String()
	})
	return dedupePlaybookMatches(matches)
}

func filterMatched(all []domain.MatchedCondition) []domain.MatchedCondition {
	out := make([]domain.MatchedCondition, 0, len(all))
	for _, c := range all {
		if c.Matched {
			out = append(out, c)
		}
	}
	return out
}

// dedupePlaybookMatches keeps highest-priority rule per playbook+operational object.
func dedupePlaybookMatches(matches []domain.RuleMatch) []domain.RuleMatch {
	if len(matches) <= 1 {
		return matches
	}
	seen := map[string]struct{}{}
	out := make([]domain.RuleMatch, 0, len(matches))
	for _, m := range matches {
		if m.SelectedPlaybookID == nil {
			out = append(out, m)
			continue
		}
		key := m.SelectedPlaybookID.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, m)
	}
	return out
}
