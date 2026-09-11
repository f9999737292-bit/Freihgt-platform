package xlsxexchange

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/xlsxsecurity"
)

func validImportSnapshot() BuyerDraftSnapshot {
	snapshot := richSnapshot()
	snapshot.Rules = nil
	snapshot.Questions[0].Question.QuestionType = domain.QuestionTypeSingleSelect
	return snapshot
}

func TestParseBuyerImportPreviewValidExportWorkbook(t *testing.T) {
	snapshot := validImportSnapshot()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	target := targetFromSnapshot(snapshot)
	preview, err := ParseBuyerImportPreview(context.Background(), data, target, ParserOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !preview.ReadyToCommit {
		t.Fatalf("expected ready_to_commit, errors=%v", preview.Errors)
	}
	if preview.CanonicalPayloadHash == "" || len(preview.CanonicalPayloadHash) != 64 {
		t.Fatalf("hash: %q", preview.CanonicalPayloadHash)
	}
	if preview.Mode != BuyerImportModeUpdateDraft {
		t.Fatalf("mode: %q", preview.Mode)
	}
}

func TestParseBuyerImportPreviewSecurityBeforeExcelize(t *testing.T) {
	var openCalls atomic.Int32
	blockOpen := func([]byte) (workbookReader, error) {
		openCalls.Add(1)
		return nil, errors.New("excelize should not run")
	}
	opts := ParserOptions{
		SecurityLimits: xlsxsecurity.DefaultLimits(),
		OpenWorkbook:   blockOpen,
	}

	cases := []struct {
		name  string
		bytes []byte
	}{
		{"invalid_zip", []byte("not-a-zip-file-content")},
		{"macro_package", buildForbiddenPackage(t, "xl/vbaProject.bin")},
		{"oversized", bytesRepeat('A', int(xlsxsecurity.DefaultMaxUploadBytes)+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			openCalls.Store(0)
			preview, err := ParseBuyerImportPreview(context.Background(), tc.bytes, minimalTarget(), opts)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if preview.ReadyToCommit {
				t.Fatal("expected invalid preview")
			}
			if openCalls.Load() != 0 {
				t.Fatalf("excelize open calls = %d", openCalls.Load())
			}
		})
	}

	t.Run("valid_workbook_calls_excelize_once", func(t *testing.T) {
		snapshot := validImportSnapshot()
		validData, err := GenerateBuyerDraftWorkbook(snapshot)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		openCalls.Store(0)
		validOpts := ParserOptions{SecurityLimits: xlsxsecurity.DefaultLimits()}
		validOpts.OpenWorkbook = func(data []byte) (workbookReader, error) {
			openCalls.Add(1)
			return defaultOpenWorkbook(data)
		}
		preview, err := ParseBuyerImportPreview(context.Background(), validData, targetFromSnapshot(snapshot), validOpts)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if openCalls.Load() != 1 {
			t.Fatalf("expected one excelize open, got %d", openCalls.Load())
		}
		if !preview.ReadyToCommit {
			t.Fatalf("expected valid preview, errors=%v", preview.Errors)
		}
	})
}

func TestParseBuyerImportPreviewMissingSheet(t *testing.T) {
	data := mutateWorkbook(t, richSnapshot(), func(f *excelize.File) {
		_ = f.DeleteSheet(sheetRules)
	})
	preview := parseWorkbook(t, data, richTarget())
	assertIssueCode(t, preview.Errors, MachineCodeMissingSheet)
}

func TestParseBuyerImportPreviewUnexpectedSheet(t *testing.T) {
	data := mutateWorkbook(t, richSnapshot(), func(f *excelize.File) {
		_, _ = f.NewSheet("Extra")
	})
	preview := parseWorkbook(t, data, richTarget())
	assertIssueCode(t, preview.Errors, MachineCodeUnexpectedSheet)
}

func TestParseBuyerImportPreviewWrongSheetOrder(t *testing.T) {
	data := mutateWorkbook(t, richSnapshot(), func(f *excelize.File) {
		_ = f.SetSheetName(sheetMetadata, "TmpMeta")
		_ = f.SetSheetName(sheetLots, sheetMetadata)
		_ = f.SetSheetName("TmpMeta", sheetLots)
	})
	preview := parseWorkbook(t, data, richTarget())
	assertIssueCode(t, preview.Errors, MachineCodeInvalidHeader)
}

func TestParseBuyerImportPreviewHiddenSheetDenied(t *testing.T) {
	data := mutateWorkbook(t, richSnapshot(), func(f *excelize.File) {
		_ = f.SetSheetVisible(sheetQuestions, false)
	})
	preview := parseWorkbook(t, data, richTarget())
	assertIssueCode(t, preview.Errors, MachineCodeHiddenSheetDenied)
}

func TestParseBuyerImportPreviewHiddenRowWarning(t *testing.T) {
	data := mutateWorkbook(t, richSnapshot(), func(f *excelize.File) {
		_ = f.SetRowVisible(sheetSections, 2, false)
	})
	preview := parseWorkbook(t, data, richTarget())
	assertIssueCode(t, preview.Warnings, MachineCodeHiddenContentWarning)
}

func TestParseBuyerImportPreviewMergedCellsDenied(t *testing.T) {
	data := mutateWorkbook(t, richSnapshot(), func(f *excelize.File) {
		_ = f.MergeCell(sheetSections, "A2", "B2")
	})
	preview := parseWorkbook(t, data, richTarget())
	assertIssueCode(t, preview.Errors, MachineCodeMergedCellDenied)
}

func TestParseBuyerImportPreviewDuplicateHeaderDenied(t *testing.T) {
	data := mutateWorkbook(t, richSnapshot(), func(f *excelize.File) {
		_ = f.SetCellStr(sheetSections, "B1", "section_code")
	})
	preview := parseWorkbook(t, data, richTarget())
	assertIssueCode(t, preview.Errors, MachineCodeInvalidHeader)
}

func TestParseBuyerImportPreviewUnsupportedSchema(t *testing.T) {
	snapshot := minimalSnapshot()
	data := mutateWorkbook(t, snapshot, func(f *excelize.File) {
		_ = f.SetCellStr(sheetMetadata, "B1", "WRONG_SCHEMA")
	})
	preview := parseWorkbookBytes(t, data, targetFromSnapshot(snapshot))
	assertIssueCode(t, preview.Errors, MachineCodeUnsupportedSchema)
}

func TestParseBuyerImportPreviewMetadataMismatchWarning(t *testing.T) {
	snapshot := richSnapshot()
	snapshot.Metadata.RfxEventID = uuid.New()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	target := targetFromSnapshot(richSnapshot())
	preview, err := ParseBuyerImportPreview(context.Background(), data, target, ParserOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	assertIssueCode(t, preview.Warnings, MachineCodeMetadataMismatch)
}

func TestParseBuyerImportPreviewWorkbookTenantCannotOverrideBaseline(t *testing.T) {
	snapshot := richSnapshot()
	snapshot.Metadata.TenantID = uuid.New()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	target := targetFromSnapshot(richSnapshot())
	preview, err := ParseBuyerImportPreview(context.Background(), data, target, ParserOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	assertIssueCode(t, preview.Warnings, MachineCodeMetadataMismatch)
	if preview.TargetEventID != target.EventID {
		t.Fatalf("target event must come from baseline")
	}
}

func TestParseBuyerImportPreviewDuplicateCodes(t *testing.T) {
	data := mutateWorkbook(t, richSnapshot(), func(f *excelize.File) {
		_ = f.SetCellStr(sheetSections, "A3", "SEC1")
		_ = f.SetCellStr(sheetSections, "B3", "Dup")
		_ = f.SetCellStr(sheetSections, "H3", "2")
	})
	preview := parseWorkbook(t, data, richTarget())
	assertIssueCode(t, preview.Errors, MachineCodeDuplicateStableCode)
}

func TestParseBuyerImportPreviewDanglingReferences(t *testing.T) {
	data := mutateWorkbook(t, richSnapshot(), func(f *excelize.File) {
		_ = f.SetCellStr(sheetQuestions, "A2", "MISSING")
	})
	preview := parseWorkbook(t, data, richTarget())
	assertIssueCode(t, preview.Errors, MachineCodeDanglingReference)
}

func TestParseBuyerImportPreviewFormulaCellDenied(t *testing.T) {
	data, err := injectWorksheetFormula(t, richSnapshot())
	if err != nil {
		t.Fatalf("inject formula: %v", err)
	}
	preview := parseWorkbookBytes(t, data, richTarget())
	if preview.ReadyToCommit {
		t.Fatal("formula workbook must not be ready")
	}
}

func TestParseBuyerImportPreviewFormulaLikeTextSafe(t *testing.T) {
	snapshot := validImportSnapshot()
	value := "=SAFE-TEXT"
	snapshot.Lots[0].Description = &value
	snapshot.Lots[0].LotNumber = "+001"
	snapshot.Lots[0].Name = "@alias"
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview := parseWorkbook(t, data, targetFromSnapshot(snapshot))
	if !preview.ReadyToCommit {
		t.Fatalf("formula-like text must remain safe: %v", preview.Errors)
	}
	if preview.Proposal.Lots[0].LotNumber != "+001" {
		t.Fatalf("lot number changed: %q", preview.Proposal.Lots[0].LotNumber)
	}
}

func TestParseBuyerImportPreviewSelfTargetRuleDenied(t *testing.T) {
	preview := parseWorkbook(t, GenerateWorkbookOrFail(t, richSnapshot()), richTarget())
	assertIssueCode(t, preview.Errors, MachineCodeSelfTargetRule)
}

func TestParseBuyerImportPreviewCyclicRuleDenied(t *testing.T) {
	snapshot := richSnapshot()
	snapshot.Rules = []BuyerDraftRule{
		{RuleCode: "R1", SourceQuestionCode: "Q1", TargetQuestionCode: "Q1", ConditionJSON: json.RawMessage(`{"operator":"EQUALS","source_question_code":"Q1","value":"a"}`), Action: domain.RuleActionShow, SortOrder: 1},
	}
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview := parseWorkbook(t, data, targetFromSnapshot(snapshot))
	assertIssueCode(t, preview.Errors, MachineCodeSelfTargetRule)
}

func TestParseBuyerImportPreviewI18NIdenticalNormalized(t *testing.T) {
	snapshot := validImportSnapshot()
	preview := parseWorkbook(t, GenerateWorkbookOrFail(t, snapshot), targetFromSnapshot(snapshot))
	if len(preview.Warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", preview.Warnings)
	}
}

func TestParseBuyerImportPreviewI18NMismatchWarning(t *testing.T) {
	snapshot := richSnapshot()
	snapshot.Sections[0].Title = "RU"
	data := mutateWorkbook(t, snapshot, func(f *excelize.File) {
		_ = f.SetCellStr(sheetSections, "C2", "EN")
	})
	preview := parseWorkbook(t, data, targetFromSnapshot(snapshot))
	assertIssueCode(t, preview.Warnings, MachineCodeI18NMonolingualMismatch)
	if preview.Proposal.Questionnaire.Sections[0].Section.Title != "RU" {
		t.Fatalf("expected ru authoritative value")
	}
}

func TestParseBuyerImportPreviewQuestionnaireDiffReuse(t *testing.T) {
	snapshot := validImportSnapshot()
	target := targetFromSnapshot(snapshot)
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview, err := ParseBuyerImportPreview(context.Background(), data, target, ParserOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if preview.QuestionnaireDiff.Summary.UnchangedCount == 0 {
		t.Fatalf("expected unchanged questionnaire diff: %+v", preview.QuestionnaireDiff.Summary)
	}
}

func TestParseBuyerImportPreviewLotsDiff(t *testing.T) {
	snapshot := richSnapshot()
	target := targetFromSnapshot(snapshot)
	snapshot.Lots[0].Name = "Changed"
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview, err := ParseBuyerImportPreview(context.Background(), data, target, ParserOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if preview.LotsDiff.Summary.ChangedCount == 0 {
		t.Fatalf("expected lot change diff: %+v", preview.LotsDiff)
	}
}

func TestParseBuyerImportPreviewDeterministicHash(t *testing.T) {
	snapshot := validImportSnapshot()
	target := targetFromSnapshot(snapshot)
	first := GenerateWorkbookOrFail(t, snapshot)
	second := GenerateWorkbookOrFail(t, snapshot)
	p1, err := ParseBuyerImportPreview(context.Background(), first, target, ParserOptions{})
	if err != nil {
		t.Fatalf("parse1: %v", err)
	}
	p2, err := ParseBuyerImportPreview(context.Background(), second, target, ParserOptions{})
	if err != nil {
		t.Fatalf("parse2: %v", err)
	}
	if p1.CanonicalPayloadHash != p2.CanonicalPayloadHash {
		t.Fatalf("hash mismatch")
	}
}

func TestParseBuyerImportPreviewRowOrderIndependent(t *testing.T) {
	snapshot := validImportSnapshot()
	snapshot.Lots = []domain.RfxLot{
		{LotNumber: "L2", Name: "Two", Status: "DRAFT"},
		{LotNumber: "L1", Name: "One", Status: "DRAFT"},
	}
	target := targetFromSnapshot(snapshot)
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	preview, err := ParseBuyerImportPreview(context.Background(), data, target, ParserOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !preview.ReadyToCommit {
		t.Fatalf("errors: %v", preview.Errors)
	}
	got := []string{preview.Proposal.Lots[0].LotNumber, preview.Proposal.Lots[1].LotNumber}
	if !reflect.DeepEqual(got, []string{"L1", "L2"}) {
		t.Fatalf("normalized lot order: %v", got)
	}
}

func TestParseBuyerImportPreviewZIPMetadataIndependent(t *testing.T) {
	snapshot := validImportSnapshot()
	target := targetFromSnapshot(snapshot)
	data := GenerateWorkbookOrFail(t, snapshot)
	rewritten := rewriteZipComment(t, data, "comment-a")
	rewritten2 := rewriteZipComment(t, data, "comment-b")
	p1, _ := ParseBuyerImportPreview(context.Background(), rewritten, target, ParserOptions{})
	p2, _ := ParseBuyerImportPreview(context.Background(), rewritten2, target, ParserOptions{})
	if p1.CanonicalPayloadHash != p2.CanonicalPayloadHash {
		t.Fatalf("zip metadata must not affect hash")
	}
}

func TestParseBuyerImportPreviewTargetBaselineAffectsHash(t *testing.T) {
	snapshot := validImportSnapshot()
	targetA := targetFromSnapshot(snapshot)
	targetB := targetA
	targetB.EventRowVersion = targetA.EventRowVersion + 1
	data := GenerateWorkbookOrFail(t, snapshot)
	p1, _ := ParseBuyerImportPreview(context.Background(), data, targetA, ParserOptions{})
	p2, _ := ParseBuyerImportPreview(context.Background(), data, targetB, ParserOptions{})
	if p1.CanonicalPayloadHash == p2.CanonicalPayloadHash {
		t.Fatal("baseline row version must affect hash")
	}
}

func TestParseBuyerImportPreviewIssueOrderingDeterministic(t *testing.T) {
	data := mutateWorkbook(t, richSnapshot(), func(f *excelize.File) {
		_ = f.SetCellStr(sheetSections, "A3", "SEC1")
		_ = f.SetCellStr(sheetQuestions, "A2", "MISSING")
	})
	p1 := parseWorkbook(t, data, richTarget())
	p2 := parseWorkbook(t, data, richTarget())
	if !reflect.DeepEqual(p1.Errors, p2.Errors) {
		t.Fatalf("errors not deterministic")
	}
}

func TestParseBuyerImportPreviewNoSideEffects(t *testing.T) {
	snapshot := validImportSnapshot()
	data := GenerateWorkbookOrFail(t, snapshot)
	original := append([]byte(nil), data...)
	target := targetFromSnapshot(snapshot)
	preview, err := ParseBuyerImportPreview(context.Background(), data, target, ParserOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !bytes.Equal(data, original) {
		t.Fatal("input bytes mutated")
	}
	if !preview.ReadyToCommit {
		t.Fatalf("unexpected errors: %v", preview.Errors)
	}
}

func TestBuyerImportParserForbiddenImports(t *testing.T) {
	forbidden := []string{
		"database/sql",
		"github.com/jackc/pgx",
		"internal/repository",
		"internal/http",
		"os.WriteFile",
	}
	matches, err := filepath.Glob("buyer_import*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, file := range matches {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		src := string(body)
		for _, needle := range forbidden {
			if strings.Contains(src, needle) {
				t.Fatalf("%s contains forbidden import pattern %q", file, needle)
			}
		}
	}
}

func parseWorkbook(t *testing.T, data []byte, target TargetDraftBaseline) BuyerImportPreview {
	t.Helper()
	preview, err := ParseBuyerImportPreview(context.Background(), data, target, ParserOptions{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return preview
}

func parseWorkbookBytes(t *testing.T, data []byte, target TargetDraftBaseline) BuyerImportPreview {
	return parseWorkbook(t, data, target)
}

func assertIssueCode(t *testing.T, issues []BuyerImportIssue, code string) {
	t.Helper()
	for _, issue := range issues {
		if issue.MachineCode == code {
			return
		}
	}
	t.Fatalf("missing issue code %s in %#v", code, issues)
}

func minimalTarget() TargetDraftBaseline {
	snapshot := minimalSnapshot()
	return targetFromSnapshot(snapshot)
}

func richTarget() TargetDraftBaseline {
	return targetFromSnapshot(richSnapshot())
}

func targetFromSnapshot(snapshot BuyerDraftSnapshot) TargetDraftBaseline {
	sections := make([]domain.SectionWithQuestions, 0, len(snapshot.Sections))
	questionByCode := map[string]domain.Question{}
	for _, item := range snapshot.Questions {
		questionByCode[item.Question.QuestionCode] = item.Question
	}
	for _, section := range snapshot.Sections {
		var questions []domain.Question
		for _, item := range snapshot.Questions {
			if item.SectionCode == section.SectionCode {
				q := item.Question
				for _, opt := range snapshot.Options {
					if opt.QuestionCode == q.QuestionCode {
						q.Options = append(q.Options, opt.Option)
					}
				}
				questions = append(questions, q)
			}
		}
		sections = append(sections, domain.SectionWithQuestions{Section: section, Questions: questions})
	}
	rules := make([]domain.QuestionRule, 0, len(snapshot.Rules))
	for _, rule := range snapshot.Rules {
		targetID := questionByCode[rule.TargetQuestionCode].ID
		rules = append(rules, domain.QuestionRule{
			ID:               uuid.New(),
			RuleCode:         rule.RuleCode,
			Action:           rule.Action,
			ConditionJSON:    rule.ConditionJSON,
			TargetQuestionID: &targetID,
			SortOrder:        rule.SortOrder,
		})
	}
	return TargetDraftBaseline{
		TenantID:           snapshot.Metadata.TenantID,
		EventID:            snapshot.Metadata.RfxEventID,
		DraftVersionID:     snapshot.Metadata.RfxVersionID,
		DraftVersionNumber: snapshot.Metadata.VersionNumber,
		EventRowVersion:    snapshot.Metadata.EventRowVersion,
		DraftRowVersion:    snapshot.Metadata.VersionRowVersion,
		Questionnaire: domain.QuestionnaireDefinition{
			EventID:              snapshot.Metadata.RfxEventID,
			RfxVersionID:         snapshot.Metadata.RfxVersionID,
			VersionNumber:        snapshot.Metadata.VersionNumber,
			QuestionnaireEnabled: true,
			VersionStatus:        domain.RfxVersionStatusDraft,
			Sections:             sections,
			Rules:                rules,
		},
		Lots: append([]domain.RfxLot(nil), snapshot.Lots...),
	}
}

func mutateWorkbook(t *testing.T, snapshot BuyerDraftSnapshot, fn func(*excelize.File)) []byte {
	t.Helper()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	fn(f)
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = f.Close()
	return buf.Bytes()
}

func GenerateWorkbookOrFail(t *testing.T, snapshot BuyerDraftSnapshot) []byte {
	t.Helper()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return data
}

func buildForbiddenPackage(t *testing.T, entry string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(entry)
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err := w.Write([]byte("macro")); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	out := buf.Bytes()
	copy(out, []byte{0x50, 0x4B, 0x03, 0x04})
	return out
}

func bytesRepeat(b byte, count int) []byte {
	out := make([]byte, count)
	for i := range out {
		out[i] = b
	}
	copy(out, []byte{0x50, 0x4B, 0x03, 0x04})
	return out
}

func injectWorksheetFormula(t *testing.T, snapshot BuyerDraftSnapshot) ([]byte, error) {
	t.Helper()
	data, err := GenerateBuyerDraftWorkbook(snapshot)
	if err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		if strings.Contains(file.Name, "xl/worksheets/") && strings.HasSuffix(file.Name, ".xml") {
			body = []byte(strings.Replace(string(body), "<c ", "<c><f>1+1</f></c><c ", 1))
		}
		w, err := zw.Create(file.Name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(body); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func replaceInZip(t *testing.T, data []byte, entry string, old, new []byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
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
		if file.Name == entry {
			body = bytes.Replace(body, old, new, 1)
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

func rewriteZipComment(t *testing.T, data []byte, comment string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.SetComment(comment)
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
