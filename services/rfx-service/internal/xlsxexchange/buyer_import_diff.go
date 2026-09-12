package xlsxexchange

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func compareQuestionnaireDiff(target TargetDraftBaseline, proposal BuyerImportProposal) domain.CompareVersionsResult {
	source := baselineCompareSnapshot(target)
	targetSnap := proposalCompareSnapshot(target, proposal)
	return domain.CompareVersionSnapshots(source, targetSnap)
}

func compareLotsDiff(baseline, proposal []domain.RfxLot) LotsCompareResult {
	baselineSorted := append([]domain.RfxLot(nil), baseline...)
	proposalSorted := append([]domain.RfxLot(nil), proposal...)
	sortLots(baselineSorted)
	sortLots(proposalSorted)

	baselineByNumber := make(map[string]domain.RfxLot, len(baselineSorted))
	for _, lot := range baselineSorted {
		baselineByNumber[lot.LotNumber] = lot
	}
	proposalByNumber := make(map[string]domain.RfxLot, len(proposalSorted))
	for _, lot := range proposalSorted {
		proposalByNumber[lot.LotNumber] = lot
	}

	keys := sortedLotKeys(baselineByNumber, proposalByNumber)
	differences := make([]LotCompareItemDiff, 0, len(keys))
	for _, lotNumber := range keys {
		left, leftOK := baselineByNumber[lotNumber]
		right, rightOK := proposalByNumber[lotNumber]
		switch {
		case leftOK && !rightOK:
			differences = append(differences, LotCompareItemDiff{
				LotNumber:  lotNumber,
				Change:     domain.VersionDiffRemoved,
				FieldDiffs: lotFieldDiffs(&left, nil),
			})
		case !leftOK && rightOK:
			differences = append(differences, LotCompareItemDiff{
				LotNumber:  lotNumber,
				Change:     domain.VersionDiffAdded,
				FieldDiffs: lotFieldDiffs(nil, &right),
			})
		default:
			item := diffLotItem(lotNumber, left, right)
			differences = append(differences, item)
		}
	}

	reordered := detectLotReorder(baselineSorted, proposalSorted, differences)
	differences = append(differences, reordered...)
	sortLotDiffItems(differences)

	summary := summarizeLotDiffItems(differences)
	hash := canonicalLotDiffHash(differences)
	return LotsCompareResult{
		Summary:           summary,
		Differences:       differences,
		CanonicalDiffHash: hash,
	}
}

func sortedLotKeys(left, right map[string]domain.RfxLot) []string {
	keys := make(map[string]struct{})
	for key := range left {
		keys[key] = struct{}{}
	}
	for key := range right {
		keys[key] = struct{}{}
	}
	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func lotFieldDiffs(before, after *domain.RfxLot) []domain.CompareFieldDiff {
	fields := []struct {
		name string
		fn   func(*domain.RfxLot) any
	}{
		{"name", func(l *domain.RfxLot) any { return l.Name }},
		{"description", func(l *domain.RfxLot) any { return optionalString(l.Description) }},
		{"category", func(l *domain.RfxLot) any { return optionalString(l.Category) }},
		{"estimated_value", func(l *domain.RfxLot) any { return optionalFloat(l.EstimatedValue) }},
		{"currency_code", func(l *domain.RfxLot) any { return optionalString(l.CurrencyCode) }},
		{"status", func(l *domain.RfxLot) any { return l.Status }},
	}
	out := make([]domain.CompareFieldDiff, 0, len(fields))
	for _, field := range fields {
		var beforeVal, afterVal any
		if before != nil {
			beforeVal = field.fn(before)
		}
		if after != nil {
			afterVal = field.fn(after)
		}
		out = append(out, domain.CompareFieldDiff{
			Field:  field.name,
			Before: beforeVal,
			After:  afterVal,
		})
	}
	return out
}

func diffLotItem(lotNumber string, before, after domain.RfxLot) LotCompareItemDiff {
	fieldDiffs := lotFieldDiffs(&before, &after)
	changed := false
	for _, diff := range fieldDiffs {
		if diff.Before != diff.After {
			changed = true
			break
		}
	}
	change := domain.VersionDiffUnchanged
	if changed {
		change = domain.VersionDiffChanged
	}
	return LotCompareItemDiff{
		LotNumber:  lotNumber,
		Change:     change,
		FieldDiffs: fieldDiffs,
	}
}

func detectLotReorder(baseline, proposal []domain.RfxLot, existing []LotCompareItemDiff) []LotCompareItemDiff {
	if len(baseline) != len(proposal) {
		return nil
	}
	changedOrAdded := make(map[string]struct{})
	for _, item := range existing {
		if item.Change == domain.VersionDiffAdded || item.Change == domain.VersionDiffRemoved || item.Change == domain.VersionDiffChanged {
			changedOrAdded[item.LotNumber] = struct{}{}
		}
	}
	baselineOrder := make([]string, 0, len(baseline))
	proposalOrder := make([]string, 0, len(proposal))
	for _, lot := range baseline {
		baselineOrder = append(baselineOrder, lot.LotNumber)
	}
	for _, lot := range proposal {
		proposalOrder = append(proposalOrder, lot.LotNumber)
	}
	if len(changedOrAdded) > 0 {
		return nil
	}
	sameSet := true
	for i := range baselineOrder {
		if baselineOrder[i] != proposalOrder[i] {
			sameSet = false
			break
		}
	}
	if sameSet {
		return nil
	}
	out := make([]LotCompareItemDiff, 0, len(proposalOrder))
	for idx, lotNumber := range proposalOrder {
		out = append(out, LotCompareItemDiff{
			LotNumber: lotNumber,
			Change:    domain.VersionDiffReordered,
			FieldDiffs: []domain.CompareFieldDiff{{
				Field:  "sort_index",
				Before: idx,
				After:  idx,
			}},
		})
	}
	return out
}

func sortLotDiffItems(items []LotCompareItemDiff) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].LotNumber < items[j].LotNumber
	})
}

func summarizeLotDiffItems(items []LotCompareItemDiff) domain.CompareSummary {
	var summary domain.CompareSummary
	for _, item := range items {
		switch item.Change {
		case domain.VersionDiffAdded:
			summary.AddedCount++
		case domain.VersionDiffRemoved:
			summary.RemovedCount++
		case domain.VersionDiffChanged:
			summary.ChangedCount++
		case domain.VersionDiffReordered:
			summary.ReorderedCount++
		case domain.VersionDiffUnchanged:
			summary.UnchangedCount++
		}
	}
	return summary
}

func canonicalLotDiffHash(items []LotCompareItemDiff) string {
	payload, _ := json.Marshal(items)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func buildImportSummary(errors, warnings []BuyerImportIssue, qDiff domain.CompareVersionsResult, lDiff LotsCompareResult) BuyerImportSummary {
	return BuyerImportSummary{
		Errors:           len(errors),
		Warnings:         len(warnings),
		SectionsAdded:    countEntityChange(qDiff.Sections, domain.VersionDiffAdded),
		SectionsChanged:  countEntityChange(qDiff.Sections, domain.VersionDiffChanged),
		SectionsRemoved:  countEntityChange(qDiff.Sections, domain.VersionDiffRemoved),
		QuestionsAdded:   countEntityChange(qDiff.Questions, domain.VersionDiffAdded),
		QuestionsRemoved: countEntityChange(qDiff.Questions, domain.VersionDiffRemoved),
		LotsAdded:        lDiff.Summary.AddedCount,
		LotsChanged:      lDiff.Summary.ChangedCount,
		LotsRemoved:      lDiff.Summary.RemovedCount,
	}
}

func countEntityChange(items []domain.CompareItemDiff, change string) int {
	count := 0
	for _, item := range items {
		if item.Change == change {
			count++
		}
	}
	return count
}
