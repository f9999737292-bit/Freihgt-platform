package xlsxexchange

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func (p *carrierImportParser) validateCarrierProposal(target TargetCarrierBaseline, proposal CarrierImportProposal) {
	if err := p.checkContext(); err != nil {
		return
	}
	rt := domain.BuildQuestionnaireRuntime(&target.Questionnaire)
	questionByCode := rt.QuestionsByCode
	lotByNumber := make(map[string]uuid.UUID, len(target.Lots))
	for _, lot := range target.Lots {
		lotByNumber[strings.TrimSpace(lot.LotNumber)] = lot.ID
	}

	existing := make(map[uuid.UUID]json.RawMessage)
	for _, row := range target.BaselineAnswers {
		q, ok := questionByCode[row.QuestionCode]
		if !ok {
			continue
		}
		existing[q.ID] = parseAnswerValueJSON(row.AnswerValue)
	}

	patches := make([]domain.AnswerPatchItem, 0, len(proposal.Answers))
	for _, answer := range proposal.Answers {
		if answer.Delete {
			continue
		}
		patches = append(patches, domain.AnswerPatchItem{
			QuestionID: answer.QuestionID,
			Value:      answer.Value,
		})
	}
	for _, detail := range domain.ValidateCarrierAnswerPatches(rt, existing, patches, false) {
		stableCode := ""
		if q, ok := rt.QuestionsByID[detail.QuestionID]; ok {
			stableCode = q.QuestionCode
		}
		p.issues.addError(carrierIssueError(
			MachineCodeInvalidType,
			"rfx.carrier_xlsx_import.invalid_answer",
			sheetAnswers, "answer_value", stableCode, 0,
			map[string]any{"rule": detail.Rule, "message_key": detail.MessageKey},
		))
	}

	seenLots := make(map[string]struct{})
	eventLevelCount := 0
	for _, line := range proposal.OfferLines {
		if line.Delete {
			continue
		}
		lotKey := strings.TrimSpace(line.LotNumber)
		if target.LotCount == 0 {
			if lotKey != "" {
				p.issues.addError(carrierIssueError(
					MachineCodeUnknownLot,
					"rfx.carrier_xlsx_import.unknown_lot",
					sheetOfferLines, "lot_number", lotKey, 0, nil,
				))
			}
			eventLevelCount++
			if eventLevelCount > 1 {
				p.issues.addError(carrierIssueError(
					MachineCodeDuplicateEventLevelOffer,
					"rfx.carrier_xlsx_import.duplicate_event_level_offer",
					sheetOfferLines, "lot_number", "", 0, nil,
				))
			}
		} else {
			if lotKey == "" {
				p.issues.addError(carrierIssueError(
					MachineCodeRfxLotIDRequired,
					"rfx.carrier_xlsx_import.rfx_lot_id_required",
					sheetOfferLines, "lot_number", "", 0, nil,
				))
				continue
			}
			if _, ok := lotByNumber[lotKey]; !ok {
				p.issues.addError(carrierIssueError(
					MachineCodeUnknownLot,
					"rfx.carrier_xlsx_import.rfx_lot_not_found",
					sheetOfferLines, "lot_number", lotKey, 0, nil,
				))
			}
		}
		if lotKey != "" {
			if _, dup := seenLots[lotKey]; dup {
				p.issues.addError(carrierIssueError(
					MachineCodeDuplicateLotNumber,
					"rfx.carrier_xlsx_import.duplicate_lot_number",
					sheetOfferLines, "lot_number", lotKey, 0, nil,
				))
			}
			seenLots[lotKey] = struct{}{}
		}
		input := domain.UpsertOfferLineInput{
			RfxLotID:     line.RfxLotID,
			Amount:       line.Amount,
			CurrencyCode: line.CurrencyCode,
			Comment:      optionalStringPtr(line.Comment),
		}
		if err := domain.ValidateOfferLineInput(input, target.LotCount); err != nil {
			p.issues.addError(carrierIssueError(
				MachineCodeInvalidType,
				"rfx.carrier_xlsx_import.invalid_offer_line",
				sheetOfferLines, "amount", lotKey, 0,
				map[string]any{"field": "amount"},
			))
		}
	}
	if err := domain.ValidateOfferLinesForEventCurrency(offerLineInputs(proposal.OfferLines), target.EventCurrency); err != nil {
		p.issues.addError(carrierIssueError(
			MachineCodeInvalidType,
			"rfx.carrier_xlsx_import.currency_mismatch",
			sheetOfferLines, "currency_code", "", 0, nil,
		))
	}
}

func offerLineInputs(lines []CarrierOfferLinePatch) []domain.UpsertOfferLineInput {
	out := make([]domain.UpsertOfferLineInput, 0, len(lines))
	for _, line := range lines {
		if line.Delete {
			continue
		}
		out = append(out, domain.UpsertOfferLineInput{
			RfxLotID:     line.RfxLotID,
			Amount:       line.Amount,
			CurrencyCode: line.CurrencyCode,
			Comment:      optionalStringPtr(line.Comment),
		})
	}
	return out
}

func parseAnswerValueJSON(text string) json.RawMessage {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return json.RawMessage(`null`)
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return json.RawMessage(trimmed)
	}
	if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return json.RawMessage(trimmed)
	}
	if trimmed == "true" || trimmed == "false" {
		return json.RawMessage(trimmed)
	}
	encoded, _ := json.Marshal(trimmed)
	return json.RawMessage(encoded)
}

func parseOfferAmount(text string) (float64, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
