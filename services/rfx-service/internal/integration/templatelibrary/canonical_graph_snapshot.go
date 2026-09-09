//go:build integration

package templatelibrary

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

type canonicalSection struct {
	SectionCode string
	Title       string
	Description *string
	SortOrder   int
}

type canonicalQuestion struct {
	SectionCode        string
	QuestionCode       string
	QuestionType       string
	Label              string
	HelpText           *string
	Required           bool
	ValidationRuleJSON string
	SortOrder          int
}

type canonicalOption struct {
	SectionCode  string
	QuestionCode string
	OptionCode   string
	Label        string
	SortOrder    int
}

type canonicalRule struct {
	RuleCode           string
	Action             string
	ConditionJSON      string
	TargetQuestionCode string
	SortOrder          int
}

type canonicalGraphSnapshot struct {
	Sections  []canonicalSection
	Questions []canonicalQuestion
	Options   []canonicalOption
	Rules     []canonicalRule
}

type graphUUIDSets struct {
	SectionIDs  []uuid.UUID
	QuestionIDs []uuid.UUID
	OptionIDs   []uuid.UUID
	RuleIDs     []uuid.UUID
}

func normalizeJSONField(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return string(raw)
	}
	return string(normalized)
}

func canonicalFromEventGraph(def *domain.QuestionnaireDefinition) canonicalGraphSnapshot {
	questionCodeByID := make(map[uuid.UUID]string)
	for _, section := range def.Sections {
		for _, question := range section.Questions {
			questionCodeByID[question.ID] = question.QuestionCode
		}
	}
	out := canonicalGraphSnapshot{}
	for _, item := range def.Sections {
		out.Sections = append(out.Sections, canonicalSection{
			SectionCode: item.Section.SectionCode,
			Title:       item.Section.Title,
			Description: item.Section.Description,
			SortOrder:   item.Section.SortOrder,
		})
		for _, question := range item.Questions {
			out.Questions = append(out.Questions, canonicalQuestion{
				SectionCode:        item.Section.SectionCode,
				QuestionCode:       question.QuestionCode,
				QuestionType:       question.QuestionType,
				Label:              question.Label,
				HelpText:           question.HelpText,
				Required:           question.Required,
				ValidationRuleJSON: normalizeJSONField(question.ValidationRuleJSON),
				SortOrder:          question.SortOrder,
			})
			for _, option := range question.Options {
				out.Options = append(out.Options, canonicalOption{
					SectionCode:  item.Section.SectionCode,
					QuestionCode: question.QuestionCode,
					OptionCode:   option.OptionCode,
					Label:        option.Label,
					SortOrder:    option.SortOrder,
				})
			}
		}
	}
	for _, rule := range def.Rules {
		targetCode := ""
		if rule.TargetQuestionID != nil {
			targetCode = questionCodeByID[*rule.TargetQuestionID]
		}
		out.Rules = append(out.Rules, canonicalRule{
			RuleCode:           rule.RuleCode,
			Action:             rule.Action,
			ConditionJSON:      normalizeJSONField(rule.ConditionJSON),
			TargetQuestionCode: targetCode,
			SortOrder:          rule.SortOrder,
		})
	}
	sortCanonicalSnapshot(&out)
	return out
}

func canonicalFromTemplateGraph(def *domain.TemplateQuestionnaireDefinition) canonicalGraphSnapshot {
	questionCodeByID := make(map[uuid.UUID]string)
	for _, section := range def.Sections {
		for _, question := range section.Questions {
			questionCodeByID[question.ID] = question.QuestionCode
		}
	}
	out := canonicalGraphSnapshot{}
	for _, item := range def.Sections {
		out.Sections = append(out.Sections, canonicalSection{
			SectionCode: item.Section.SectionCode,
			Title:       item.Section.Title,
			Description: item.Section.Description,
			SortOrder:   item.Section.SortOrder,
		})
		for _, question := range item.Questions {
			out.Questions = append(out.Questions, canonicalQuestion{
				SectionCode:        item.Section.SectionCode,
				QuestionCode:       question.QuestionCode,
				QuestionType:       question.QuestionType,
				Label:              question.Label,
				HelpText:           question.HelpText,
				Required:           question.Required,
				ValidationRuleJSON: normalizeJSONField(question.ValidationRuleJSON),
				SortOrder:          question.SortOrder,
			})
			for _, option := range question.Options {
				out.Options = append(out.Options, canonicalOption{
					SectionCode:  item.Section.SectionCode,
					QuestionCode: question.QuestionCode,
					OptionCode:   option.OptionCode,
					Label:        option.Label,
					SortOrder:    option.SortOrder,
				})
			}
		}
	}
	for _, rule := range def.Rules {
		targetCode := ""
		if rule.TargetQuestionID != nil {
			targetCode = questionCodeByID[*rule.TargetQuestionID]
		}
		out.Rules = append(out.Rules, canonicalRule{
			RuleCode:           rule.RuleCode,
			Action:             rule.Action,
			ConditionJSON:      normalizeJSONField(rule.ConditionJSON),
			TargetQuestionCode: targetCode,
			SortOrder:          rule.SortOrder,
		})
	}
	sortCanonicalSnapshot(&out)
	return out
}

func sortCanonicalSnapshot(s *canonicalGraphSnapshot) {
	sort.Slice(s.Sections, func(i, j int) bool { return s.Sections[i].SectionCode < s.Sections[j].SectionCode })
	sort.Slice(s.Questions, func(i, j int) bool {
		if s.Questions[i].SectionCode != s.Questions[j].SectionCode {
			return s.Questions[i].SectionCode < s.Questions[j].SectionCode
		}
		return s.Questions[i].QuestionCode < s.Questions[j].QuestionCode
	})
	sort.Slice(s.Options, func(i, j int) bool {
		if s.Options[i].SectionCode != s.Options[j].SectionCode {
			return s.Options[i].SectionCode < s.Options[j].SectionCode
		}
		if s.Options[i].QuestionCode != s.Options[j].QuestionCode {
			return s.Options[i].QuestionCode < s.Options[j].QuestionCode
		}
		return s.Options[i].OptionCode < s.Options[j].OptionCode
	})
	sort.Slice(s.Rules, func(i, j int) bool { return s.Rules[i].RuleCode < s.Rules[j].RuleCode })
}

func assertCanonicalGraphEqual(t *testing.T, left, right canonicalGraphSnapshot) {
	t.Helper()
	leftJSON, err := json.Marshal(left)
	if err != nil {
		t.Fatalf("marshal left snapshot: %v", err)
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		t.Fatalf("marshal right snapshot: %v", err)
	}
	if !bytes.Equal(leftJSON, rightJSON) {
		t.Fatalf("canonical graph mismatch:\nleft=%s\nright=%s", string(leftJSON), string(rightJSON))
	}
}

func assertTemplateAndEventCanonicalEqual(t *testing.T, tmpl *domain.TemplateQuestionnaireDefinition, event *domain.QuestionnaireDefinition) {
	t.Helper()
	assertCanonicalGraphEqual(t, canonicalFromTemplateGraph(tmpl), canonicalFromEventGraph(event))
}

func eventGraphUUIDSets(def *domain.QuestionnaireDefinition) graphUUIDSets {
	out := graphUUIDSets{}
	for _, section := range def.Sections {
		out.SectionIDs = append(out.SectionIDs, section.Section.ID)
		for _, question := range section.Questions {
			out.QuestionIDs = append(out.QuestionIDs, question.ID)
			for _, option := range question.Options {
				out.OptionIDs = append(out.OptionIDs, option.ID)
			}
		}
	}
	for _, rule := range def.Rules {
		out.RuleIDs = append(out.RuleIDs, rule.ID)
	}
	return out
}

func templateGraphUUIDSets(def *domain.TemplateQuestionnaireDefinition) graphUUIDSets {
	out := graphUUIDSets{}
	for _, section := range def.Sections {
		out.SectionIDs = append(out.SectionIDs, section.Section.ID)
		for _, question := range section.Questions {
			out.QuestionIDs = append(out.QuestionIDs, question.ID)
			for _, option := range question.Options {
				out.OptionIDs = append(out.OptionIDs, option.ID)
			}
		}
	}
	for _, rule := range def.Rules {
		out.RuleIDs = append(out.RuleIDs, rule.ID)
	}
	return out
}

func assertGraphUUIDSetsDisjoint(t *testing.T, tmpl *domain.TemplateQuestionnaireDefinition, event *domain.QuestionnaireDefinition) {
	t.Helper()
	tmplIDs := templateGraphUUIDSets(tmpl)
	eventIDs := eventGraphUUIDSets(event)
	assertNoUUIDOverlap(t, "section", tmplIDs.SectionIDs, eventIDs.SectionIDs)
	assertNoUUIDOverlap(t, "question", tmplIDs.QuestionIDs, eventIDs.QuestionIDs)
	assertNoUUIDOverlap(t, "option", tmplIDs.OptionIDs, eventIDs.OptionIDs)
	assertNoUUIDOverlap(t, "rule", tmplIDs.RuleIDs, eventIDs.RuleIDs)
}

func assertNoUUIDOverlap(t *testing.T, kind string, left, right []uuid.UUID) {
	t.Helper()
	rightSet := make(map[uuid.UUID]struct{}, len(right))
	for _, id := range right {
		rightSet[id] = struct{}{}
	}
	for _, id := range left {
		if _, ok := rightSet[id]; ok {
			t.Fatalf("%s UUID overlap between template and event graph: %s", kind, id)
		}
	}
}

func assertEventRuleTargetsAreEventLocal(t *testing.T, event *domain.QuestionnaireDefinition) {
	t.Helper()
	eventQuestionIDs := make(map[uuid.UUID]struct{})
	for _, section := range event.Sections {
		for _, question := range section.Questions {
			eventQuestionIDs[question.ID] = struct{}{}
		}
	}
	for _, rule := range event.Rules {
		if rule.TargetQuestionID == nil {
			continue
		}
		if _, ok := eventQuestionIDs[*rule.TargetQuestionID]; !ok {
			t.Fatalf("rule %s target %s is not an event-local question UUID", rule.RuleCode, rule.TargetQuestionID)
		}
	}
}

func assertEventGraphUsesNoTemplateUUIDs(t *testing.T, tmpl *domain.TemplateQuestionnaireDefinition, event *domain.QuestionnaireDefinition) {
	t.Helper()
	tmplIDs := templateGraphUUIDSets(tmpl)
	eventIDs := eventGraphUUIDSets(event)
	allTemplate := append(append(append(tmplIDs.SectionIDs, tmplIDs.QuestionIDs...), tmplIDs.OptionIDs...), tmplIDs.RuleIDs...)
	allEvent := append(append(append(eventIDs.SectionIDs, eventIDs.QuestionIDs...), eventIDs.OptionIDs...), eventIDs.RuleIDs...)
	eventSet := make(map[uuid.UUID]struct{}, len(allEvent))
	for _, id := range allEvent {
		eventSet[id] = struct{}{}
	}
	for _, templateID := range allTemplate {
		if _, ok := eventSet[templateID]; ok {
			t.Fatalf("event graph references template UUID %s", templateID)
		}
	}
}

type templatePublishedTimestamps struct {
	SectionUpdatedAt map[string]string
}

func captureTemplatePublishedTimestamps(def *domain.TemplateQuestionnaireDefinition) templatePublishedTimestamps {
	out := templatePublishedTimestamps{SectionUpdatedAt: make(map[string]string)}
	for _, item := range def.Sections {
		out.SectionUpdatedAt[item.Section.SectionCode] = item.Section.UpdatedAt.UTC().Format(timeRFC3339Nano)
	}
	return out
}

func assertTemplatePublishedTimestampsUnchanged(t *testing.T, before, after templatePublishedTimestamps) {
	t.Helper()
	for code, ts := range before.SectionUpdatedAt {
		got, ok := after.SectionUpdatedAt[code]
		if !ok {
			t.Fatalf("published section %s missing after event mutation", code)
		}
		if got != ts {
			t.Fatalf("published section %s updated_at changed: before=%s after=%s", code, ts, got)
		}
	}
}

const timeRFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"

type publishedTemplateContract struct {
	VersionID     uuid.UUID
	VersionNumber int
	PublishedAt   string
	UpdatedAt     string
	Canonical     canonicalGraphSnapshot
	Timestamps    templatePublishedTimestamps
}

func capturePublishedTemplateContract(def *domain.TemplateQuestionnaireDefinition, published *domain.RfxTemplateVersion) publishedTemplateContract {
	out := publishedTemplateContract{
		VersionID:     published.ID,
		VersionNumber: published.VersionNumber,
		Canonical:     canonicalFromTemplateGraph(def),
		Timestamps:    captureTemplatePublishedTimestamps(def),
	}
	if published.PublishedAt != nil {
		out.PublishedAt = published.PublishedAt.UTC().Format(timeRFC3339Nano)
	}
	out.UpdatedAt = published.UpdatedAt.UTC().Format(timeRFC3339Nano)
	return out
}

func assertPublishedTemplateContractUnchanged(t *testing.T, before, after publishedTemplateContract) {
	t.Helper()
	if before.VersionID != after.VersionID {
		t.Fatalf("published version id changed: before=%s after=%s", before.VersionID, after.VersionID)
	}
	if before.VersionNumber != after.VersionNumber {
		t.Fatalf("published version number changed: before=%d after=%d", before.VersionNumber, after.VersionNumber)
	}
	if before.PublishedAt != after.PublishedAt {
		t.Fatalf("published_at changed: before=%s after=%s", before.PublishedAt, after.PublishedAt)
	}
	if before.UpdatedAt != after.UpdatedAt {
		t.Fatalf("published version updated_at changed: before=%s after=%s", before.UpdatedAt, after.UpdatedAt)
	}
	assertCanonicalGraphEqual(t, before.Canonical, after.Canonical)
	assertTemplatePublishedTimestampsUnchanged(t, before.Timestamps, after.Timestamps)
}

func loadEventGraphSnapshot(t *testing.T, env *testEnv, fix buyerFixture, versionID uuid.UUID) (canonicalGraphSnapshot, *domain.QuestionnaireDefinition) {
	t.Helper()
	graph, err := env.qRepo.LoadQuestionnaire(context.Background(), versionID, fix.TenantID)
	if err != nil {
		t.Fatalf("load event graph: %v", err)
	}
	return canonicalFromEventGraph(graph), graph
}

func loadTemplateGraphSnapshot(t *testing.T, env *testEnv, fix buyerFixture, templateID, versionID uuid.UUID) (canonicalGraphSnapshot, *domain.TemplateQuestionnaireDefinition) {
	t.Helper()
	graph, err := env.tmplQRepo.LoadQuestionnaire(context.Background(), templateID, versionID, fix.TenantID)
	if err != nil {
		t.Fatalf("load template graph: %v", err)
	}
	return canonicalFromTemplateGraph(graph), graph
}
