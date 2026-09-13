package xlsxexchange

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/freight-platform/rfx-service/internal/domain"
)

type baselineLotFingerprintEntry struct {
	LotNumber      string   `json:"lot_number"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Category       string   `json:"category,omitempty"`
	EstimatedValue *float64 `json:"estimated_value,omitempty"`
	CurrencyCode   string   `json:"currency_code,omitempty"`
	Status         string   `json:"status"`
}

// ComputeBaselineLotsFingerprint hashes server-authoritative lot state for stale detection.
func ComputeBaselineLotsFingerprint(lots []domain.RfxLot) (string, error) {
	entries := baselineLotFingerprintEntries(lots)
	raw, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// ComputeProposedLotsHash hashes the imported lot proposal distinct from baseline fingerprint.
func ComputeProposedLotsHash(lots []canonicalLot) (string, error) {
	sorted := append([]canonicalLot(nil), lots...)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.TrimSpace(sorted[i].LotNumber) < strings.TrimSpace(sorted[j].LotNumber)
	})
	raw, err := json.Marshal(sorted)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func baselineLotFingerprintEntries(lots []domain.RfxLot) []baselineLotFingerprintEntry {
	sorted := append([]domain.RfxLot(nil), lots...)
	sortLots(sorted)
	out := make([]baselineLotFingerprintEntry, 0, len(sorted))
	for _, lot := range sorted {
		status := strings.TrimSpace(lot.Status)
		if status == "" {
			status = "ACTIVE"
		}
		out = append(out, baselineLotFingerprintEntry{
			LotNumber:      strings.TrimSpace(lot.LotNumber),
			Name:           strings.TrimSpace(lot.Name),
			Description:    optionalString(lot.Description),
			Category:       optionalString(lot.Category),
			EstimatedValue: lot.EstimatedValue,
			CurrencyCode:   optionalString(lot.CurrencyCode),
			Status:         status,
		})
	}
	return out
}
