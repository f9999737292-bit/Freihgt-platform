package xlsxexchange

import (
	"sort"
	"strings"
)

type issueCollector struct {
	errors   []BuyerImportIssue
	warnings []BuyerImportIssue
}

func newIssueCollector() *issueCollector {
	return &issueCollector{}
}

func (c *issueCollector) addError(issue BuyerImportIssue) {
	c.errors = append(c.errors, issue)
}

func (c *issueCollector) addWarning(issue BuyerImportIssue) {
	c.warnings = append(c.warnings, issue)
}

func (c *issueCollector) sorted() ([]BuyerImportIssue, []BuyerImportIssue) {
	errors := append([]BuyerImportIssue(nil), c.errors...)
	warnings := append([]BuyerImportIssue(nil), c.warnings...)
	sortIssues(errors)
	sortIssues(warnings)
	return errors, warnings
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
