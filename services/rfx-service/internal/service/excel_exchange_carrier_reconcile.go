package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

type carrierImportCommitChanges struct {
	Answers    entityChangeCounts `json:"answers"`
	OfferLines entityChangeCounts `json:"offer_lines"`
}

func reconcileCarrierImportAnswers(
	ctx context.Context,
	answerRepo *repository.AnswerRepository,
	tenantID, responseID, actorUserID uuid.UUID,
	questionnaire domain.QuestionnaireDefinition,
	currentAnswers []domain.CarrierAnswer,
	proposal xlsxexchange.CarrierImportProposal,
	counts *entityChangeCounts,
) error {
	rt := domain.BuildQuestionnaireRuntime(&questionnaire)
	existing := answersMap(currentAnswers)

	patches := make([]domain.AnswerPatchItem, 0, len(proposal.Answers))
	for _, patch := range proposal.Answers {
		if patch.Delete {
			continue
		}
		patches = append(patches, domain.AnswerPatchItem{
			QuestionID: patch.QuestionID,
			Value:      patch.Value,
		})
	}

	mergedPreview := domain.MergeAnswerMaps(existing, patches)
	hiddenSet := make(map[uuid.UUID]struct{}, len(domain.HiddenQuestionIDs(rt, mergedPreview)))
	for _, id := range domain.HiddenQuestionIDs(rt, mergedPreview) {
		hiddenSet[id] = struct{}{}
	}
	filteredPatches := make([]domain.AnswerPatchItem, 0, len(patches))
	for _, patch := range patches {
		if _, hidden := hiddenSet[patch.QuestionID]; hidden {
			continue
		}
		filteredPatches = append(filteredPatches, patch)
	}
	if validationErrs := domain.ValidateCarrierAnswerPatches(rt, existing, filteredPatches, false); len(validationErrs) > 0 {
		return validationFailed(validationErrs)
	}

	merged := domain.MergeAnswerMaps(existing, filteredPatches)
	hiddenIDs := domain.HiddenQuestionIDs(rt, merged)
	for _, id := range hiddenIDs {
		delete(merged, id)
	}

	desired := make(map[uuid.UUID]json.RawMessage, len(merged))
	for qid, value := range merged {
		desired[qid] = value
	}

	if err := answerRepo.UpsertBatch(ctx, tenantID, responseID, filteredPatches, actorUserID); err != nil {
		return err
	}
	if len(hiddenIDs) > 0 {
		if err := answerRepo.DeleteByQuestionIDs(ctx, responseID, tenantID, hiddenIDs); err != nil {
			return err
		}
	}

	deleteIDs := make([]uuid.UUID, 0)
	for qid := range existing {
		if _, hidden := hiddenSet[qid]; hidden {
			continue
		}
		if _, keep := desired[qid]; keep {
			continue
		}
		deleteIDs = append(deleteIDs, qid)
	}
	for _, patch := range proposal.Answers {
		if !patch.Delete {
			continue
		}
		if _, hidden := hiddenSet[patch.QuestionID]; hidden {
			continue
		}
		deleteIDs = append(deleteIDs, patch.QuestionID)
	}
	if len(deleteIDs) > 0 {
		if err := answerRepo.DeleteByQuestionIDs(ctx, responseID, tenantID, uniqueUUIDs(deleteIDs)); err != nil {
			return err
		}
	}

	if counts != nil {
		baselineRows := make([]xlsxexchange.CarrierAnswerRow, 0, len(currentAnswers))
		questionCodes := rt.QuestionsByID
		for _, answer := range currentAnswers {
			question, ok := questionCodes[answer.QuestionID]
			if !ok {
				continue
			}
			baselineRows = append(baselineRows, xlsxexchange.CarrierAnswerRow{
				QuestionCode: question.QuestionCode,
				AnswerValue:  xlsxexchange.FormatCarrierAnswerValue(answer.AnswerValueJSON),
			})
		}
		proposedRows := proposedAnswerRowsFromDesired(rt, desired)
		diff := xlsxexchange.CompareCarrierAnswersDiffForCommit(baselineRows, proposedRows)
		counts.Added = len(diff.Added)
		counts.Updated = len(diff.Changed)
		counts.Deleted = len(diff.Removed)
	}
	return nil
}

func reconcileCarrierImportOfferLines(
	ctx context.Context,
	rfxRepo *repository.RfxRepository,
	tenantID, responseID uuid.UUID,
	target xlsxexchange.TargetCarrierBaseline,
	currentOfferLines []domain.RfxResponseOfferLine,
	proposal xlsxexchange.CarrierImportProposal,
	counts *entityChangeCounts,
) error {
	lines := make([]domain.UpsertOfferLineInput, 0, len(proposal.OfferLines))
	for _, line := range proposal.OfferLines {
		if line.Delete {
			continue
		}
		lines = append(lines, domain.UpsertOfferLineInput{
			RfxLotID:     line.RfxLotID,
			Amount:       line.Amount,
			CurrencyCode: line.CurrencyCode,
			Comment:      optionalStringField(line.Comment),
		})
	}
	if err := domain.ValidateOfferLinesForEventCurrency(lines, target.EventCurrency); err != nil {
		return apperrors.Unprocessable("stored import offer lines failed revalidation", map[string]any{
			"machine_code": domain.MachineCodeProposalRevalidation,
		})
	}
	if _, err := rfxRepo.ReplaceOfferLines(ctx, responseID, tenantID, lines); err != nil {
		return err
	}
	if counts != nil {
		lotNumberByID := make(map[uuid.UUID]string, len(target.Lots))
		for _, lot := range target.Lots {
			lotNumberByID[lot.ID] = lot.LotNumber
		}
		baselineRows := make([]xlsxexchange.CarrierOfferLineRow, 0, len(currentOfferLines))
		for _, line := range currentOfferLines {
			lotNumber := ""
			if line.RfxLotID != uuid.Nil {
				lotNumber = lotNumberByID[line.RfxLotID]
			}
			baselineRows = append(baselineRows, xlsxexchange.CarrierOfferLineRow{
				LotNumber:    lotNumber,
				Amount:       xlsxexchange.FormatOfferAmountForExport(line.Amount),
				CurrencyCode: line.CurrencyCode,
				Comment:      optionalStringPtr(line.Comment),
			})
		}
		proposedRows := xlsxexchange.ProposedOfferRowsFromProposal(proposal)
		diff := xlsxexchange.CompareCarrierOfferLinesDiffForCommit(baselineRows, proposedRows)
		counts.Added = len(diff.Added)
		counts.Updated = len(diff.Changed)
		counts.Deleted = len(diff.Removed)
	}
	return nil
}

func proposedAnswerRowsFromDesired(rt domain.QuestionnaireRuntime, desired map[uuid.UUID]json.RawMessage) []xlsxexchange.CarrierAnswerRow {
	out := make([]xlsxexchange.CarrierAnswerRow, 0, len(desired))
	for qid, value := range desired {
		question, ok := rt.QuestionsByID[qid]
		if !ok {
			continue
		}
		out = append(out, xlsxexchange.CarrierAnswerRow{
			QuestionCode: question.QuestionCode,
			AnswerValue:  xlsxexchange.FormatCarrierAnswerValue(value),
		})
	}
	return out
}

func uniqueUUIDs(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
