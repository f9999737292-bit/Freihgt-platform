//go:build integration

package templatelibrary

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func assertE6HistoricalGraphPG(t *testing.T, def *domain.TemplateQuestionnaireDefinition, expect e6HistoricalGraphExpectation) {
	t.Helper()
	if def == nil {
		t.Fatal("nil questionnaire definition")
	}
	if def.TemplateID == uuid.Nil || def.RfxTemplateVersionID == uuid.Nil {
		t.Fatal("expected template and version identifiers on PG graph")
	}
	if def.VersionNumber < 1 {
		t.Fatalf("expected version_number >= 1, got %d", def.VersionNumber)
	}
	if def.VersionStatus == "" {
		t.Fatal("expected version_status on PG graph")
	}
	if len(def.Sections) != len(expect.Sections) {
		t.Fatalf("section count: got %d want %d", len(def.Sections), len(expect.Sections))
	}
	for i, wantSec := range expect.Sections {
		gotSec := def.Sections[i]
		if gotSec.Section.SectionCode != wantSec.Code {
			t.Fatalf("section[%d] code: got %s want %s", i, gotSec.Section.SectionCode, wantSec.Code)
		}
		if gotSec.Section.SortOrder != wantSec.SortOrder {
			t.Fatalf("section[%d] %s sort_order: got %d want %d", i, wantSec.Code, gotSec.Section.SortOrder, wantSec.SortOrder)
		}
		if len(gotSec.Questions) != len(wantSec.Questions) {
			t.Fatalf("section[%d] %s question count: got %d want %d", i, wantSec.Code, len(gotSec.Questions), len(wantSec.Questions))
		}
		for j, wantQ := range wantSec.Questions {
			gotQ := gotSec.Questions[j]
			if gotQ.QuestionCode != wantQ.Code {
				t.Fatalf("section[%d] question[%d] code: got %s want %s", i, j, gotQ.QuestionCode, wantQ.Code)
			}
			if gotQ.QuestionType != wantQ.Type {
				t.Fatalf("section[%d] question[%d] %s type: got %s want %s", i, j, wantQ.Code, gotQ.QuestionType, wantQ.Type)
			}
			if gotQ.SortOrder != wantQ.SortOrder {
				t.Fatalf("section[%d] question[%d] %s sort_order: got %d want %d", i, j, wantQ.Code, gotQ.SortOrder, wantQ.SortOrder)
			}
			gotValidation := normalizeJSONField(gotQ.ValidationRuleJSON)
			if gotValidation != wantQ.ValidationRuleJSON {
				t.Fatalf("section[%d] question[%d] %s validation: got %s want %s", i, j, wantQ.Code, gotValidation, wantQ.ValidationRuleJSON)
			}
			if len(gotQ.Options) != len(wantQ.OptionCodes) {
				t.Fatalf("section[%d] question[%d] %s option count: got %d want %d", i, j, wantQ.Code, len(gotQ.Options), len(wantQ.OptionCodes))
			}
			for k, wantOpt := range wantQ.OptionCodes {
				if gotQ.Options[k].OptionCode != wantOpt {
					t.Fatalf("section[%d] question[%d] option[%d]: got %s want %s", i, j, k, gotQ.Options[k].OptionCode, wantOpt)
				}
			}
		}
	}
	assertE6HistoricalRulesPG(t, def, expect.Rules)
}

func assertE6HistoricalRulesPG(t *testing.T, def *domain.TemplateQuestionnaireDefinition, expectRules []e6ExpectedRule) {
	t.Helper()
	questionCodeByID := map[string]string{}
	for _, sec := range def.Sections {
		for _, q := range sec.Questions {
			questionCodeByID[q.ID.String()] = q.QuestionCode
		}
	}
	if len(def.Rules) != len(expectRules) {
		t.Fatalf("rule count: got %d want %d", len(def.Rules), len(expectRules))
	}
	for i, wantRule := range expectRules {
		gotRule := def.Rules[i]
		if gotRule.RuleCode != wantRule.RuleCode {
			t.Fatalf("rule[%d] code: got %s want %s", i, gotRule.RuleCode, wantRule.RuleCode)
		}
		if gotRule.Action != wantRule.Action {
			t.Fatalf("rule[%d] %s action: got %s want %s", i, wantRule.RuleCode, gotRule.Action, wantRule.Action)
		}
		if gotRule.SortOrder != wantRule.SortOrder {
			t.Fatalf("rule[%d] %s sort_order: got %d want %d", i, wantRule.RuleCode, gotRule.SortOrder, wantRule.SortOrder)
		}
		targetCode := ""
		if gotRule.TargetQuestionID != nil {
			targetCode = questionCodeByID[gotRule.TargetQuestionID.String()]
		}
		if targetCode != wantRule.TargetQuestionCode {
			t.Fatalf("rule[%d] %s target: got %s want %s", i, wantRule.RuleCode, targetCode, wantRule.TargetQuestionCode)
		}
		gotCond := normalizeJSONField(gotRule.ConditionJSON)
		if gotCond != wantRule.ConditionJSON {
			t.Fatalf("rule[%d] %s condition: got %s want %s", i, wantRule.RuleCode, gotCond, wantRule.ConditionJSON)
		}
	}
}

func assertE6HistoricalGraphHTTP(t *testing.T, rec *httptest.ResponseRecorder, published *domain.RfxTemplateVersion, expect e6HistoricalGraphExpectation) {
	t.Helper()
	if rec.Code != 200 {
		t.Fatalf("expected HTTP 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode HTTP response: %v", err)
	}
	if got, _ := payload["template_id"].(string); got != published.TemplateID.String() {
		t.Fatalf("template_id: got %s want %s", got, published.TemplateID)
	}
	if got, _ := payload["rfx_template_version_id"].(string); got != published.ID.String() {
		t.Fatalf("rfx_template_version_id: got %s want %s", got, published.ID)
	}
	versionNumber, ok := payload["version_number"].(float64)
	if !ok || int(versionNumber) != published.VersionNumber {
		t.Fatalf("version_number: got %#v want %d", payload["version_number"], published.VersionNumber)
	}
	if got, _ := payload["version_status"].(string); got != published.Status {
		t.Fatalf("version_status: got %s want %s", got, published.Status)
	}

	sections, ok := payload["sections"].([]any)
	if !ok {
		t.Fatalf("sections: expected array, got %#v", payload["sections"])
	}
	if len(sections) != len(expect.Sections) {
		t.Fatalf("section count: got %d want %d", len(sections), len(expect.Sections))
	}
	questionCodeByID := map[string]string{}
	for i, wantSec := range expect.Sections {
		secItem, ok := sections[i].(map[string]any)
		if !ok {
			t.Fatalf("section[%d]: invalid item %#v", i, sections[i])
		}
		secMeta, ok := secItem["section"].(map[string]any)
		if !ok {
			t.Fatalf("section[%d]: missing section metadata", i)
		}
		if got, _ := secMeta["section_code"].(string); got != wantSec.Code {
			t.Fatalf("section[%d] code: got %s want %s", i, got, wantSec.Code)
		}
		if got := intFromAny(secMeta["sort_order"]); got != wantSec.SortOrder {
			t.Fatalf("section[%d] %s sort_order: got %d want %d", i, wantSec.Code, got, wantSec.SortOrder)
		}
		questions, ok := secItem["questions"].([]any)
		if !ok {
			t.Fatalf("section[%d] %s questions: expected array", i, wantSec.Code)
		}
		if len(questions) != len(wantSec.Questions) {
			t.Fatalf("section[%d] %s question count: got %d want %d", i, wantSec.Code, len(questions), len(wantSec.Questions))
		}
		for j, wantQ := range wantSec.Questions {
			qItem, ok := questions[j].(map[string]any)
			if !ok {
				t.Fatalf("section[%d] question[%d]: invalid item", i, j)
			}
			if got, _ := qItem["question_code"].(string); got != wantQ.Code {
				t.Fatalf("section[%d] question[%d] code: got %s want %s", i, j, got, wantQ.Code)
			}
			if got, _ := qItem["question_type"].(string); got != wantQ.Type {
				t.Fatalf("section[%d] question[%d] %s type: got %s want %s", i, j, wantQ.Code, got, wantQ.Type)
			}
			if got := intFromAny(qItem["sort_order"]); got != wantQ.SortOrder {
				t.Fatalf("section[%d] question[%d] %s sort_order: got %d want %d", i, j, wantQ.Code, got, wantQ.SortOrder)
			}
			if qID, _ := qItem["id"].(string); qID != "" {
				questionCodeByID[qID] = wantQ.Code
			}
			gotValidation := normalizeJSONField(mustJSONRaw(qItem["validation_rule_json"]))
			if gotValidation != wantQ.ValidationRuleJSON {
				t.Fatalf("section[%d] question[%d] %s validation: got %s want %s", i, j, wantQ.Code, gotValidation, wantQ.ValidationRuleJSON)
			}
			options, ok := qItem["options"].([]any)
			if !ok {
				t.Fatalf("section[%d] question[%d] %s options: expected array", i, j, wantQ.Code)
			}
			if len(options) != len(wantQ.OptionCodes) {
				t.Fatalf("section[%d] question[%d] %s option count: got %d want %d", i, j, wantQ.Code, len(options), len(wantQ.OptionCodes))
			}
			for k, wantOpt := range wantQ.OptionCodes {
				optItem, ok := options[k].(map[string]any)
				if !ok {
					t.Fatalf("section[%d] question[%d] option[%d]: invalid item", i, j, k)
				}
				if got, _ := optItem["option_code"].(string); got != wantOpt {
					t.Fatalf("section[%d] question[%d] option[%d]: got %s want %s", i, j, k, got, wantOpt)
				}
			}
		}
	}

	rules, ok := payload["rules"].([]any)
	if !ok {
		t.Fatalf("rules: expected array, got %#v", payload["rules"])
	}
	if len(rules) != len(expect.Rules) {
		t.Fatalf("rule count: got %d want %d", len(rules), len(expect.Rules))
	}
	for i, wantRule := range expect.Rules {
		ruleItem, ok := rules[i].(map[string]any)
		if !ok {
			t.Fatalf("rule[%d]: invalid item", i)
		}
		if got, _ := ruleItem["rule_code"].(string); got != wantRule.RuleCode {
			t.Fatalf("rule[%d] code: got %s want %s", i, got, wantRule.RuleCode)
		}
		if got, _ := ruleItem["action"].(string); got != wantRule.Action {
			t.Fatalf("rule[%d] %s action: got %s want %s", i, wantRule.RuleCode, got, wantRule.Action)
		}
		if got := intFromAny(ruleItem["sort_order"]); got != wantRule.SortOrder {
			t.Fatalf("rule[%d] %s sort_order: got %d want %d", i, wantRule.RuleCode, got, wantRule.SortOrder)
		}
		targetID, _ := ruleItem["target_question_id"].(string)
		targetCode := questionCodeByID[targetID]
		if targetCode != wantRule.TargetQuestionCode {
			t.Fatalf("rule[%d] %s target: got %s want %s", i, wantRule.RuleCode, targetCode, wantRule.TargetQuestionCode)
		}
		gotCond := normalizeJSONField(mustJSONRaw(ruleItem["condition_json"]))
		if gotCond != wantRule.ConditionJSON {
			t.Fatalf("rule[%d] %s condition: got %s want %s", i, wantRule.RuleCode, gotCond, wantRule.ConditionJSON)
		}
	}
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

func mustJSONRaw(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal json field: %v", err))
	}
	return b
}
