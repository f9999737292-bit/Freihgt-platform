package xlsxexchange

import "encoding/json"

// ERPLotDraft is the ERP UPDATE lot projection used to reuse buyer reconcile.
type ERPLotDraft struct {
	LotNumber      string
	Name           string
	Description    string
	Category       string
	EstimatedValue *float64
	CurrencyCode   string
}

// ERPSectionDraft is an exported ERP section row for questionnaire reconcile.
type ERPSectionDraft struct {
	SectionCode string
	Title       string
	Description string
	SortOrder   int
}

// ERPQuestionDraft is an exported ERP question row for questionnaire reconcile.
type ERPQuestionDraft struct {
	SectionCode    string
	QuestionCode   string
	QuestionType   string
	Label          string
	HelpText       string
	Required       bool
	SortOrder      int
	ValidationJSON json.RawMessage
}

// ERPOptionDraft is an exported ERP option row for questionnaire reconcile.
type ERPOptionDraft struct {
	QuestionCode string
	OptionCode   string
	Label        string
	SortOrder    int
}

// ERPRuleDraft is an exported ERP rule row for questionnaire reconcile.
type ERPRuleDraft struct {
	RuleCode           string
	SourceQuestionCode string
	TargetQuestionCode string
	Action             string
	ConditionJSON      json.RawMessage
	SortOrder          int
}

// ERPQuestionnaireDraft is the flattened ERP questionnaire projection for reconcile.
type ERPQuestionnaireDraft struct {
	Sections  []ERPSectionDraft
	Questions []ERPQuestionDraft
	Options   []ERPOptionDraft
	Rules     []ERPRuleDraft
}

// StoredImportPayloadFromERPGraph adapts ERP UPDATE lots/questionnaire to the XLSX reconcile payload.
func StoredImportPayloadFromERPGraph(lots []ERPLotDraft, questionnaire ERPQuestionnaireDraft) StoredImportPayload {
	outLots := make([]canonicalLot, 0, len(lots))
	for _, lot := range lots {
		outLots = append(outLots, canonicalLot{
			LotNumber:      lot.LotNumber,
			Name:           lot.Name,
			Description:    lot.Description,
			Category:       lot.Category,
			EstimatedValue: lot.EstimatedValue,
			CurrencyCode:   lot.CurrencyCode,
			Status:         "ACTIVE",
		})
	}
	sections := make([]canonicalSection, 0, len(questionnaire.Sections))
	for _, sec := range questionnaire.Sections {
		sections = append(sections, canonicalSection{
			SectionCode: sec.SectionCode,
			Title:       sec.Title,
			Description: sec.Description,
			SortOrder:   sec.SortOrder,
		})
	}
	questions := make([]canonicalQuestion, 0, len(questionnaire.Questions))
	for _, q := range questionnaire.Questions {
		questions = append(questions, canonicalQuestion{
			SectionCode:    q.SectionCode,
			QuestionCode:   q.QuestionCode,
			QuestionType:   q.QuestionType,
			Label:          q.Label,
			HelpText:       q.HelpText,
			Required:       q.Required,
			SortOrder:      q.SortOrder,
			ValidationJSON: q.ValidationJSON,
		})
	}
	options := make([]canonicalOption, 0, len(questionnaire.Options))
	for _, opt := range questionnaire.Options {
		options = append(options, canonicalOption{
			QuestionCode: opt.QuestionCode,
			OptionCode:   opt.OptionCode,
			Label:        opt.Label,
			SortOrder:    opt.SortOrder,
		})
	}
	rules := make([]canonicalRule, 0, len(questionnaire.Rules))
	for _, rule := range questionnaire.Rules {
		rules = append(rules, canonicalRule{
			RuleCode:           rule.RuleCode,
			SourceQuestionCode: rule.SourceQuestionCode,
			TargetQuestionCode: rule.TargetQuestionCode,
			Action:             rule.Action,
			ConditionJSON:      rule.ConditionJSON,
			SortOrder:          rule.SortOrder,
		})
	}
	return StoredImportPayload{
		Lots: outLots,
		Questionnaire: canonicalQuestionnaire{
			Sections:  sections,
			Questions: questions,
			Options:   options,
			Rules:     rules,
		},
	}
}
