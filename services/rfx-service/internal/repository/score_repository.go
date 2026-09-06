package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type ScoreRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewScoreRepository(pool *pgxpool.Pool) *ScoreRepository {
	return &ScoreRepository{pool: pool}
}

func (r *ScoreRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *ScoreRepository) WithTx(tx pgx.Tx) *ScoreRepository {
	return &ScoreRepository{pool: r.pool, exec: tx}
}

func (r *ScoreRepository) GetOrCreateDraftModel(ctx context.Context, tenantID, versionID uuid.UUID, createdBy *uuid.UUID) (*domain.ScoreModel, error) {
	const existing = `
		SELECT id, tenant_id, rfx_version_id, model_version, status, model_type, definition_json,
			created_by, created_at, updated_at, published_at
		FROM rfx.rfx_score_models
		WHERE tenant_id = $1 AND rfx_version_id = $2 AND status = 'DRAFT'
		ORDER BY model_version DESC
		LIMIT 1
	`
	if model, err := r.scanScoreModel(r.db().QueryRow(ctx, existing, tenantID, versionID)); err == nil {
		return model, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, mapDBError(err)
	}

	if _, err := r.GetPublishedModelForVersion(ctx, tenantID, versionID); err == nil {
		return nil, apperrors.Conflict("published score model is immutable", map[string]any{"field": "status"})
	} else {
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
			return nil, err
		}
	}

	const insert = `
		INSERT INTO rfx.rfx_score_models (
			tenant_id, rfx_version_id, model_version, status, model_type, definition_json, created_by
		) VALUES ($1, $2, 1, 'DRAFT', 'AUTOMATIC', '{}'::jsonb, $3)
		RETURNING id, tenant_id, rfx_version_id, model_version, status, model_type, definition_json,
			created_by, created_at, updated_at, published_at
	`
	return r.scanScoreModel(r.db().QueryRow(ctx, insert, tenantID, versionID, createdBy))
}

func (r *ScoreRepository) GetModelByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ScoreModel, error) {
	const query = `
		SELECT id, tenant_id, rfx_version_id, model_version, status, model_type, definition_json,
			created_by, created_at, updated_at, published_at
		FROM rfx.rfx_score_models
		WHERE id = $1 AND tenant_id = $2
	`
	model, err := r.scanScoreModel(r.db().QueryRow(ctx, query, id, tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("score model not found")
		}
		return nil, mapDBError(err)
	}
	return model, nil
}

func (r *ScoreRepository) GetPublishedModelForVersion(ctx context.Context, tenantID, versionID uuid.UUID) (*domain.ScoreModel, error) {
	const query = `
		SELECT id, tenant_id, rfx_version_id, model_version, status, model_type, definition_json,
			created_by, created_at, updated_at, published_at
		FROM rfx.rfx_score_models
		WHERE tenant_id = $1 AND rfx_version_id = $2 AND status = 'PUBLISHED'
		LIMIT 1
	`
	model, err := r.scanScoreModel(r.db().QueryRow(ctx, query, tenantID, versionID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("published score model not found")
		}
		return nil, mapDBError(err)
	}
	return model, nil
}

func (r *ScoreRepository) GetDraftModelForVersion(ctx context.Context, tenantID, versionID uuid.UUID) (*domain.ScoreModel, error) {
	const query = `
		SELECT id, tenant_id, rfx_version_id, model_version, status, model_type, definition_json,
			created_by, created_at, updated_at, published_at
		FROM rfx.rfx_score_models
		WHERE tenant_id = $1 AND rfx_version_id = $2 AND status = 'DRAFT'
		ORDER BY model_version DESC
		LIMIT 1
	`
	model, err := r.scanScoreModel(r.db().QueryRow(ctx, query, tenantID, versionID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("draft score model not found")
		}
		return nil, mapDBError(err)
	}
	return model, nil
}

func (r *ScoreRepository) ReplaceDraftDefinition(
	ctx context.Context,
	modelID, tenantID uuid.UUID,
	criteria []domain.ScoreCriterion,
	bindings []domain.ScoreBinding,
) error {
	model, err := r.GetModelByID(ctx, modelID, tenantID)
	if err != nil {
		return err
	}
	if model.Status != domain.ScoreModelStatusDraft {
		return apperrors.Conflict("published score model is immutable", map[string]any{"field": "status"})
	}

	if _, err := r.db().Exec(ctx, `DELETE FROM rfx.rfx_score_bindings WHERE score_model_id = $1 AND tenant_id = $2`, modelID, tenantID); err != nil {
		return mapDBError(err)
	}
	if _, err := r.db().Exec(ctx, `DELETE FROM rfx.rfx_score_criteria WHERE score_model_id = $1 AND tenant_id = $2`, modelID, tenantID); err != nil {
		return mapDBError(err)
	}

	for _, c := range criteria {
		const insertCriterion = `
			INSERT INTO rfx.rfx_score_criteria (
				id, tenant_id, score_model_id, criterion_code, name, weight, normalization_json, sort_order
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`
		if _, err := r.db().Exec(ctx, insertCriterion,
			c.ID, tenantID, modelID, c.CriterionCode, c.Name, c.Weight, c.NormalizationJSON, c.SortOrder,
		); err != nil {
			return mapDBError(err)
		}
	}
	for _, b := range bindings {
		const insertBinding = `
			INSERT INTO rfx.rfx_score_bindings (
				id, tenant_id, score_model_id, criterion_id, question_id, binding_type,
				scoring_rule_json, knockout_rule_json
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`
		if _, err := r.db().Exec(ctx, insertBinding,
			b.ID, tenantID, modelID, b.CriterionID, b.QuestionID, b.BindingType,
			defaultJSON(b.ScoringRuleJSON), nullableJSON(b.KnockoutRuleJSON),
		); err != nil {
			return mapDBError(err)
		}
	}
	if _, err := r.db().Exec(ctx, `UPDATE rfx.rfx_score_models SET updated_at = now() WHERE id = $1 AND tenant_id = $2`, modelID, tenantID); err != nil {
		return mapDBError(err)
	}
	return nil
}

func (r *ScoreRepository) ListCriteriaByModel(ctx context.Context, modelID, tenantID uuid.UUID) ([]domain.ScoreCriterion, error) {
	const query = `
		SELECT id, tenant_id, score_model_id, criterion_code, name, weight, normalization_json,
			sort_order, created_at, updated_at
		FROM rfx.rfx_score_criteria
		WHERE score_model_id = $1 AND tenant_id = $2
		ORDER BY sort_order, criterion_code
	`
	rows, err := r.db().Query(ctx, query, modelID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.ScoreCriterion, 0)
	for rows.Next() {
		var c domain.ScoreCriterion
		if err := rows.Scan(
			&c.ID, &c.TenantID, &c.ScoreModelID, &c.CriterionCode, &c.Name, &c.Weight,
			&c.NormalizationJSON, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ScoreRepository) ListBindingsByModel(ctx context.Context, modelID, tenantID uuid.UUID) ([]domain.ScoreBinding, error) {
	const query = `
		SELECT id, tenant_id, score_model_id, criterion_id, question_id, binding_type,
			scoring_rule_json, knockout_rule_json, created_at, updated_at
		FROM rfx.rfx_score_bindings
		WHERE score_model_id = $1 AND tenant_id = $2
	`
	rows, err := r.db().Query(ctx, query, modelID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.ScoreBinding, 0)
	for rows.Next() {
		var b domain.ScoreBinding
		var knockout []byte
		if err := rows.Scan(
			&b.ID, &b.TenantID, &b.ScoreModelID, &b.CriterionID, &b.QuestionID, &b.BindingType,
			&b.ScoringRuleJSON, &knockout, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, mapDBError(err)
		}
		if len(knockout) > 0 {
			b.KnockoutRuleJSON = knockout
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *ScoreRepository) PublishModel(ctx context.Context, modelID, tenantID uuid.UUID) (*domain.ScoreModel, error) {
	model, err := r.GetModelByID(ctx, modelID, tenantID)
	if err != nil {
		return nil, err
	}
	if model.Status != domain.ScoreModelStatusDraft {
		return nil, apperrors.Conflict("score model is already published", map[string]any{"field": "status"})
	}
	const query = `
		UPDATE rfx.rfx_score_models
		SET status = 'PUBLISHED', published_at = now(), updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND status = 'DRAFT'
		RETURNING id, tenant_id, rfx_version_id, model_version, status, model_type, definition_json,
			created_by, created_at, updated_at, published_at
	`
	return r.scanScoreModel(r.db().QueryRow(ctx, query, modelID, tenantID))
}

func (r *ScoreRepository) ReplaceScoringResults(
	ctx context.Context,
	tenantID uuid.UUID,
	responseID uuid.UUID,
	model domain.ScoreModel,
	qualification domain.QualificationResult,
	answerScores []domain.AnswerScore,
) error {
	if _, err := r.db().Exec(ctx,
		`DELETE FROM rfx.rfx_answer_scores WHERE rfx_response_id = $1 AND tenant_id = $2 AND score_model_version = $3`,
		responseID, tenantID, model.ModelVersion,
	); err != nil {
		return mapDBError(err)
	}

	for _, s := range answerScores {
		const insertScore = `
			INSERT INTO rfx.rfx_answer_scores (
				id, tenant_id, rfx_response_id, answer_id, criterion_id, score_model_id, score_model_version,
				raw_score, normalized_score, weighted_contribution, explanation_json, calculated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		`
		if _, err := r.db().Exec(ctx, insertScore,
			s.ID, tenantID, responseID, s.AnswerID, s.CriterionID, model.ID, model.ModelVersion,
			s.RawScore, s.NormalizedScore, s.WeightedContribution, s.ExplanationJSON, s.CalculatedAt,
		); err != nil {
			return mapDBError(err)
		}
	}

	const upsertQual = `
		INSERT INTO rfx.rfx_qualification_results (
			id, tenant_id, rfx_response_id, score_model_id, score_model_version,
			status, calculation_status, total_score, knockout_triggered, knockout_reason_json, calculated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (rfx_response_id, score_model_version) DO UPDATE SET
			score_model_id = EXCLUDED.score_model_id,
			status = EXCLUDED.status,
			calculation_status = EXCLUDED.calculation_status,
			total_score = EXCLUDED.total_score,
			knockout_triggered = EXCLUDED.knockout_triggered,
			knockout_reason_json = EXCLUDED.knockout_reason_json,
			calculated_at = EXCLUDED.calculated_at,
			updated_at = now()
	`
	if _, err := r.db().Exec(ctx, upsertQual,
		qualification.ID, tenantID, responseID, model.ID, model.ModelVersion,
		qualification.Status, qualification.CalculationStatus, qualification.TotalScore,
		qualification.KnockoutTriggered, nullableJSON(qualification.KnockoutReasonJSON), qualification.CalculatedAt,
	); err != nil {
		return mapDBError(err)
	}
	return nil
}

func (r *ScoreRepository) MarkScoringFailed(ctx context.Context, tenantID, responseID uuid.UUID, model domain.ScoreModel) error {
	now := time.Now().UTC()
	const upsert = `
		INSERT INTO rfx.rfx_qualification_results (
			id, tenant_id, rfx_response_id, score_model_id, score_model_version,
			status, calculation_status, total_score, knockout_triggered, calculated_at
		) VALUES ($1,$2,$3,$4,$5,'PENDING_REVIEW','FAILED',NULL,false,$6)
		ON CONFLICT (rfx_response_id, score_model_version) DO UPDATE SET
			calculation_status = 'FAILED',
			updated_at = now()
	`
	_, err := r.db().Exec(ctx, upsert, uuid.New(), tenantID, responseID, model.ID, model.ModelVersion, now)
	return mapDBError(err)
}

func (r *ScoreRepository) GetQualificationResult(ctx context.Context, responseID, tenantID uuid.UUID, modelVersion int) (*domain.QualificationResult, error) {
	const query = `
		SELECT id, tenant_id, rfx_response_id, score_model_id, score_model_version, status,
			calculation_status, total_score, knockout_triggered, knockout_reason_json, calculated_at,
			created_at, updated_at
		FROM rfx.rfx_qualification_results
		WHERE rfx_response_id = $1 AND tenant_id = $2 AND score_model_version = $3
	`
	return r.scanQualification(r.db().QueryRow(ctx, query, responseID, tenantID, modelVersion))
}

func (r *ScoreRepository) GetLatestQualificationForResponse(ctx context.Context, responseID, tenantID uuid.UUID) (*domain.QualificationResult, error) {
	const query = `
		SELECT id, tenant_id, rfx_response_id, score_model_id, score_model_version, status,
			calculation_status, total_score, knockout_triggered, knockout_reason_json, calculated_at,
			created_at, updated_at
		FROM rfx.rfx_qualification_results
		WHERE rfx_response_id = $1 AND tenant_id = $2
		ORDER BY score_model_version DESC
		LIMIT 1
	`
	result, err := r.scanQualification(r.db().QueryRow(ctx, query, responseID, tenantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("qualification result not found")
		}
		return nil, mapDBError(err)
	}
	return result, nil
}

func (r *ScoreRepository) ListAnswerScores(ctx context.Context, responseID, tenantID uuid.UUID, modelVersion int) ([]domain.AnswerScore, error) {
	const query = `
		SELECT id, tenant_id, rfx_response_id, answer_id, criterion_id, score_model_id, score_model_version,
			raw_score, normalized_score, weighted_contribution, explanation_json, calculated_at
		FROM rfx.rfx_answer_scores
		WHERE rfx_response_id = $1 AND tenant_id = $2 AND score_model_version = $3
		ORDER BY calculated_at
	`
	rows, err := r.db().Query(ctx, query, responseID, tenantID, modelVersion)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	out := make([]domain.AnswerScore, 0)
	for rows.Next() {
		var s domain.AnswerScore
		if err := rows.Scan(
			&s.ID, &s.TenantID, &s.RfxResponseID, &s.AnswerID, &s.CriterionID, &s.ScoreModelID, &s.ScoreModelVersion,
			&s.RawScore, &s.NormalizedScore, &s.WeightedContribution, &s.ExplanationJSON, &s.CalculatedAt,
		); err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ScoreRepository) AssertVersionBelongsToTenant(ctx context.Context, versionID, tenantID uuid.UUID) error {
	const query = `SELECT 1 FROM rfx.rfx_versions WHERE id = $1 AND tenant_id = $2`
	var one int
	err := r.db().QueryRow(ctx, query, versionID, tenantID).Scan(&one)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.NotFound("rfx version not found")
		}
		return mapDBError(err)
	}
	return nil
}

func (r *ScoreRepository) scanScoreModel(row pgx.Row) (*domain.ScoreModel, error) {
	var m domain.ScoreModel
	err := row.Scan(
		&m.ID, &m.TenantID, &m.RfxVersionID, &m.ModelVersion, &m.Status, &m.ModelType, &m.DefinitionJSON,
		&m.CreatedBy, &m.CreatedAt, &m.UpdatedAt, &m.PublishedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *ScoreRepository) scanQualification(row pgx.Row) (*domain.QualificationResult, error) {
	var q domain.QualificationResult
	var knockout []byte
	err := row.Scan(
		&q.ID, &q.TenantID, &q.RfxResponseID, &q.ScoreModelID, &q.ScoreModelVersion, &q.Status,
		&q.CalculationStatus, &q.TotalScore, &q.KnockoutTriggered, &knockout, &q.CalculatedAt,
		&q.CreatedAt, &q.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(knockout) > 0 {
		q.KnockoutReasonJSON = knockout
	}
	return &q, nil
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func defaultJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
