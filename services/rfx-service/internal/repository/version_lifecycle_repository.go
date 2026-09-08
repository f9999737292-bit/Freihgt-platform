package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type EventVersionState struct {
	EventID            uuid.UUID
	TenantID           uuid.UUID
	EventVersion       int
	DraftVersionID     *uuid.UUID
	PublishedVersionID *uuid.UUID
}

type PublishVersionResult struct {
	Published  *domain.RfxVersion
	Superseded *domain.RfxVersion
}

func (r *QuestionnaireRepository) LockEventVersionState(ctx context.Context, eventID, tenantID uuid.UUID) (*EventVersionState, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, version, draft_version_id, published_version_id
		FROM rfx.rfx_events
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		FOR UPDATE
	`, eventID, tenantID)
	var state EventVersionState
	if err := row.Scan(&state.EventID, &state.TenantID, &state.EventVersion, &state.DraftVersionID, &state.PublishedVersionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx event not found")
		}
		return nil, mapDBError(err)
	}
	return &state, nil
}

func (r *QuestionnaireRepository) GetEventVersionState(ctx context.Context, eventID, tenantID uuid.UUID) (*EventVersionState, error) {
	row := r.db().QueryRow(ctx, `
		SELECT id, tenant_id, version, draft_version_id, published_version_id
		FROM rfx.rfx_events
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, eventID, tenantID)
	var state EventVersionState
	if err := row.Scan(&state.EventID, &state.TenantID, &state.EventVersion, &state.DraftVersionID, &state.PublishedVersionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx event not found")
		}
		return nil, mapDBError(err)
	}
	return &state, nil
}

func (r *QuestionnaireRepository) LockVersionByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxVersion, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+rfxVersionSelectColumns+`
		FROM rfx.rfx_versions
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		FOR UPDATE
	`, id, tenantID)
	version, err := scanRfxVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx version not found")
		}
		return nil, mapDBError(err)
	}
	return version, nil
}

func (r *QuestionnaireRepository) ListVersionsForEvent(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxVersion, error) {
	rows, err := r.db().Query(ctx, `
		SELECT `+rfxVersionSelectColumns+`
		FROM rfx.rfx_versions
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		ORDER BY version_number DESC
	`, eventID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	versions := make([]domain.RfxVersion, 0)
	for rows.Next() {
		version, err := scanRfxVersion(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		versions = append(versions, *version)
	}
	return versions, rows.Err()
}

func (r *QuestionnaireRepository) GetVersionForEvent(ctx context.Context, eventID, versionID, tenantID uuid.UUID) (*domain.RfxVersion, error) {
	row := r.db().QueryRow(ctx, `
		SELECT `+rfxVersionSelectColumns+`
		FROM rfx.rfx_versions
		WHERE id = $1 AND rfx_event_id = $2 AND tenant_id = $3 AND deleted_at IS NULL
	`, versionID, eventID, tenantID)
	version, err := scanRfxVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx version not found")
		}
		return nil, mapDBError(err)
	}
	return version, nil
}

func (r *QuestionnaireRepository) PublishVersionTx(
	ctx context.Context,
	eventID, tenantID uuid.UUID,
	expectedVersion int,
	changeSummary string,
	publishedBy uuid.UUID,
	rescoringRequired bool,
) (*PublishVersionResult, error) {
	state, err := r.LockEventVersionState(ctx, eventID, tenantID)
	if err != nil {
		return nil, err
	}
	if state.DraftVersionID == nil {
		return nil, apperrors.Conflict("draft questionnaire version not found", map[string]any{"field": "draft_version_id"})
	}

	draft, err := r.LockVersionByID(ctx, *state.DraftVersionID, tenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateVersionPublishStatus(draft.Status); err != nil {
		return nil, err
	}
	if draft.Version != expectedVersion {
		return nil, apperrors.Conflict("questionnaire version was updated by another request", map[string]any{"field": "version"})
	}

	var superseded *domain.RfxVersion
	if state.PublishedVersionID != nil {
		superseded, err = r.LockVersionByID(ctx, *state.PublishedVersionID, tenantID)
		if err != nil {
			return nil, err
		}
	}

	if superseded != nil {
		row := r.db().QueryRow(ctx, `
			UPDATE rfx.rfx_versions
			SET status = $3,
				superseded_at = now(),
				superseded_by_version_id = $4,
				updated_at = now(),
				version = version + 1
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
			RETURNING `+rfxVersionSelectColumns+`
		`, superseded.ID, tenantID, domain.RfxVersionStatusSuperseded, draft.ID)
		superseded, err = scanRfxVersion(row)
		if err != nil {
			return nil, mapDBError(err)
		}
	}

	row := r.db().QueryRow(ctx, `
		UPDATE rfx.rfx_versions
		SET status = $3,
			change_summary = $4,
			published_at = now(),
			published_by = $5,
			superseded_at = NULL,
			superseded_by_version_id = NULL,
			rescoring_required = $6,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		RETURNING `+rfxVersionSelectColumns+`
	`, draft.ID, tenantID, domain.RfxVersionStatusPublished, strings.TrimSpace(changeSummary), publishedBy, rescoringRequired)
	published, err := scanRfxVersion(row)
	if err != nil {
		return nil, mapDBError(err)
	}

	if _, err := r.db().Exec(ctx, `
		UPDATE rfx.rfx_events
		SET published_version_id = $3,
			draft_version_id = NULL,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, eventID, tenantID, published.ID); err != nil {
		return nil, mapDBError(err)
	}

	return &PublishVersionResult{Published: published, Superseded: superseded}, nil
}

func (r *QuestionnaireRepository) ForkDraftFromPublishedTx(ctx context.Context, eventID, sourceVersionID, tenantID uuid.UUID) (*domain.RfxVersion, error) {
	state, err := r.LockEventVersionState(ctx, eventID, tenantID)
	if err != nil {
		return nil, err
	}
	if state.DraftVersionID != nil {
		return nil, apperrors.Conflict("draft questionnaire version already exists", map[string]any{"field": "draft_version_id"})
	}

	source, err := r.LockVersionByID(ctx, sourceVersionID, tenantID)
	if err != nil {
		return nil, err
	}
	if source.RfxEventID != eventID {
		return nil, apperrors.NotFound("rfx version not found")
	}
	if err := domain.ValidateVersionForkSourceStatus(source.Status); err != nil {
		return nil, err
	}

	maxVersionNumber := 0
	if err := r.db().QueryRow(ctx, `
		SELECT COALESCE(MAX(version_number), 0)
		FROM rfx.rfx_versions
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, eventID, tenantID).Scan(&maxVersionNumber); err != nil {
		return nil, mapDBError(err)
	}

	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_versions (
			tenant_id, rfx_event_id, version_number, status, questionnaire_enabled
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING `+rfxVersionSelectColumns+`
	`, tenantID, eventID, maxVersionNumber+1, domain.RfxVersionStatusDraft, source.QuestionnaireEnabled)
	draft, err := scanRfxVersion(row)
	if err != nil {
		return nil, mapDBError(err)
	}

	if _, err := r.copyQuestionnaireGraph(ctx, tenantID, source.ID, draft.ID); err != nil {
		return nil, err
	}

	if _, err := r.db().Exec(ctx, `
		UPDATE rfx.rfx_events
		SET draft_version_id = $3,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, eventID, tenantID, draft.ID); err != nil {
		return nil, mapDBError(err)
	}

	return draft, nil
}

func (r *QuestionnaireRepository) RestoreVersionAsDraftTx(
	ctx context.Context,
	eventID, sourceVersionID, tenantID uuid.UUID,
	changeSummary string,
) (*domain.RfxVersion, map[uuid.UUID]uuid.UUID, error) {
	state, err := r.LockEventVersionState(ctx, eventID, tenantID)
	if err != nil {
		return nil, nil, err
	}
	if state.DraftVersionID != nil {
		return nil, nil, apperrors.Conflict("draft questionnaire version already exists", map[string]any{"field": "draft_version_id"})
	}

	source, err := r.LockVersionByID(ctx, sourceVersionID, tenantID)
	if err != nil {
		return nil, nil, err
	}
	if source.RfxEventID != eventID {
		return nil, nil, apperrors.NotFound("rfx version not found")
	}
	if err := domain.ValidateVersionRestoreSourceStatus(source.Status); err != nil {
		return nil, nil, err
	}

	maxVersionNumber := 0
	if err := r.db().QueryRow(ctx, `
		SELECT COALESCE(MAX(version_number), 0)
		FROM rfx.rfx_versions
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, eventID, tenantID).Scan(&maxVersionNumber); err != nil {
		return nil, nil, mapDBError(err)
	}

	row := r.db().QueryRow(ctx, `
		INSERT INTO rfx.rfx_versions (
			tenant_id, rfx_event_id, version_number, status, questionnaire_enabled, change_summary
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+rfxVersionSelectColumns+`
	`, tenantID, eventID, maxVersionNumber+1, domain.RfxVersionStatusDraft, source.QuestionnaireEnabled, strings.TrimSpace(changeSummary))
	draft, err := scanRfxVersion(row)
	if err != nil {
		return nil, nil, mapDBError(err)
	}

	questionIDMap, err := r.copyQuestionnaireGraph(ctx, tenantID, source.ID, draft.ID)
	if err != nil {
		return nil, nil, err
	}

	if _, err := r.db().Exec(ctx, `
		UPDATE rfx.rfx_events
		SET draft_version_id = $3,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, eventID, tenantID, draft.ID); err != nil {
		return nil, nil, mapDBError(err)
	}

	return draft, questionIDMap, nil
}

func (r *QuestionnaireRepository) CountAffectedResponsesOnVersion(
	ctx context.Context,
	tenantID uuid.UUID,
	sourceVersionID uuid.UUID,
	scope domain.ResponseImpactScope,
) (draftCount, submittedCount int, err error) {
	if !scope.CountAllResponsesOnVersion && len(scope.QuestionCodesWithAnswers) == 0 {
		return 0, 0, nil
	}
	row := r.db().QueryRow(ctx, `
		WITH affected AS (
			SELECT DISTINCT rr.id, rr.status
			FROM rfx.rfx_responses rr
			WHERE rr.tenant_id = $1
				AND rr.rfx_version_id = $2
				AND rr.deleted_at IS NULL
				AND (
					$3::boolean
					OR EXISTS (
						SELECT 1
						FROM rfx.rfx_answers ra
						INNER JOIN rfx.rfx_questions q ON q.id = ra.question_id AND q.tenant_id = ra.tenant_id
						WHERE ra.rfx_response_id = rr.id
							AND ra.tenant_id = rr.tenant_id
							AND q.question_code = ANY($4)
					)
				)
		)
		SELECT
			COUNT(*) FILTER (WHERE status = $5),
			COUNT(*) FILTER (WHERE status = $6)
		FROM affected
	`, tenantID, sourceVersionID, scope.CountAllResponsesOnVersion, scope.QuestionCodesWithAnswers,
		domain.RfxResponseStatusDraft, domain.RfxResponseStatusSubmitted)
	if err := row.Scan(&draftCount, &submittedCount); err != nil {
		return 0, 0, mapDBError(err)
	}
	return draftCount, submittedCount, nil
}

func (r *QuestionnaireRepository) CountResponsesAndScoresForEvent(ctx context.Context, eventID, tenantID uuid.UUID) (int, int, error) {
	var responseCount int
	if err := r.db().QueryRow(ctx, `
		SELECT COUNT(*)
		FROM rfx.rfx_responses
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`, eventID, tenantID).Scan(&responseCount); err != nil {
		return 0, 0, mapDBError(err)
	}

	var scoredResponseCount int
	if err := r.db().QueryRow(ctx, `
		SELECT COUNT(DISTINCT qr.rfx_response_id)
		FROM rfx.rfx_qualification_results qr
		INNER JOIN rfx.rfx_responses rr
			ON rr.id = qr.rfx_response_id
			AND rr.tenant_id = qr.tenant_id
		WHERE rr.rfx_event_id = $1
			AND qr.tenant_id = $2
	`, eventID, tenantID).Scan(&scoredResponseCount); err != nil {
		return 0, 0, mapDBError(err)
	}

	return responseCount, scoredResponseCount, nil
}

func (r *QuestionnaireRepository) copyQuestionnaireGraph(ctx context.Context, tenantID, sourceVersionID, targetVersionID uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	definition, err := r.LoadQuestionnaire(ctx, sourceVersionID, tenantID)
	if err != nil {
		return nil, err
	}

	questionIDMap := make(map[uuid.UUID]uuid.UUID)
	for _, sectionWithQuestions := range definition.Sections {
		section, err := r.CreateSection(ctx, tenantID, targetVersionID, domain.CreateSectionInput{
			SectionCode: sectionWithQuestions.Section.SectionCode,
			Title:       sectionWithQuestions.Section.Title,
			Description: sectionWithQuestions.Section.Description,
			SortOrder:   intPtr(sectionWithQuestions.Section.SortOrder),
		})
		if err != nil {
			return nil, err
		}
		for _, sourceQuestion := range sectionWithQuestions.Questions {
			question, err := r.CreateQuestion(ctx, tenantID, section.ID, domain.CreateQuestionInput{
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
				if _, err := r.CreateOption(ctx, tenantID, question.ID, domain.CreateQuestionOptionInput{
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
		if _, err := r.CreateQuestionRule(ctx, tenantID, targetVersionID, targetQuestionID, domain.CreateQuestionRuleInput{
			RuleCode:      rule.RuleCode,
			Action:        rule.Action,
			ConditionJSON: rule.ConditionJSON,
			SortOrder:     intPtr(rule.SortOrder),
		}); err != nil {
			return nil, err
		}
	}

	return questionIDMap, nil
}
