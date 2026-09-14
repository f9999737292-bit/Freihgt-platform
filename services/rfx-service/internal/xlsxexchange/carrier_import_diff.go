package xlsxexchange

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func compareCarrierAnswersDiff(baseline, proposed []CarrierAnswerRow) CarrierAnswersDiff {
	baseMap := make(map[string]string, len(baseline))
	for _, row := range baseline {
		baseMap[row.QuestionCode] = row.AnswerValue
	}
	propMap := make(map[string]string, len(proposed))
	for _, row := range proposed {
		propMap[row.QuestionCode] = row.AnswerValue
	}
	codes := sortedStringKeys(baseMap, propMap)
	diff := CarrierAnswersDiff{}
	for _, code := range codes {
		left, leftOK := baseMap[code]
		right, rightOK := propMap[code]
		switch {
		case !leftOK && rightOK:
			diff.Added = append(diff.Added, code)
		case leftOK && !rightOK:
			diff.Removed = append(diff.Removed, code)
		case leftOK && rightOK && left != right:
			diff.Changed = append(diff.Changed, code)
		}
	}
	diff.DiffHash = hashCarrierDiff(map[string]any{
		"added": diff.Added, "changed": diff.Changed, "removed": diff.Removed,
	})
	return diff
}

func compareCarrierOfferLinesDiff(baseline, proposed []CarrierOfferLineRow) CarrierOfferLinesDiff {
	baseMap := offerLinesByKey(baseline)
	propMap := offerLinesByKey(proposed)
	keys := sortedOfferLineKeys(baseMap, propMap)
	diff := CarrierOfferLinesDiff{}
	for _, key := range keys {
		left, leftOK := baseMap[key]
		right, rightOK := propMap[key]
		switch {
		case !leftOK && rightOK:
			diff.Added = append(diff.Added, key)
		case leftOK && !rightOK:
			diff.Removed = append(diff.Removed, key)
		case leftOK && rightOK && !offerLineRowEqual(left, right):
			diff.Changed = append(diff.Changed, key)
		}
	}
	diff.DiffHash = hashCarrierDiff(map[string]any{
		"added": diff.Added, "changed": diff.Changed, "removed": diff.Removed,
	})
	return diff
}

func offerLinesByKey(lines []CarrierOfferLineRow) map[string]CarrierOfferLineRow {
	out := make(map[string]CarrierOfferLineRow, len(lines))
	for _, line := range lines {
		key := line.LotNumber
		if key == "" {
			key = carrierEventLevelOfferKey
		}
		out[key] = line
	}
	return out
}

func offerLineRowEqual(left, right CarrierOfferLineRow) bool {
	return left.LotNumber == right.LotNumber &&
		left.Amount == right.Amount &&
		left.CurrencyCode == right.CurrencyCode &&
		left.Comment == right.Comment
}

func buildCarrierImportSummary(errors, warnings []BuyerImportIssue, aDiff CarrierAnswersDiff, oDiff CarrierOfferLinesDiff) CarrierImportSummary {
	return CarrierImportSummary{
		Errors:            len(errors),
		Warnings:          len(warnings),
		AnswersAdded:      len(aDiff.Added),
		AnswersChanged:    len(aDiff.Changed),
		AnswersRemoved:    len(aDiff.Removed),
		OfferLinesAdded:   len(oDiff.Added),
		OfferLinesChanged: len(oDiff.Changed),
		OfferLinesRemoved: len(oDiff.Removed),
	}
}

func sortedOfferLineKeys(maps ...map[string]CarrierOfferLineRow) []string {
	keys := make(map[string]struct{})
	for _, m := range maps {
		for k := range m {
			keys[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedStringKeys(maps ...map[string]string) []string {
	keys := make(map[string]struct{})
	for _, m := range maps {
		for k := range m {
			keys[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func hashCarrierDiff(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
