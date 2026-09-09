package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type PublishTemplateVersionResult struct {
	Published  *domain.RfxTemplateVersion
	Superseded *domain.RfxTemplateVersion
}

func (r *TemplateLibraryRepository) PublishTemplateVersionTx(
	ctx context.Context,
	templateID, tenantID uuid.UUID,
	expectedTemplateVersion, expectedDraftVersion int,
	changeSummary string,
	publishedBy uuid.UUID,
) (*PublishTemplateVersionResult, error) {
	tmpl, err := r.LockTemplateByID(ctx, templateID, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.EnsureTemplateActive(tmpl.Status); err != nil {
		return nil, err
	}
	if tmpl.Version != expectedTemplateVersion {
		return nil, apperrors.Conflict("template was modified by another request", map[string]any{"field": "expected_template_version"})
	}

	draft, err := r.GetDraftVersion(ctx, templateID, tenantID)
	if err != nil {
		return nil, err
	}
	if draft == nil {
		return nil, apperrors.Conflict("draft template version not found", map[string]any{"field": "draft_version_id"})
	}
	draft, err = r.LockVersionByID(ctx, draft.ID, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.EnsureTemplateVersionPublishable(draft.Status); err != nil {
		return nil, err
	}
	if draft.Version != expectedDraftVersion {
		return nil, apperrors.Conflict("draft version was modified by another request", map[string]any{"field": "expected_draft_version"})
	}

	var superseded *domain.RfxTemplateVersion
	publishedVer, err := r.GetPublishedVersion(ctx, templateID, tenantID)
	if err != nil {
		return nil, err
	}
	if publishedVer != nil {
		superseded, err = r.LockVersionByID(ctx, publishedVer.ID, tenantID)
		if err != nil {
			return nil, err
		}
		row := r.db().QueryRow(ctx, `
			UPDATE rfx.rfx_template_versions
			SET status=$3, updated_at=now(), version=version+1
			WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL
			RETURNING `+rfxTemplateVersionSelectColumns,
			superseded.ID, tenantID, domain.RfxVersionStatusSuperseded)
		superseded, err = scanRfxTemplateVersion(row)
		if err != nil {
			return nil, mapDBError(err)
		}
	}

	row := r.db().QueryRow(ctx, `
		UPDATE rfx.rfx_template_versions
		SET status=$3, change_summary=$4, published_at=now(), published_by=$5, updated_at=now(), version=version+1
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL
		RETURNING `+rfxTemplateVersionSelectColumns,
		draft.ID, tenantID, domain.RfxVersionStatusPublished, strings.TrimSpace(changeSummary), publishedBy)
	published, err := scanRfxTemplateVersion(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	published.IsPublished = true

	if _, err := r.db().Exec(ctx, `
		UPDATE rfx.rfx_templates SET updated_at=now(), version=version+1
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, templateID, tenantID); err != nil {
		return nil, mapDBError(err)
	}

	return &PublishTemplateVersionResult{Published: published, Superseded: superseded}, nil
}

func (r *TemplateLibraryRepository) ForkDraftFromPublishedTx(
	ctx context.Context,
	templateID, tenantID, forkActorUserID uuid.UUID,
	qRepo *TemplateQuestionnaireRepository,
) (*domain.RfxTemplateVersion, error) {
	tmpl, err := r.LockTemplateByID(ctx, templateID, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.EnsureTemplateActive(tmpl.Status); err != nil {
		return nil, err
	}
	if draft, err := r.GetDraftVersion(ctx, templateID, tenantID); err != nil {
		return nil, err
	} else if draft != nil {
		return nil, apperrors.Conflict("draft template version already exists", map[string]any{"field": "draft_version_id"})
	}

	published, err := r.GetPublishedVersion(ctx, templateID, tenantID)
	if err != nil {
		return nil, err
	}
	if published == nil {
		return nil, apperrors.Conflict("published template version not found", map[string]any{"field": "published_version_id"})
	}
	source, err := r.LockVersionByID(ctx, published.ID, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.EnsureTemplateVersionForkSource(source.Status); err != nil {
		return nil, err
	}

	maxVersionNumber := 0
	if err := r.db().QueryRow(ctx, `
		SELECT COALESCE(MAX(version_number), 0)
		FROM rfx.rfx_template_versions
		WHERE template_id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, templateID, tenantID).Scan(&maxVersionNumber); err != nil {
		return nil, mapDBError(err)
	}

	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_template_versions (tenant_id, template_id, version_number, status, created_by)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING `+rfxTemplateVersionSelectColumns,
		tenantID, templateID, maxVersionNumber+1, domain.RfxVersionStatusDraft, forkActorUserID)
	draft, err := scanRfxTemplateVersion(row)
	if err != nil {
		return nil, mapDBError(err)
	}

	if _, err := copyTemplateGraph(ctx, qRepo, tenantID, templateID, source.ID, draft.ID); err != nil {
		return nil, err
	}

	if _, err := r.db().Exec(ctx, `
		UPDATE rfx.rfx_templates SET updated_at=now(), version=version+1
		WHERE id=$1 AND tenant_id=$2 AND deleted_at IS NULL`, templateID, tenantID); err != nil {
		return nil, mapDBError(err)
	}

	draft.IsActiveDraft = true
	return draft, nil
}

func copyTemplateGraph(
	ctx context.Context,
	qRepo *TemplateQuestionnaireRepository,
	tenantID, templateID, sourceVersionID, targetVersionID uuid.UUID,
) (map[uuid.UUID]uuid.UUID, error) {
	definition, err := qRepo.LoadQuestionnaire(ctx, templateID, sourceVersionID, tenantID)
	if err != nil {
		return nil, err
	}
	questionIDMap := make(map[uuid.UUID]uuid.UUID)
	for _, sectionWithQuestions := range definition.Sections {
		section, err := qRepo.CreateSection(ctx, tenantID, templateID, targetVersionID, domain.CreateSectionInput{
			SectionCode: sectionWithQuestions.Section.SectionCode,
			Title:       sectionWithQuestions.Section.Title,
			Description: sectionWithQuestions.Section.Description,
			SortOrder:   intPtr(sectionWithQuestions.Section.SortOrder),
		})
		if err != nil {
			return nil, err
		}
		for _, sourceQuestion := range sectionWithQuestions.Questions {
			question, err := qRepo.CreateQuestion(ctx, tenantID, section.ID, domain.CreateQuestionInput{
				QuestionCode:       sourceQuestion.QuestionCode,
				QuestionType:       sourceQuestion.QuestionType,
				Label:              sourceQuestion.Label,
				HelpText:           sourceQuestion.HelpText,
				Required:           sourceQuestion.Required,
				ValidationRuleJSON: sourceQuestion.ValidationRuleJSON,
				SortOrder:          intPtr(sourceQuestion.SortOrder),
			})
			if err != nil {
				return nil, err
			}
			questionIDMap[sourceQuestion.ID] = question.ID
			for _, option := range sourceQuestion.Options {
				if _, err := qRepo.CreateOption(ctx, tenantID, question.ID, domain.CreateQuestionOptionInput{
					OptionCode: option.OptionCode,
					Label:      option.Label,
					SortOrder:  intPtr(option.SortOrder),
				}); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, rule := range definition.Rules {
		var targetQuestionID *uuid.UUID
		if rule.TargetQuestionID != nil {
			mappedID, ok := questionIDMap[*rule.TargetQuestionID]
			if !ok {
				return nil, apperrors.Internal("failed to map copied rule target question", nil)
			}
			targetQuestionID = &mappedID
		}
		if _, err := qRepo.CreateRule(ctx, tenantID, templateID, targetVersionID, targetQuestionID, domain.CreateQuestionRuleInput{
			RuleCode: rule.RuleCode, Action: rule.Action, ConditionJSON: rule.ConditionJSON, SortOrder: intPtr(rule.SortOrder),
		}); err != nil {
			return nil, err
		}
	}
	return questionIDMap, nil
}
