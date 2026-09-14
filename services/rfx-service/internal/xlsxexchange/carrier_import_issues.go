package xlsxexchange

import (
	"sort"
	"strings"
)

func carrierSheetOrderIndex(sheet string) int {
	sheet = strings.TrimSpace(sheet)
	if sheet == "" {
		return -1
	}
	for idx, name := range carrierSheetOrder {
		if name == sheet {
			return idx
		}
	}
	return len(carrierSheetOrder) + 1
}

func sortCarrierIssues(issues []BuyerImportIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		left, right := issues[i], issues[j]
		if left.Severity != right.Severity {
			return left.Severity == IssueSeverityError
		}
		leftSheetIdx := carrierSheetOrderIndex(left.Sheet)
		rightSheetIdx := carrierSheetOrderIndex(right.Sheet)
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

func carrierIssueError(machineCode, messageKey, sheet, column, stableCode string, row int, params map[string]any) BuyerImportIssue {
	return issueError(machineCode, messageKey, sheet, column, stableCode, row, params)
}

func carrierIssueWarning(machineCode, messageKey, sheet, column, stableCode string, row int, params map[string]any) BuyerImportIssue {
	return issueWarning(machineCode, messageKey, sheet, column, stableCode, row, params)
}

func isCarrierForbiddenColumn(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	if _, forbidden := carrierForbiddenColumnNames[lower]; forbidden {
		return true
	}
	for _, prefix := range carrierForbiddenColumnPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	if strings.HasSuffix(lower, "_company_id") && lower != "carrier_company_id" {
		return true
	}
	return false
}
