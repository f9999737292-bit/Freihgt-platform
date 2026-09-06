package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type AnswerRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewAnswerRepository(pool *pgxpool.Pool) *AnswerRepository {
	return &AnswerRepository{pool: pool}
}

func (r *AnswerRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *AnswerRepository) ListByResponse(ctx context.Context, responseID, tenantID uuid.UUID) ([]domain.CarrierAnswer, error) {
	const query = `
		SELECT id, tenant_id, rfx_response_id, question_id, answer_value_json, answer_source,
			validation_version, rule_version, updated_by, updated_at, version
		FROM rfx.rfx_answers
		WHERE rfx_response_id = $1 AND tenant_id = $2
		ORDER BY updated_at
	`
	rows, err := r.db().Query(ctx, query, responseID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	answers := make([]domain.CarrierAnswer, 0)
	for rows.Next() {
		answer, err := scanCarrierAnswer(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		answers = append(answers, *answer)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return answers, nil
}

func (r *AnswerRepository) UpsertBatch(
	ctx context.Context,
	tenantID, responseID uuid.UUID,
	patches []domain.AnswerPatchItem,
	updatedBy uuid.UUID,
) error {
	const query = `
		INSERT INTO rfx.rfx_answers (
			tenant_id, rfx_response_id, question_id, answer_value_json, answer_source,
			validation_version, updated_by, updated_at, version
		) VALUES ($1, $2, $3, $4, $5, 1, $6, now(), 1)
		ON CONFLICT (rfx_response_id, question_id) DO UPDATE SET
			answer_value_json = EXCLUDED.answer_value_json,
			updated_by = EXCLUDED.updated_by,
			updated_at = now(),
			version = rfx.rfx_answers.version + 1
	`
	for _, patch := range patches {
		if _, err := r.db().Exec(ctx, query,
			tenantID,
			responseID,
			patch.QuestionID,
			json.RawMessage(patch.Value),
			domain.AnswerSourceCarrierDeclared,
			optionalUUID(&updatedBy),
		); err != nil {
			return mapDBError(err)
		}
	}
	return nil
}

func (r *AnswerRepository) DeleteByQuestionIDs(ctx context.Context, responseID, tenantID uuid.UUID, questionIDs []uuid.UUID) error {
	if len(questionIDs) == 0 {
		return nil
	}
	const query = `
		DELETE FROM rfx.rfx_answers
		WHERE rfx_response_id = $1 AND tenant_id = $2 AND question_id = ANY($3)
	`
	if _, err := r.db().Exec(ctx, query, responseID, tenantID, questionIDs); err != nil {
		return mapDBError(err)
	}
	return nil
}

func scanCarrierAnswer(row pgx.Row) (*domain.CarrierAnswer, error) {
	var answer domain.CarrierAnswer
	var raw []byte
	var ruleVersion *int
	var updatedBy *uuid.UUID
	err := row.Scan(
		&answer.ID,
		&answer.TenantID,
		&answer.RfxResponseID,
		&answer.QuestionID,
		&raw,
		&answer.AnswerSource,
		&answer.ValidationVersion,
		&ruleVersion,
		&updatedBy,
		&answer.UpdatedAt,
		&answer.Version,
	)
	if err != nil {
		return nil, err
	}
	answer.AnswerValueJSON = json.RawMessage(raw)
	answer.RuleVersion = ruleVersion
	answer.UpdatedBy = updatedBy
	return &answer, nil
}

func answersByQuestionID(answers []domain.CarrierAnswer) map[uuid.UUID]json.RawMessage {
	out := make(map[uuid.UUID]json.RawMessage, len(answers))
	for _, a := range answers {
		out[a.QuestionID] = a.AnswerValueJSON
	}
	return out
}

func (r *AnswerRepository) GetAnswerMap(ctx context.Context, responseID, tenantID uuid.UUID) (map[uuid.UUID]json.RawMessage, error) {
	list, err := r.ListByResponse(ctx, responseID, tenantID)
	if err != nil {
		return nil, err
	}
	return answersByQuestionID(list), nil
}

func (r *AnswerRepository) AssertResponseOwned(ctx context.Context, responseID, tenantID uuid.UUID) error {
	const query = `SELECT EXISTS (SELECT 1 FROM rfx.rfx_responses WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL)`
	var exists bool
	if err := r.db().QueryRow(ctx, query, responseID, tenantID).Scan(&exists); err != nil {
		return mapDBError(err)
	}
	if !exists {
		return apperrors.NotFound("rfx response not found")
	}
	return nil
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
