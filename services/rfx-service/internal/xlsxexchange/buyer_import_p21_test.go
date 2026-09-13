package xlsxexchange

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

func TestProductionLimitsFixed(t *testing.T) {
	sec := productionSecurityLimits()
	if sec.MaxUploadBytes != xlsxsecurity.DefaultMaxUploadBytes {
		t.Fatalf("upload limit drift: %d", sec.MaxUploadBytes)
	}
	if sec.MaxZipEntries != xlsxsecurity.DefaultMaxZipEntries {
		t.Fatalf("zip entries drift: %d", sec.MaxZipEntries)
	}
	if sec.MaxExpandedBytes != xlsxsecurity.DefaultMaxExpandedBytes {
		t.Fatalf("expanded bytes drift: %d", sec.MaxExpandedBytes)
	}
	if sec.MaxZipRatio != xlsxsecurity.DefaultMaxZipRatio {
		t.Fatalf("zip ratio drift: %v", sec.MaxZipRatio)
	}

	imp := productionImportLimits()
	if imp.MaxRowsPerSheet != 10_000 || imp.MaxTotalCells != 500_000 || imp.MaxStringLength != 8_192 {
		t.Fatalf("import row/cell/string limits drift: %+v", imp)
	}
	if imp.MaxLots != 500 || imp.MaxSections != 500 || imp.MaxQuestions != 5_000 || imp.MaxOptions != 20_000 || imp.MaxRules != 5_000 {
		t.Fatalf("entity limits drift: %+v", imp)
	}
}

func TestParseBuyerImportPreviewPublicAPISurface(t *testing.T) {
	fn := reflect.ValueOf(ParseBuyerImportPreview)
	if fn.Kind() != reflect.Func {
		t.Fatal("expected function")
	}
	if fn.Type().NumIn() != 3 {
		t.Fatalf("public parser must accept ctx, bytes, target only; got %d params", fn.Type().NumIn())
	}
	if fn.Type().NumOut() != 2 {
		t.Fatalf("expected preview and error; got %d returns", fn.Type().NumOut())
	}

	var openCalls atomic.Int32
	cfg := productionParseConfig()
	cfg.openWorkbook = func([]byte) (workbookReader, error) {
		openCalls.Add(1)
		return nil, errors.New("blocked")
	}
	_, _ = parseBuyerImportPreview(context.Background(), []byte("not-zip"), minimalTarget(), cfg)
	if openCalls.Load() != 0 {
		t.Fatalf("security gate must run before workbook open")
	}
	_, _ = ParseBuyerImportPreview(context.Background(), []byte("not-zip"), minimalTarget())
	if openCalls.Load() != 0 {
		t.Fatalf("public API must not accept custom workbook opener")
	}
}

func TestIssueCollectorCapExactlyLimit(t *testing.T) {
	collector := newIssueCollector()
	for i := 0; i < MaxPreviewIssues-1; i++ {
		if !collector.addError(issueError(MachineCodeMissingRequiredValue, "rfx.test.issue", sheetSections, "section_code", fmt.Sprintf("C%d", i), i, nil)) {
			t.Fatalf("failed to add issue %d before cap", i)
		}
	}
	errors, warnings := collector.sorted()
	if collector.truncated || countIssuesWithCode(errors, MachineCodeIssueLimitReached) != 0 {
		t.Fatalf("unexpected truncation at %d regular issues", len(errors))
	}
	if len(errors) != MaxPreviewIssues-1 || len(warnings) != 0 {
		t.Fatalf("expected %d errors, got errors=%d warnings=%d", MaxPreviewIssues-1, len(errors), len(warnings))
	}
}

func TestIssueCollectorCapLimitPlusOne(t *testing.T) {
	collector := newIssueCollector()
	for i := 0; i < MaxPreviewIssues; i++ {
		collector.addError(issueError(MachineCodeMissingRequiredValue, "rfx.test.issue", sheetSections, "section_code", fmt.Sprintf("C%d", i), i, nil))
	}
	errors, _ := collector.sorted()
	if len(errors) != MaxPreviewIssues {
		t.Fatalf("expected %d total issues, got %d", MaxPreviewIssues, len(errors))
	}
	if countIssuesWithCode(errors, MachineCodeIssueLimitReached) != 1 {
		t.Fatalf("expected one truncation issue")
	}
	last := errors[len(errors)-1]
	if last.MachineCode != MachineCodeIssueLimitReached || last.Severity != IssueSeverityError {
		t.Fatalf("truncation issue contract: %+v", last)
	}
	if last.Params["limit"] != MaxPreviewIssues {
		t.Fatalf("truncation limit param: %+v", last.Params)
	}
}

func TestIssueCollectorCapWellAboveLimitDeterministic(t *testing.T) {
	build := func() []BuyerImportIssue {
		collector := newIssueCollector()
		for i := 0; i < MaxPreviewIssues+500; i++ {
			collector.addError(issueError(MachineCodeMissingRequiredValue, "rfx.test.issue", sheetSections, "section_code", fmt.Sprintf("C%d", i), i, nil))
		}
		errors, _ := collector.sorted()
		return errors
	}
	first := build()
	second := build()
	if !reflect.DeepEqual(first, second) {
		t.Fatal("truncated issue list must be deterministic")
	}
	if len(first) != MaxPreviewIssues {
		t.Fatalf("expected capped list length %d, got %d", MaxPreviewIssues, len(first))
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal errors: %v", err)
	}
	const maxSerializedErrors = 2_500_000
	if len(encoded) > maxSerializedErrors {
		t.Fatalf("serialized errors exceed bounded size: %d", len(encoded))
	}
}

func TestIssueCapIntegrationTruncatesLargeWorkbook(t *testing.T) {
	data := workbookWithManyErrors(t, MaxPreviewIssues+500)
	preview := parseWorkbook(t, data, richTarget())
	if len(preview.Errors) > MaxPreviewIssues {
		t.Fatalf("issue list exceeded cap: %d", len(preview.Errors))
	}
	if countIssuesWithCode(preview.Errors, MachineCodeIssueLimitReached) != 1 {
		t.Fatalf("expected one truncation issue")
	}
	if preview.ReadyToCommit || preview.CanonicalPayloadHash != "" {
		t.Fatal("truncated preview must not be ready or hashed")
	}
	p2 := parseWorkbook(t, data, richTarget())
	if !reflect.DeepEqual(preview.Errors, p2.Errors) {
		t.Fatal("truncated parser output must be deterministic")
	}
}

func TestIssueCapPreservesLeadingOrder(t *testing.T) {
	data := workbookWithSectionErrors(t, MaxPreviewIssues+10)
	preview := parseWorkbook(t, data, richTarget())
	if len(preview.Errors) < 2 {
		t.Fatal("expected multiple issues")
	}
	if preview.Errors[0].MachineCode == MachineCodeIssueLimitReached {
		t.Fatal("truncation must not be first")
	}
	if preview.Errors[len(preview.Errors)-1].MachineCode != MachineCodeIssueLimitReached {
		t.Fatal("truncation must be appended after sort")
	}
}

func TestParseBuyerImportPreviewContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var rowUnits atomic.Int32
	cfg := productionParseConfig()
	cfg.openWorkbook = func(data []byte) (workbookReader, error) {
		inner, err := defaultOpenWorkbook(data)
		if err != nil {
			return nil, err
		}
		return cancelProbeWorkbook{inner: inner, cancel: cancel, rowUnits: &rowUnits}, nil
	}

	data := workbookWithSectionErrors(t, 300)
	_, err := parseBuyerImportPreview(ctx, data, richTarget(), cfg)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v (row units=%d)", err, rowUnits.Load())
	}
}

type cancelProbeWorkbook struct {
	inner    workbookReader
	cancel   context.CancelFunc
	rowUnits *atomic.Int32
}

func (w cancelProbeWorkbook) GetSheetList() []string { return w.inner.GetSheetList() }
func (w cancelProbeWorkbook) GetSheetIndex(sheet string) (int, error) {
	return w.inner.GetSheetIndex(sheet)
}
func (w cancelProbeWorkbook) GetSheetVisible(sheet string) (excelizeSheetVisibility, bool, error) {
	return w.inner.GetSheetVisible(sheet)
}
func (w cancelProbeWorkbook) GetMergeCells(sheet string) ([]mergeCellRange, error) {
	return w.inner.GetMergeCells(sheet)
}
func (w cancelProbeWorkbook) GetRows(sheet string) ([][]string, error) {
	rows, err := w.inner.GetRows(sheet)
	if err == nil {
		w.rowUnits.Add(int32(len(rows)))
		if w.rowUnits.Load() >= contextCheckRowInterval {
			w.cancel()
		}
	}
	return rows, err
}
func (w cancelProbeWorkbook) GetCellFormula(sheet, cell string) (string, error) {
	return w.inner.GetCellFormula(sheet, cell)
}
func (w cancelProbeWorkbook) GetCellValue(sheet, cell string) (string, error) {
	return w.inner.GetCellValue(sheet, cell)
}
func (w cancelProbeWorkbook) GetRowVisible(sheet string, row int) (bool, error) {
	return w.inner.GetRowVisible(sheet, row)
}
func (w cancelProbeWorkbook) GetColVisible(sheet, col string) (bool, error) {
	return w.inner.GetColVisible(sheet, col)
}
func (w cancelProbeWorkbook) Close() error { return w.inner.Close() }

func TestExportImportSemanticRoundTrip(t *testing.T) {
	snapshot := fullRoundTripSnapshot()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	target := targetFromSnapshot(snapshot)
	preview, err := ParseBuyerImportPreview(context.Background(), data, target)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !preview.ReadyToCommit {
		t.Fatalf("expected ready preview, errors=%v warnings=%v", preview.Errors, preview.Warnings)
	}
	assertLotsRoundTrip(t, snapshot.Lots, preview.Proposal.Lots)
	assertSectionsRoundTrip(t, snapshot.Sections, preview.Proposal.Questionnaire.Sections)
	assertQuestionsRoundTrip(t, snapshot, preview.Proposal.Questionnaire.Sections)
	assertOptionsRoundTrip(t, snapshot.Options, preview.Proposal.Questionnaire.Sections)
	assertRulesRoundTrip(t, snapshot.Rules, preview.Proposal.Questionnaire.Rules)
}

func TestParseBuyerImportPreviewMultiNodeCycle(t *testing.T) {
	snapshot := cycleSnapshot()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	target := targetFromSnapshot(snapshot)
	p1, err := ParseBuyerImportPreview(context.Background(), data, target)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p2, _ := ParseBuyerImportPreview(context.Background(), data, target)
	if !reflect.DeepEqual(p1.Errors, p2.Errors) {
		t.Fatal("cycle detection must be deterministic")
	}
	assertIssueCode(t, p1.Errors, MachineCodeCyclicRule)
	if p1.ReadyToCommit || p1.CanonicalPayloadHash != "" {
		t.Fatal("cycle must block commit and hash")
	}
}

func TestCanonicalHashSensitivityMatrix(t *testing.T) {
	base := fullRoundTripSnapshot()
	baseData := GenerateWorkbookOrFail(t, base)
	baseTarget := targetFromSnapshot(base)
	basePreview, err := ParseBuyerImportPreview(context.Background(), baseData, baseTarget)
	if err != nil || !basePreview.ReadyToCommit {
		t.Fatalf("baseline parse failed: err=%v errors=%v", err, basePreview.Errors)
	}
	baseHash := basePreview.CanonicalPayloadHash

	cases := []struct {
		name string
		mut  func(*BuyerDraftSnapshot, *TargetDraftBaseline)
	}{
		{"lot_name", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) { s.Lots[0].Name = "Changed lot" }},
		{"lot_order", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) {
			s.Lots[0], s.Lots[1] = s.Lots[1], s.Lots[0]
		}},
		{"section_title", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) { s.Sections[0].Title = "New title" }},
		{"section_sort", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) { s.Sections[0].SortOrder = 99 }},
		{"question_title", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) { s.Questions[0].Question.Label = "New label" }},
		{"question_type", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) {
			s.Questions[1].Question.QuestionType = domain.QuestionTypeLongText
			s.Options = nil
		}},
		{"required", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) { s.Questions[0].Question.Required = false }},
		{"validation_json", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) {
			s.Questions[2].Question.ValidationRuleJSON = json.RawMessage(`{"min":2,"max":20}`)
		}},
		{"option_label", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) { s.Options[0].Option.Label = "Alt" }},
		{"option_sort", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) { s.Options[1].Option.SortOrder = 9 }},
		{"rule_condition", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) {
			s.Rules[0].ConditionJSON = json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q2","value":"x"}`)
		}},
		{"rule_action", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) { s.Rules[0].Action = domain.RuleActionHide }},
		{"rule_source", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) { s.Rules[0].SourceQuestionCode = "Q1" }},
		{"rule_target", func(s *BuyerDraftSnapshot, _ *TargetDraftBaseline) { s.Rules[0].TargetQuestionCode = "Q1" }},
		{"target_event_id", func(_ *BuyerDraftSnapshot, target *TargetDraftBaseline) { target.EventID = uuid.New() }},
		{"target_draft_id", func(_ *BuyerDraftSnapshot, target *TargetDraftBaseline) { target.DraftVersionID = uuid.New() }},
		{"target_version_number", func(_ *BuyerDraftSnapshot, target *TargetDraftBaseline) { target.DraftVersionNumber++ }},
		{"event_row_version", func(_ *BuyerDraftSnapshot, target *TargetDraftBaseline) { target.EventRowVersion++ }},
		{"draft_row_version", func(_ *BuyerDraftSnapshot, target *TargetDraftBaseline) { target.DraftRowVersion++ }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := fullRoundTripSnapshot()
			target := targetFromSnapshot(snapshot)
			tc.mut(&snapshot, &target)
			data, err := GenerateBuyerDraftWorkbook(snapshot)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			preview, err := ParseBuyerImportPreview(context.Background(), data, target)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !preview.ReadyToCommit {
				t.Fatalf("expected ready preview for %s: %v", tc.name, preview.Errors)
			}
			if preview.CanonicalPayloadHash == baseHash {
				t.Fatalf("hash must change for %s", tc.name)
			}
		})
	}
}

func TestCanonicalHashValidationJSONKeyOrderIndependent(t *testing.T) {
	snapshot := fullRoundTripSnapshot()
	snapshot.Questions[2].Question.ValidationRuleJSON = json.RawMessage(`{"max":10,"min":1}`)
	dataA, _ := GenerateBuyerDraftWorkbook(snapshot)
	snapshot.Questions[2].Question.ValidationRuleJSON = json.RawMessage(`{"min":1,"max":10}`)
	dataB, _ := GenerateBuyerDraftWorkbook(snapshot)
	target := targetFromSnapshot(snapshot)
	pA, _ := ParseBuyerImportPreview(context.Background(), dataA, target)
	pB, _ := ParseBuyerImportPreview(context.Background(), dataB, target)
	if pA.CanonicalPayloadHash != pB.CanonicalPayloadHash {
		t.Fatal("equivalent validation JSON must hash identically")
	}
}

func TestInspectUploadRejectsExternalDefinedNameEvidence(t *testing.T) {
	data := buildWorkbookWithDefinedName(t, "[OtherBook]Sheet1!$A$1")
	if _, err := xlsxsecurity.InspectUpload("", data, xlsxsecurity.DefaultLimits()); err == nil {
		t.Fatal("expected external defined name rejection")
	}
}

func TestInspectUploadRejectsFormulaDefinedNameEvidence(t *testing.T) {
	data := buildWorkbookWithDefinedName(t, "=SUM(A1:A2)")
	if _, err := xlsxsecurity.InspectUpload("", data, xlsxsecurity.DefaultLimits()); err == nil {
		t.Fatal("expected formula defined name rejection")
	}
}

func TestInspectUploadAcceptsLocalRangeDefinedNameEvidence(t *testing.T) {
	data := buildWorkbookWithDefinedName(t, "Sections!$A$1:$B$2")
	if _, err := xlsxsecurity.InspectUpload("", data, xlsxsecurity.DefaultLimits()); err != nil {
		t.Fatalf("expected local range defined name to be accepted: %v", err)
	}
}

func TestQuestionSectionCodeRoundTripExplicit(t *testing.T) {
	snapshot := fullRoundTripSnapshot()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview, err := ParseBuyerImportPreview(context.Background(), data, targetFromSnapshot(snapshot))
	if err != nil || !preview.ReadyToCommit {
		t.Fatalf("parse failed: err=%v errors=%v", err, preview.Errors)
	}
	wantByQuestion := map[string]string{}
	for _, item := range snapshot.Questions {
		wantByQuestion[item.Question.QuestionCode] = item.SectionCode
	}
	for _, swq := range preview.Proposal.Questionnaire.Sections {
		for _, q := range swq.Questions {
			if swq.Section.SectionCode != wantByQuestion[q.QuestionCode] {
				t.Fatalf("question %s bound to section %s want %s", q.QuestionCode, swq.Section.SectionCode, wantByQuestion[q.QuestionCode])
			}
		}
	}
}

func TestRuleTargetQuestionCodeRoundTripExplicit(t *testing.T) {
	snapshot := fullRoundTripSnapshot()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview, err := ParseBuyerImportPreview(context.Background(), data, targetFromSnapshot(snapshot))
	if err != nil || !preview.ReadyToCommit {
		t.Fatalf("parse failed: err=%v errors=%v", err, preview.Errors)
	}
	wantTarget := map[string]string{}
	for _, rule := range snapshot.Rules {
		wantTarget[rule.RuleCode] = rule.TargetQuestionCode
	}
	codeByID := map[uuid.UUID]string{}
	for _, swq := range preview.Proposal.Questionnaire.Sections {
		for _, q := range swq.Questions {
			codeByID[q.ID] = q.QuestionCode
		}
	}
	for _, rule := range preview.Proposal.Questionnaire.Rules {
		targetCode := ""
		if rule.TargetQuestionID != nil {
			targetCode = codeByID[*rule.TargetQuestionID]
		}
		if targetCode != wantTarget[rule.RuleCode] {
			t.Fatalf("rule %s target=%q want=%q", rule.RuleCode, targetCode, wantTarget[rule.RuleCode])
		}
	}
}

func TestUnicodeAndEmptyOptionalRoundTrip(t *testing.T) {
	snapshot := fullRoundTripSnapshot()
	for i := range snapshot.Lots {
		if snapshot.Lots[i].LotNumber == "L1" {
			snapshot.Lots[i].Description = nil
		}
	}
	snapshot.Questions[0].Question.Label = "Компания «БинТранс» 中文"
	snapshot.Options[0].Option.Label = "Да ✓"
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview, err := ParseBuyerImportPreview(context.Background(), data, targetFromSnapshot(snapshot))
	if err != nil || !preview.ReadyToCommit {
		t.Fatalf("parse failed: err=%v errors=%v", err, preview.Errors)
	}
	foundEmptyOptionalLot := false
	for _, lot := range preview.Proposal.Lots {
		if lot.LotNumber != "L1" {
			continue
		}
		if lot.Description == nil || strings.TrimSpace(stringPtrValue(lot.Description)) == "" {
			foundEmptyOptionalLot = true
		}
	}
	if !foundEmptyOptionalLot {
		t.Fatal("expected empty optional lot description for L1")
	}
	foundUnicode := false
	for _, swq := range preview.Proposal.Questionnaire.Sections {
		for _, q := range swq.Questions {
			if q.QuestionCode == "Q1" && q.Label == "Компания «БинТранс» 中文" {
				foundUnicode = true
			}
		}
	}
	if !foundUnicode {
		t.Fatal("unicode question label not round-tripped")
	}
}

func TestParseBuyerImportPreviewTwoIndependentRuleCycles(t *testing.T) {
	cases := []BuyerDraftSnapshot{cycleSnapshot(), independentCycleSnapshot()}
	for _, snapshot := range cases {
		data, err := GenerateBuyerDraftWorkbook(snapshot)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		preview, err := ParseBuyerImportPreview(context.Background(), data, targetFromSnapshot(snapshot))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		assertIssueCode(t, preview.Errors, MachineCodeCyclicRule)
	}
}

func independentCycleSnapshot() BuyerDraftSnapshot {
	snapshot := minimalSnapshot()
	snapshot.Sections = []domain.Section{{SectionCode: "SEC2", Title: "Independent cycle", SortOrder: 1}}
	snapshot.Questions = []BuyerDraftQuestion{
		{SectionCode: "SEC2", Question: domain.Question{QuestionCode: "Q1", QuestionType: domain.QuestionTypeText, Label: "A", Required: true, SortOrder: 1}},
		{SectionCode: "SEC2", Question: domain.Question{QuestionCode: "Q2", QuestionType: domain.QuestionTypeText, Label: "B", Required: true, SortOrder: 2}},
	}
	snapshot.Rules = []BuyerDraftRule{
		{RuleCode: "R1", SourceQuestionCode: "Q1", ConditionJSON: json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q1","value":"a"}`), TargetQuestionCode: "Q2", Action: domain.RuleActionShow, SortOrder: 1},
		{RuleCode: "R2", SourceQuestionCode: "Q2", ConditionJSON: json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q2","value":"b"}`), TargetQuestionCode: "Q1", Action: domain.RuleActionShow, SortOrder: 2},
	}
	return snapshot
}

func workbookWithManyErrors(t *testing.T, errorCount int) []byte {
	t.Helper()
	snapshot := minimalSnapshot()
	snapshot.Sections = []domain.Section{{SectionCode: "SEC0", Title: "Seed", SortOrder: 0}}
	perSheet := (errorCount / 3) + 1
	return mutateWorkbook(t, snapshot, func(f *excelize.File) {
		for i := 0; i < perSheet; i++ {
			row := i + 3
			_ = f.SetCellStr(sheetSections, fmt.Sprintf("A%d", row), "SEC0")
			_ = f.SetCellStr(sheetSections, fmt.Sprintf("B%d", row), fmt.Sprintf("Section %d", row))
			_ = f.SetCellStr(sheetSections, fmt.Sprintf("H%d", row), fmt.Sprintf("%d", row))
			_ = f.SetCellStr(sheetQuestions, fmt.Sprintf("A%d", row), "MISSING")
			_ = f.SetCellStr(sheetQuestions, fmt.Sprintf("B%d", row), fmt.Sprintf("Q%d", row))
			_ = f.SetCellStr(sheetLots, fmt.Sprintf("A%d", row), "")
			_ = f.SetCellStr(sheetLots, fmt.Sprintf("B%d", row), fmt.Sprintf("Lot %d", row))
		}
	})
}

func workbookWithSectionErrors(t *testing.T, errorRows int) []byte {
	t.Helper()
	snapshot := minimalSnapshot()
	snapshot.Sections = []domain.Section{{SectionCode: "SEC0", Title: "Seed", SortOrder: 0}}
	return mutateWorkbook(t, snapshot, func(f *excelize.File) {
		for i := 0; i < errorRows; i++ {
			row := i + 3
			_ = f.SetCellStr(sheetSections, fmt.Sprintf("A%d", row), "SEC0")
			_ = f.SetCellStr(sheetSections, fmt.Sprintf("B%d", row), fmt.Sprintf("Title %d", row))
			_ = f.SetCellStr(sheetSections, fmt.Sprintf("H%d", row), fmt.Sprintf("%d", row))
		}
	})
}

func buildWorkbookWithDefinedName(t *testing.T, definedValue string) []byte {
	t.Helper()
	base := GenerateWorkbookOrFail(t, minimalSnapshot())
	reader, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	workbookXML := `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><definedNames><definedName name="ExtRef">` + definedValue + `</definedName></definedNames></workbook>`
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open entry: %v", err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read entry: %v", err)
		}
		if file.Name == "xl/workbook.xml" {
			body = []byte(workbookXML)
		}
		w, err := zw.Create(file.Name)
		if err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func fullRoundTripSnapshot() BuyerDraftSnapshot {
	snapshot := minimalSnapshot()
	desc1 := "First lot"
	desc2 := "Second lot"
	category := "FREIGHT"
	value := 1500.0
	currency := "RUB"
	snapshot.Lots = []domain.RfxLot{
		{LotNumber: "L2", Name: "Secondary lane", Description: &desc2, Category: &category, EstimatedValue: &value, CurrencyCode: &currency, Status: "DRAFT"},
		{LotNumber: "L1", Name: "Primary lane", Description: &desc1, Category: &category, EstimatedValue: &value, CurrencyCode: &currency, Status: "DRAFT"},
	}
	snapshot.Sections = []domain.Section{
		{SectionCode: "SEC2", Title: "Commercial", Description: strPtr("Commercial terms"), SortOrder: 2},
		{SectionCode: "SEC1", Title: "General", Description: strPtr("General questions"), SortOrder: 1},
	}
	help := "Provide details"
	snapshot.Questions = []BuyerDraftQuestion{
		{SectionCode: "SEC1", Question: domain.Question{
			QuestionCode: "Q1", QuestionType: domain.QuestionTypeText, Label: "Company name", HelpText: &help, Required: true, SortOrder: 1,
		}},
		{SectionCode: "SEC1", Question: domain.Question{
			QuestionCode: "Q2", QuestionType: domain.QuestionTypeSingleSelect, Label: "Service level", Required: true, SortOrder: 2,
		}},
		{SectionCode: "SEC2", Question: domain.Question{
			QuestionCode: "Q3", QuestionType: domain.QuestionTypeNumber, Label: "Annual volume", Required: false, SortOrder: 1,
			ValidationRuleJSON: json.RawMessage(`{"min":1,"max":10}`),
		}},
	}
	snapshot.Options = []BuyerDraftOption{
		{QuestionCode: "Q2", Option: domain.QuestionOption{OptionCode: "O2", Label: "Premium", SortOrder: 2}},
		{QuestionCode: "Q2", Option: domain.QuestionOption{OptionCode: "O1", Label: "Standard", SortOrder: 1}},
	}
	snapshot.Rules = []BuyerDraftRule{
		{
			RuleCode: "R1", SourceQuestionCode: "Q2",
			ConditionJSON:      json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q2","value":"Standard"}`),
			TargetQuestionCode: "Q3", Action: domain.RuleActionShow, SortOrder: 1,
		},
	}
	return snapshot
}

func cycleSnapshot() BuyerDraftSnapshot {
	snapshot := minimalSnapshot()
	snapshot.Sections = []domain.Section{{SectionCode: "SEC1", Title: "Cycle", SortOrder: 1}}
	snapshot.Questions = []BuyerDraftQuestion{
		{SectionCode: "SEC1", Question: domain.Question{QuestionCode: "Q1", QuestionType: domain.QuestionTypeText, Label: "A", Required: true, SortOrder: 1}},
		{SectionCode: "SEC1", Question: domain.Question{QuestionCode: "Q2", QuestionType: domain.QuestionTypeText, Label: "B", Required: true, SortOrder: 2}},
		{SectionCode: "SEC1", Question: domain.Question{QuestionCode: "Q3", QuestionType: domain.QuestionTypeText, Label: "C", Required: true, SortOrder: 3}},
	}
	snapshot.Rules = []BuyerDraftRule{
		{RuleCode: "R1", SourceQuestionCode: "Q1", ConditionJSON: json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q1","value":"a"}`), TargetQuestionCode: "Q2", Action: domain.RuleActionShow, SortOrder: 1},
		{RuleCode: "R2", SourceQuestionCode: "Q2", ConditionJSON: json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q2","value":"b"}`), TargetQuestionCode: "Q3", Action: domain.RuleActionShow, SortOrder: 2},
		{RuleCode: "R3", SourceQuestionCode: "Q3", ConditionJSON: json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q3","value":"c"}`), TargetQuestionCode: "Q1", Action: domain.RuleActionShow, SortOrder: 3},
	}
	return snapshot
}

func strPtr(v string) *string { return &v }

func assertLotsRoundTrip(t *testing.T, want []domain.RfxLot, got []domain.RfxLot) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("lots count: got %d want %d", len(got), len(want))
	}
	byNumber := map[string]domain.RfxLot{}
	for _, lot := range got {
		byNumber[lot.LotNumber] = lot
	}
	for _, expected := range want {
		actual, ok := byNumber[expected.LotNumber]
		if !ok {
			t.Fatalf("missing lot %s", expected.LotNumber)
		}
		if actual.Name != expected.Name || stringPtrValue(actual.Description) != stringPtrValue(expected.Description) ||
			stringPtrValue(actual.Category) != stringPtrValue(expected.Category) ||
			floatPtrValue(actual.EstimatedValue) != floatPtrValue(expected.EstimatedValue) ||
			stringPtrValue(actual.CurrencyCode) != stringPtrValue(expected.CurrencyCode) ||
			actual.Status != expected.Status {
			t.Fatalf("lot %s mismatch: got %+v want %+v", expected.LotNumber, actual, expected)
		}
	}
}

func assertSectionsRoundTrip(t *testing.T, want []domain.Section, got []domain.SectionWithQuestions) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("sections count: got %d want %d", len(got), len(want))
	}
	byCode := map[string]domain.Section{}
	for _, swq := range got {
		byCode[swq.Section.SectionCode] = swq.Section
	}
	for _, expected := range want {
		actual, ok := byCode[expected.SectionCode]
		if !ok {
			t.Fatalf("missing section %s", expected.SectionCode)
		}
		if actual.Title != expected.Title || stringPtrValue(actual.Description) != stringPtrValue(expected.Description) || actual.SortOrder != expected.SortOrder {
			t.Fatalf("section %s mismatch: got %+v want %+v", expected.SectionCode, actual, expected)
		}
	}
}

func assertQuestionsRoundTrip(t *testing.T, snapshot BuyerDraftSnapshot, sections []domain.SectionWithQuestions) {
	t.Helper()
	gotByCode := map[string]domain.Question{}
	for _, swq := range sections {
		for _, q := range swq.Questions {
			gotByCode[q.QuestionCode] = q
		}
	}
	for _, item := range snapshot.Questions {
		got, ok := gotByCode[item.Question.QuestionCode]
		if !ok {
			t.Fatalf("missing question %s", item.Question.QuestionCode)
		}
		want := item.Question
		if got.QuestionType != want.QuestionType || got.Label != want.Label ||
			stringPtrValue(got.HelpText) != stringPtrValue(want.HelpText) ||
			got.Required != want.Required || got.SortOrder != want.SortOrder ||
			!jsonSemanticEqual(got.ValidationRuleJSON, want.ValidationRuleJSON) {
			t.Fatalf("question %s mismatch: got %+v want %+v", want.QuestionCode, got, want)
		}
	}
}

func assertOptionsRoundTrip(t *testing.T, want []BuyerDraftOption, sections []domain.SectionWithQuestions) {
	t.Helper()
	gotByKey := map[string]domain.QuestionOption{}
	for _, swq := range sections {
		for _, q := range swq.Questions {
			for _, opt := range q.Options {
				gotByKey[q.QuestionCode+"\x00"+opt.OptionCode] = opt
			}
		}
	}
	for _, item := range want {
		key := item.QuestionCode + "\x00" + item.Option.OptionCode
		got, ok := gotByKey[key]
		if !ok {
			t.Fatalf("missing option %s/%s", item.QuestionCode, item.Option.OptionCode)
		}
		if got.Label != item.Option.Label || got.SortOrder != item.Option.SortOrder {
			t.Fatalf("option %s mismatch: got %+v want %+v", key, got, item.Option)
		}
	}
}

func assertRulesRoundTrip(t *testing.T, want []BuyerDraftRule, got []domain.QuestionRule) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("rules count: got %d want %d", len(got), len(want))
	}
	byCode := map[string]domain.QuestionRule{}
	for _, rule := range got {
		byCode[rule.RuleCode] = rule
	}
	for _, expected := range want {
		actual, ok := byCode[expected.RuleCode]
		if !ok {
			t.Fatalf("missing rule %s", expected.RuleCode)
		}
		if actual.Action != expected.Action || actual.SortOrder != expected.SortOrder ||
			!bytes.Equal(actual.ConditionJSON, expected.ConditionJSON) {
			t.Fatalf("rule %s mismatch: got %+v want %+v", expected.RuleCode, actual, expected)
		}
	}
}

func stringPtrValue(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func floatPtrValue(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func jsonSemanticEqual(left, right json.RawMessage) bool {
	if len(left) == 0 && len(right) == 0 {
		return true
	}
	var a any
	var b any
	if err := json.Unmarshal(left, &a); err != nil {
		return bytes.Equal(left, right)
	}
	if err := json.Unmarshal(right, &b); err != nil {
		return bytes.Equal(left, right)
	}
	normalizedA, errA := json.Marshal(a)
	normalizedB, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return bytes.Equal(left, right)
	}
	return bytes.Equal(normalizedA, normalizedB)
}
