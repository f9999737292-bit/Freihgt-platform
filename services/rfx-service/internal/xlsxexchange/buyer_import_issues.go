package xlsxexchange

import (
	"sort"
	"strings"
)

const (
	// MaxPreviewIssues is the maximum number of issues returned across errors and warnings.
	// When exceeded, the final slot is a deterministic ISSUE_LIMIT_REACHED truncation issue.
	MaxPreviewIssues = 2000
)

type issueCollector struct {
	errors            []BuyerImportIssue
	warnings          []BuyerImportIssue
	collectingStopped bool
	truncated         bool
}

func newIssueCollector() *issueCollector {
	return &issueCollector{}
}

func (c *issueCollector) regularCount() int {
	return len(c.errors) + len(c.warnings)
}

func (c *issueCollector) stopped() bool {
	return c.collectingStopped
}

func (c *issueCollector) addError(issue BuyerImportIssue) bool {
	if c.collectingStopped {
		return false
	}
	if c.regularCount() >= MaxPreviewIssues-1 {
		c.markTruncated()
		return false
	}
	c.errors = append(c.errors, issue)
	return true
}

func (c *issueCollector) addWarning(issue BuyerImportIssue) bool {
	if c.collectingStopped {
		return false
	}
	if c.regularCount() >= MaxPreviewIssues-1 {
		c.markTruncated()
		return false
	}
	c.warnings = append(c.warnings, issue)
	return true
}

func (c *issueCollector) markTruncated() {
	if c.truncated {
		c.collectingStopped = true
		return
	}
	c.truncated = true
	c.collectingStopped = true
}

func truncationIssue() BuyerImportIssue {
	return BuyerImportIssue{
		Severity:    IssueSeverityError,
		MachineCode: MachineCodeIssueLimitReached,
		MessageKey:  "rfx.buyer_xlsx_import.issue_limit_reached",
		Params:      map[string]any{"limit": MaxPreviewIssues},
	}
}

func (c *issueCollector) sorted() ([]BuyerImportIssue, []BuyerImportIssue) {
	errors := append([]BuyerImportIssue(nil), c.errors...)
	warnings := append([]BuyerImportIssue(nil), c.warnings...)
	sortIssues(errors)
	sortIssues(warnings)
	if c.truncated {
		errors = append(errors, truncationIssue())
	}
	return errors, warnings
}

func countIssuesWithCode(issues []BuyerImportIssue, code string) int {
	n := 0
	for _, issue := range issues {
		if issue.MachineCode == code {
			n++
		}
	}
	return n
}

// Package-level issues (empty sheet) sort before sheet-scoped issues.
func sortIssues(issues []BuyerImportIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		left, right := issues[i], issues[j]
		if left.Severity != right.Severity {
			return left.Severity == IssueSeverityError
		}
		leftSheetIdx := sheetOrderIndex(left.Sheet)
		rightSheetIdx := sheetOrderIndex(right.Sheet)
		if leftSheetIdx != rightSheetIdx {
			return leftSheetIdx < rightSheetIdx
		}
		if left.Row != right.Row {
			return left.Row < right.Row
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		if left.MachineCode != right.MachineCode {
			return left.MachineCode < right.MachineCode
		}
		return left.StableCode < right.StableCode
	})
}

func sheetOrderIndex(sheet string) int {
	sheet = strings.TrimSpace(sheet)
	if sheet == "" {
		return -1
	}
	for idx, name := range buyerSheetOrder {
		if name == sheet {
			return idx
		}
	}
	return len(buyerSheetOrder) + 1
}

func issueError(machineCode, messageKey, sheet, column, stableCode string, row int, params map[string]any) BuyerImportIssue {
	return BuyerImportIssue{
		Severity:    IssueSeverityError,
		MachineCode: machineCode,
		Sheet:       sheet,
		Row:         row,
		Column:      column,
		StableCode:  stableCode,
		MessageKey:  messageKey,
		Params:      params,
	}
}

func issueWarning(machineCode, messageKey, sheet, column, stableCode string, row int, params map[string]any) BuyerImportIssue {
	return BuyerImportIssue{
		Severity:    IssueSeverityWarning,
		MachineCode: machineCode,
		Sheet:       sheet,
		Row:         row,
		Column:      column,
		StableCode:  stableCode,
		MessageKey:  messageKey,
		Params:      params,
	}
}
