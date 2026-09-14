package xlsxexchange

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

type carrierLotFingerprintEntry struct {
	LotNumber    string `json:"lot_number"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Category     string `json:"category,omitempty"`
	CurrencyCode string `json:"currency_code,omitempty"`
	Status       string `json:"status"`
}

type carrierAnswerFingerprintEntry struct {
	QuestionCode string `json:"question_code"`
	AnswerValue  string `json:"answer_value"`
}

type carrierOfferLineFingerprintValue struct {
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currency_code"`
	Comment      string `json:"comment,omitempty"`
}

// ComputeCarrierAvailableLotsFingerprint hashes carrier-visible lot state (no estimated_value).
func ComputeCarrierAvailableLotsFingerprint(lots []domain.RfxLot) (string, error) {
	sorted := append([]domain.RfxLot(nil), lots...)
	sortLots(sorted)
	entries := make([]carrierLotFingerprintEntry, 0, len(sorted))
	for _, lot := range sorted {
		status := strings.TrimSpace(lot.Status)
		if status == "" {
			status = "ACTIVE"
		}
		entries = append(entries, carrierLotFingerprintEntry{
			LotNumber:    strings.TrimSpace(lot.LotNumber),
			Name:         strings.TrimSpace(lot.Name),
			Description:  optionalString(lot.Description),
			Category:     optionalString(lot.Category),
			CurrencyCode: optionalString(lot.CurrencyCode),
			Status:       status,
		})
	}
	return hashJSON(entries)
}

// ComputeCarrierAnswersFingerprint hashes visible answer map keyed by question_code.
func ComputeCarrierAnswersFingerprint(answers []CarrierAnswerRow) (string, error) {
	sorted := append([]CarrierAnswerRow(nil), answers...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].QuestionCode < sorted[j].QuestionCode
	})
	entries := make([]carrierAnswerFingerprintEntry, 0, len(sorted))
	for _, answer := range sorted {
		entries = append(entries, carrierAnswerFingerprintEntry{
			QuestionCode: strings.TrimSpace(answer.QuestionCode),
			AnswerValue:  answer.AnswerValue,
		})
	}
	return hashJSON(entries)
}

// ComputeCarrierOfferLinesFingerprint hashes offer lines keyed by lot_number or event-level sentinel.
func ComputeCarrierOfferLinesFingerprint(
	lines []domain.RfxResponseOfferLine,
	lotNumberByID map[uuid.UUID]string,
) (string, error) {
	type keyedEntry struct {
		key   string
		value carrierOfferLineFingerprintValue
	}
	entries := make([]keyedEntry, 0, len(lines))
	for _, line := range lines {
		key := carrierEventLevelOfferKey
		if line.RfxLotID != uuid.Nil {
			key = strings.TrimSpace(lotNumberByID[line.RfxLotID])
		}
		entries = append(entries, keyedEntry{
			key: key,
			value: carrierOfferLineFingerprintValue{
				Amount:       formatOfferAmount(line.Amount),
				CurrencyCode: strings.TrimSpace(line.CurrencyCode),
				Comment:      optionalString(line.Comment),
			},
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].key < entries[j].key
	})
	out := make(map[string]carrierOfferLineFingerprintValue, len(entries))
	for _, entry := range entries {
		out[entry.key] = entry.value
	}
	return hashJSON(out)
}

func hashJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func formatOfferAmount(amount float64) string {
	return strconv.FormatFloat(amount, 'f', -1, 64)
}

// FormatOfferAmountForExport renders offer line amount as workbook text.
func FormatOfferAmountForExport(amount float64) string {
	return formatOfferAmount(amount)
}
