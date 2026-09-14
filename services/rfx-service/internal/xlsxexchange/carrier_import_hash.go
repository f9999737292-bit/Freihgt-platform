package xlsxexchange

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

type canonicalCarrierImportPayload struct {
	SchemaName               string                       `json:"schema_name"`
	SchemaVersion            string                       `json:"schema_version"`
	Mode                     string                       `json:"mode"`
	TargetEventID            uuid.UUID                    `json:"target_event_id"`
	TargetResponseID         uuid.UUID                    `json:"target_response_id"`
	TargetRfxVersionID       uuid.UUID                    `json:"target_rfx_version_id"`
	TargetVersionNumber      int                          `json:"target_version_number"`
	EventRowVersion          int                          `json:"event_row_version"`
	ResponseSaveVersion      int64                        `json:"response_save_version"`
	CarrierCompanyID         uuid.UUID                    `json:"carrier_company_id"`
	AvailableLotsFingerprint string                       `json:"available_lots_fingerprint"`
	AnswersFingerprint       string                       `json:"answers_fingerprint"`
	OfferLinesFingerprint    string                       `json:"offer_lines_fingerprint"`
	Answers                  []canonicalCarrierAnswer     `json:"answers"`
	OfferLines               []canonicalCarrierOfferLine  `json:"offer_lines"`
	AnswersDiffHash          string                       `json:"answers_diff_hash"`
	OfferLinesDiffHash       string                       `json:"offer_lines_diff_hash"`
	Counts                   canonicalCarrierImportCounts `json:"counts"`
}

type canonicalCarrierAnswer struct {
	QuestionCode string `json:"question_code"`
	AnswerValue  string `json:"answer_value,omitempty"`
}

type canonicalCarrierOfferLine struct {
	LotNumber    string `json:"lot_number,omitempty"`
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currency_code"`
	Comment      string `json:"comment,omitempty"`
}

type canonicalCarrierImportCounts struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Answers  int `json:"answers"`
	Offers   int `json:"offer_lines"`
}

// CanonicalCarrierImportPayloadJSON returns deterministic JSON bytes for immutable preview persistence.
func CanonicalCarrierImportPayloadJSON(preview CarrierImportPreview, target TargetCarrierBaseline, proposal CarrierImportProposal) ([]byte, error) {
	payload, err := buildCanonicalCarrierImportPayload(preview, target, proposal)
	if err != nil {
		return nil, err
	}
	raw, err := marshalCanonical(payload)
	if err != nil {
		return nil, err
	}
	stored, _, err := StableStoredCarrierPayload(raw)
	return stored, err
}

// StableStoredCarrierPayload re-canonicalizes carrier import payload bytes for JSONB-stable hashing.
func StableStoredCarrierPayload(payloadJSON []byte) ([]byte, string, error) {
	pgLike, err := normalizeJSONDocument(payloadJSON)
	if err != nil {
		return nil, "", err
	}
	var payload canonicalCarrierImportPayload
	if err := json.Unmarshal(pgLike, &payload); err != nil {
		return nil, "", err
	}
	normalized, err := marshalCanonical(payload)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(normalized)
	return normalized, hex.EncodeToString(sum[:]), nil
}

// StableStoredCarrierPayloadHash returns the JSONB-stable SHA-256 hex digest for carrier payload bytes.
func StableStoredCarrierPayloadHash(payloadJSON []byte) (string, error) {
	_, hash, err := StableStoredCarrierPayload(payloadJSON)
	return hash, err
}

// VerifyStoredCarrierCanonicalPayloadHash checks hash against JSONB-round-trip-safe canonical bytes.
func VerifyStoredCarrierCanonicalPayloadHash(payloadJSON []byte, hash string) error {
	_, computed, err := StableStoredCarrierPayload(payloadJSON)
	if err != nil {
		return err
	}
	if strings.ToLower(strings.TrimSpace(hash)) != computed {
		return fmt.Errorf("canonical hash mismatch")
	}
	return nil
}

func buildCanonicalCarrierImportPayload(preview CarrierImportPreview, target TargetCarrierBaseline, proposal CarrierImportProposal) (canonicalCarrierImportPayload, error) {
	answers := canonicalAnswersFromProposal(proposal)
	offerLines := make([]canonicalCarrierOfferLine, 0, len(proposal.OfferLines))
	for _, line := range proposal.OfferLines {
		if line.Delete {
			continue
		}
		offerLines = append(offerLines, canonicalCarrierOfferLine{
			LotNumber:    line.LotNumber,
			Amount:       formatOfferAmount(line.Amount),
			CurrencyCode: strings.TrimSpace(line.CurrencyCode),
			Comment:      line.Comment,
		})
	}
	return canonicalCarrierImportPayload{
		SchemaName:               domain.SchemaVersionCarrierXLSXV1,
		SchemaVersion:            schemaVersionNumber,
		Mode:                     CarrierImportModeUpdateDraft,
		TargetEventID:            target.EventID,
		TargetResponseID:         target.ResponseID,
		TargetRfxVersionID:       target.RfxVersionID,
		TargetVersionNumber:      target.QuestionnaireVersionNumber,
		EventRowVersion:          target.EventRowVersion,
		ResponseSaveVersion:      target.ResponseSaveVersion,
		CarrierCompanyID:         target.CarrierCompanyID,
		AvailableLotsFingerprint: target.AvailableLotsFingerprint,
		AnswersFingerprint:       target.AnswersFingerprint,
		OfferLinesFingerprint:    target.OfferLinesFingerprint,
		Answers:                  canonicalAnswersFromProposal(proposal),
		OfferLines:               offerLines,
		AnswersDiffHash:          preview.AnswersDiff.DiffHash,
		OfferLinesDiffHash:       preview.OfferLinesDiff.DiffHash,
		Counts: canonicalCarrierImportCounts{
			Errors:   len(preview.Errors),
			Warnings: len(preview.Warnings),
			Answers:  len(answers),
			Offers:   len(offerLines),
		},
	}, nil
}

func canonicalAnswersFromProposal(proposal CarrierImportProposal) []canonicalCarrierAnswer {
	out := make([]canonicalCarrierAnswer, 0, len(proposal.Answers))
	for _, patch := range proposal.Answers {
		if patch.Delete {
			continue
		}
		out = append(out, canonicalCarrierAnswer{
			QuestionCode: patch.QuestionCode,
			AnswerValue:  strings.TrimSpace(string(patch.Value)),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].QuestionCode < out[j].QuestionCode })
	return out
}
