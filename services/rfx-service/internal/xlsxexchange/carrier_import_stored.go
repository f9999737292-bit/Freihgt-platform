package xlsxexchange

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

// StoredCarrierPayload is the server-authoritative decoded carrier analysis payload for commit.
type StoredCarrierPayload struct {
	TargetEventID            uuid.UUID
	TargetResponseID         uuid.UUID
	TargetRfxVersionID       uuid.UUID
	TargetVersionNumber      int
	EventRowVersion          int
	ResponseSaveVersion      int64
	CarrierCompanyID         uuid.UUID
	AvailableLotsFingerprint string
	AnswersFingerprint       string
	OfferLinesFingerprint    string
	Answers                  []CarrierAnswerRow
	OfferLines               []CarrierOfferLineRow
}

// ParseStoredCarrierPayload decodes immutable carrier analysis JSON into commit-safe server proposal fields.
func ParseStoredCarrierPayload(payloadJSON []byte) (StoredCarrierPayload, error) {
	stored, _, err := StableStoredCarrierPayload(payloadJSON)
	if err != nil {
		return StoredCarrierPayload{}, err
	}
	var payload canonicalCarrierImportPayload
	if err := json.Unmarshal(stored, &payload); err != nil {
		return StoredCarrierPayload{}, err
	}
	answers := make([]CarrierAnswerRow, 0, len(payload.Answers))
	for _, answer := range payload.Answers {
		answers = append(answers, CarrierAnswerRow{
			QuestionCode: answer.QuestionCode,
			AnswerValue:  answer.AnswerValue,
		})
	}
	offerLines := make([]CarrierOfferLineRow, 0, len(payload.OfferLines))
	for _, line := range payload.OfferLines {
		offerLines = append(offerLines, CarrierOfferLineRow{
			LotNumber:    line.LotNumber,
			Amount:       line.Amount,
			CurrencyCode: line.CurrencyCode,
			Comment:      line.Comment,
		})
	}
	return StoredCarrierPayload{
		TargetEventID:            payload.TargetEventID,
		TargetResponseID:         payload.TargetResponseID,
		TargetRfxVersionID:       payload.TargetRfxVersionID,
		TargetVersionNumber:      payload.TargetVersionNumber,
		EventRowVersion:          payload.EventRowVersion,
		ResponseSaveVersion:      payload.ResponseSaveVersion,
		CarrierCompanyID:         payload.CarrierCompanyID,
		AvailableLotsFingerprint: payload.AvailableLotsFingerprint,
		AnswersFingerprint:       payload.AnswersFingerprint,
		OfferLinesFingerprint:    payload.OfferLinesFingerprint,
		Answers:                  answers,
		OfferLines:               offerLines,
	}, nil
}

// ProposalFromStoredCarrierPayload materializes a carrier mutation proposal from stored canonical payload.
func ProposalFromStoredCarrierPayload(stored StoredCarrierPayload, target TargetCarrierBaseline) CarrierImportProposal {
	rt := domain.BuildQuestionnaireRuntime(&target.Questionnaire)
	baselineByCode := make(map[string]string, len(target.BaselineAnswers))
	for _, row := range target.BaselineAnswers {
		baselineByCode[row.QuestionCode] = row.AnswerValue
	}
	storedByCode := make(map[string]string, len(stored.Answers))
	for _, row := range stored.Answers {
		storedByCode[row.QuestionCode] = row.AnswerValue
	}

	answers := make([]CarrierAnswerPatch, 0, len(stored.Answers)+len(target.BaselineAnswers))
	for code, baselineValue := range baselineByCode {
		storedValue, ok := storedByCode[code]
		if !ok || strings.TrimSpace(storedValue) == "" {
			question, exists := rt.QuestionsByCode[code]
			if !exists {
				continue
			}
			answers = append(answers, CarrierAnswerPatch{
				QuestionCode: code,
				QuestionID:   question.ID,
				Delete:       true,
			})
			continue
		}
		if storedValue == baselineValue {
			continue
		}
		question, exists := rt.QuestionsByCode[code]
		if !exists {
			continue
		}
		answers = append(answers, CarrierAnswerPatch{
			QuestionCode: code,
			QuestionID:   question.ID,
			Value:        parseAnswerValueJSON(storedValue),
		})
	}
	for code, storedValue := range storedByCode {
		if strings.TrimSpace(storedValue) == "" {
			continue
		}
		if _, had := baselineByCode[code]; had {
			continue
		}
		question, exists := rt.QuestionsByCode[code]
		if !exists {
			continue
		}
		answers = append(answers, CarrierAnswerPatch{
			QuestionCode: code,
			QuestionID:   question.ID,
			Value:        parseAnswerValueJSON(storedValue),
		})
	}

	lotByNumber := make(map[string]uuid.UUID, len(target.Lots))
	for _, lot := range target.Lots {
		lotByNumber[strings.TrimSpace(lot.LotNumber)] = lot.ID
	}
	offerLines := make([]CarrierOfferLinePatch, 0, len(stored.OfferLines))
	for _, line := range stored.OfferLines {
		amount, ok := parseOfferAmount(line.Amount)
		if !ok {
			continue
		}
		lotID := uuid.Nil
		if target.LotCount > 0 {
			lotID = lotByNumber[strings.TrimSpace(line.LotNumber)]
		}
		offerLines = append(offerLines, CarrierOfferLinePatch{
			LotNumber:    line.LotNumber,
			RfxLotID:     lotID,
			Amount:       amount,
			CurrencyCode: line.CurrencyCode,
			Comment:      line.Comment,
		})
	}
	return CarrierImportProposal{
		Answers:    answers,
		OfferLines: offerLines,
	}
}

// ValidateCarrierCommitProposal re-runs domain validation for a stored carrier import proposal at commit time.
func ValidateCarrierCommitProposal(ctx context.Context, target TargetCarrierBaseline, proposal CarrierImportProposal) []BuyerImportIssue {
	p := newCarrierImportParser(ctx, internalParseConfig{
		contentType:  "",
		openWorkbook: defaultOpenWorkbook,
	})
	p.validateCarrierProposal(target, proposal)
	return p.issues.errors
}
