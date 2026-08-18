package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func (r *RfxRepository) LotBelongsToEvent(ctx context.Context, lotID, eventID, tenantID uuid.UUID) (bool, error) {
	if lotID == uuid.Nil {
		return true, nil
	}
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM rfx.rfx_lots
			WHERE id = $1 AND rfx_event_id = $2 AND tenant_id = $3
		)
	`
	var exists bool
	if err := r.db().QueryRow(ctx, query, lotID, eventID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *RfxRepository) ReplaceOfferLines(ctx context.Context, responseID, tenantID uuid.UUID, lines []domain.UpsertOfferLineInput) ([]domain.RfxResponseOfferLine, error) {
	const deleteQuery = `DELETE FROM rfx.rfx_response_offer_lines WHERE rfx_response_id = $1 AND tenant_id = $2`
	if _, err := r.db().Exec(ctx, deleteQuery, responseID, tenantID); err != nil {
		return nil, mapDBError(err)
	}
	if len(lines) == 0 {
		return []domain.RfxResponseOfferLine{}, nil
	}
	const insertQuery = `
		INSERT INTO rfx.rfx_response_offer_lines (
			tenant_id, rfx_response_id, rfx_lot_id, amount, currency_code, comment
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, tenant_id, rfx_response_id, rfx_lot_id, amount, currency_code, comment, version
	`
	out := make([]domain.RfxResponseOfferLine, 0, len(lines))
	for _, line := range lines {
		var lotID *uuid.UUID
		if line.RfxLotID != uuid.Nil {
			id := line.RfxLotID
			lotID = &id
		}
		row := r.db().QueryRow(ctx, insertQuery,
			tenantID,
			responseID,
			lotID,
			line.Amount,
			domain.NormalizeCurrencyCode(line.CurrencyCode),
			line.Comment,
		)
		parsed, err := scanOfferLine(row)
		if err != nil {
			return nil, mapDBError(err)
		}
		out = append(out, *parsed)
	}
	return out, nil
}

func (r *RfxRepository) ListOfferLinesByResponse(ctx context.Context, responseID, tenantID uuid.UUID) ([]domain.RfxResponseOfferLine, error) {
	const query = `
		SELECT id, tenant_id, rfx_response_id, rfx_lot_id, amount, currency_code, comment, version
		FROM rfx.rfx_response_offer_lines
		WHERE rfx_response_id = $1 AND tenant_id = $2
		ORDER BY created_at
	`
	rows, err := r.db().Query(ctx, query, responseID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	lines := make([]domain.RfxResponseOfferLine, 0)
	for rows.Next() {
		line, err := scanOfferLine(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		lines = append(lines, *line)
	}
	return lines, rows.Err()
}

func (r *RfxRepository) ListSubmittedResponsesByEvent(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxResponse, error) {
	const query = `
		SELECT id, tenant_id, rfx_event_id, participant_company_id, status,
			submitted_at, commercial_score, technical_score, total_score, evaluation_rank,
			created_at, updated_at, version
		FROM rfx.rfx_responses
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = $3
		ORDER BY submitted_at NULLS LAST, created_at
	`
	rows, err := r.db().Query(ctx, query, eventID, tenantID, domain.RfxResponseStatusSubmitted)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	responses := make([]domain.RfxResponse, 0)
	for rows.Next() {
		response, err := scanRfxResponseEvaluation(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		responses = append(responses, *response)
	}
	return responses, rows.Err()
}

func (r *RfxRepository) UpdateResponseEvaluation(ctx context.Context, responseID, tenantID uuid.UUID, commercialScore, manualScore, totalScore *float64, rank *int) error {
	const query = `
		UPDATE rfx.rfx_responses SET
			commercial_score = $3,
			technical_score = $4,
			total_score = $5,
			evaluation_rank = $6,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	tag, err := r.db().Exec(ctx, query, responseID, tenantID, commercialScore, manualScore, totalScore, rank)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("rfx response not found")
	}
	return nil
}

func (r *RfxRepository) UpdateParticipantStatus(ctx context.Context, eventID, companyID, tenantID uuid.UUID, status string) error {
	const query = `
		UPDATE rfx.rfx_participants SET status = $4
		WHERE rfx_event_id = $1 AND company_id = $2 AND tenant_id = $3
	`
	tag, err := r.db().Exec(ctx, query, eventID, companyID, tenantID, status)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("participant not found")
	}
	return nil
}

func (r *RfxRepository) GetAwardByEvent(ctx context.Context, eventID, tenantID uuid.UUID) (*domain.RfxAward, error) {
	const query = `
		SELECT id, tenant_id, rfx_event_id, rfx_response_id, carrier_company_id,
			total_amount, currency_code, awarded_by, awarded_at, version
		FROM rfx.rfx_awards
		WHERE rfx_event_id = $1 AND tenant_id = $2
	`
	row := r.db().QueryRow(ctx, query, eventID, tenantID)
	award, err := scanRfxAward(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx award not found")
		}
		return nil, mapDBError(err)
	}
	return award, nil
}

func (r *RfxRepository) GetAwardForCarrier(ctx context.Context, eventID, carrierCompanyID, tenantID uuid.UUID) (*domain.RfxAward, error) {
	const query = `
		SELECT id, tenant_id, rfx_event_id, rfx_response_id, carrier_company_id,
			total_amount, currency_code, awarded_by, awarded_at, version
		FROM rfx.rfx_awards
		WHERE rfx_event_id = $1 AND carrier_company_id = $2 AND tenant_id = $3
	`
	row := r.db().QueryRow(ctx, query, eventID, carrierCompanyID, tenantID)
	award, err := scanRfxAward(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx award not found")
		}
		return nil, mapDBError(err)
	}
	return award, nil
}

func (r *RfxRepository) CreateAwardTransactional(
	ctx context.Context,
	in domain.CreateRfxAwardInput,
	newEventStatus string,
	preCommit func(context.Context, pgx.Tx) error,
) (*domain.RfxAward, error) {
	var result *domain.RfxAward
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	const lockEvent = `
		SELECT status FROM rfx.rfx_events
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		FOR UPDATE
	`
	var currentStatus string
	if err := tx.QueryRow(ctx, lockEvent, in.RfxEventID, in.TenantID).Scan(&currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx event not found")
		}
		return nil, mapDBError(err)
	}
	if err := domain.ValidateAwardEventStatus(currentStatus); err != nil {
		return errConflictIfAwardExists(ctx, tx, in.RfxEventID, in.TenantID, err)
	}

	const insertAward = `
		INSERT INTO rfx.rfx_awards (
			tenant_id, rfx_event_id, rfx_response_id, carrier_company_id,
			total_amount, currency_code, awarded_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, rfx_event_id, rfx_response_id, carrier_company_id,
			total_amount, currency_code, awarded_by, awarded_at, version
	`
	row := tx.QueryRow(ctx, insertAward,
		in.TenantID,
		in.RfxEventID,
		in.RfxResponseID,
		in.CarrierCompanyID,
		in.TotalAmount,
		in.CurrencyCode,
		in.AwardedBy,
	)
	award, err := scanRfxAward(row)
	if err != nil {
		return nil, mapDBError(err)
	}

	const updateWinner = `
		UPDATE rfx.rfx_participants SET status = $4
		WHERE rfx_event_id = $1 AND company_id = $2 AND tenant_id = $3
	`
	if _, err := tx.Exec(ctx, updateWinner, in.RfxEventID, in.CarrierCompanyID, in.TenantID, domain.ParticipantStatusAwarded); err != nil {
		return nil, mapDBError(err)
	}

	const updateLosers = `
		UPDATE rfx.rfx_participants SET status = $4
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND company_id <> $3
			AND status = ANY($5::text[])
	`
	if _, err := tx.Exec(ctx, updateLosers,
		in.RfxEventID,
		in.TenantID,
		in.CarrierCompanyID,
		domain.ParticipantStatusNotAwarded,
		[]string{
			domain.ParticipantStatusResponseSubmitted,
			domain.ParticipantStatusShortlisted,
			domain.ParticipantStatusInvited,
		},
	); err != nil {
		return nil, mapDBError(err)
	}

	const updateEvent = `
		UPDATE rfx.rfx_events SET status = $3, updated_at = now(), version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	if _, err := tx.Exec(ctx, updateEvent, in.RfxEventID, in.TenantID, newEventStatus); err != nil {
		return nil, mapDBError(err)
	}

	if preCommit != nil {
		if err := preCommit(ctx, tx); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, mapDBError(err)
	}
	result = award
	return result, nil
}

func errConflictIfAwardExists(ctx context.Context, tx pgx.Tx, eventID, tenantID uuid.UUID, baseErr error) (*domain.RfxAward, error) {
	var exists bool
	_ = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM rfx.rfx_awards WHERE rfx_event_id = $1 AND tenant_id = $2)`, eventID, tenantID).Scan(&exists)
	if exists {
		return nil, apperrors.Conflict("rfx event is already awarded", map[string]any{"field": "status"})
	}
	return nil, baseErr
}

func scanOfferLine(row pgx.Row) (*domain.RfxResponseOfferLine, error) {
	var line domain.RfxResponseOfferLine
	var lotID *uuid.UUID
	err := row.Scan(
		&line.ID,
		&line.TenantID,
		&line.RfxResponseID,
		&lotID,
		&line.Amount,
		&line.CurrencyCode,
		&line.Comment,
		&line.Version,
	)
	if err != nil {
		return nil, err
	}
	if lotID != nil {
		line.RfxLotID = *lotID
	}
	return &line, nil
}

func scanRfxResponseEvaluation(row pgx.Row) (*domain.RfxResponse, error) {
	var response domain.RfxResponse
	err := row.Scan(
		&response.ID,
		&response.TenantID,
		&response.RfxEventID,
		&response.ParticipantCompanyID,
		&response.Status,
		&response.SubmittedAt,
		&response.CommercialScore,
		&response.ManualScore,
		&response.TotalScore,
		&response.EvaluationRank,
		&response.CreatedAt,
		&response.UpdatedAt,
		&response.Version,
	)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func scanRfxAward(row pgx.Row) (*domain.RfxAward, error) {
	var award domain.RfxAward
	err := row.Scan(
		&award.ID,
		&award.TenantID,
		&award.RfxEventID,
		&award.RfxResponseID,
		&award.CarrierCompanyID,
		&award.TotalAmount,
		&award.CurrencyCode,
		&award.AwardedBy,
		&award.AwardedAt,
		&award.Version,
	)
	if err != nil {
		return nil, err
	}
	return &award, nil
}
