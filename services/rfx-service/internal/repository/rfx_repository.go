package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type RfxRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewRfxRepository(pool *pgxpool.Pool) *RfxRepository {
	return &RfxRepository{pool: pool}
}

func (r *RfxRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.companies
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.db().QueryRow(ctx, query, companyID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *RfxRepository) CreateEvent(ctx context.Context, in domain.CreateRfxEventInput) (*domain.RfxEvent, error) {
	var result *domain.RfxEvent
	err := measureDB("rfx_repository", "create_rfx_event", func() error {
		const query = `
		INSERT INTO rfx.rfx_events (
			tenant_id, rfx_number, rfx_type, category, title, description,
			owner_company_id, currency_code, valid_from, valid_to, response_deadline, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, tenant_id, rfx_number, rfx_type, category, title, description,
			owner_company_id, status, currency_code, valid_from, valid_to, response_deadline,
			created_at, updated_at, version
	`
		row := r.db().QueryRow(ctx, query,
			in.TenantID,
			strings.TrimSpace(in.RfxNumber),
			strings.TrimSpace(in.RfxType),
			strings.TrimSpace(in.Category),
			strings.TrimSpace(in.Title),
			optionalString(in.Description),
			in.OwnerCompanyID,
			optionalString(in.CurrencyCode),
			optionalDate(in.ValidFrom),
			optionalDate(in.ValidTo),
			in.ResponseDeadline,
			domain.RfxStatusDraft,
		)
		event, err := scanRfxEvent(row)
		if err != nil {
			return mapDBError(err)
		}
		result = event
		return nil
	})
	return result, err
}

func (r *RfxRepository) GetEventByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxEvent, error) {
	var result *domain.RfxEvent
	err := measureDB("rfx_repository", "get_rfx_event", func() error {
		const query = `
		SELECT id, tenant_id, rfx_number, rfx_type, category, title, description,
			owner_company_id, status, currency_code, valid_from, valid_to, response_deadline,
			created_at, updated_at, version
		FROM rfx.rfx_events
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
		row := r.db().QueryRow(ctx, query, id, tenantID)
		event, err := scanRfxEvent(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.NotFound("rfx event not found")
			}
			return mapDBError(err)
		}
		result = event
		return nil
	})
	return result, err
}

func (r *RfxRepository) ListEvents(ctx context.Context, filter domain.ListRfxEventsFilter) ([]domain.RfxEvent, int, error) {
	var events []domain.RfxEvent
	var total int
	err := measureDB("rfx_repository", "list_rfx_events", func() error {
		where := strings.Builder{}
		where.WriteString(" FROM rfx.rfx_events WHERE tenant_id = $1 AND deleted_at IS NULL")
		args := []any{filter.TenantID}
		argIdx := 2

		if filter.RfxType != nil {
			where.WriteString(fmt.Sprintf(" AND rfx_type = $%d", argIdx))
			args = append(args, *filter.RfxType)
			argIdx++
		}
		if filter.Category != nil {
			where.WriteString(fmt.Sprintf(" AND category = $%d", argIdx))
			args = append(args, *filter.Category)
			argIdx++
		}
		if filter.Status != nil {
			where.WriteString(fmt.Sprintf(" AND status = $%d", argIdx))
			args = append(args, *filter.Status)
			argIdx++
		}
		if filter.OwnerCompanyID != nil {
			where.WriteString(fmt.Sprintf(" AND owner_company_id = $%d", argIdx))
			args = append(args, *filter.OwnerCompanyID)
			argIdx++
		} else if len(filter.OwnerCompanyIDs) > 0 {
			if len(filter.OwnerCompanyIDs) == 1 && filter.OwnerCompanyIDs[0] == uuid.Nil {
				where.WriteString(" AND 1=0")
			} else {
				where.WriteString(fmt.Sprintf(" AND owner_company_id = ANY($%d)", argIdx))
				args = append(args, filter.OwnerCompanyIDs)
				argIdx++
			}
		}

		countQuery := "SELECT COUNT(*)" + where.String()
		if err := r.db().QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}

		listQuery := `
		SELECT id, tenant_id, rfx_number, rfx_type, category, title, description,
			owner_company_id, status, currency_code, valid_from, valid_to, response_deadline,
			created_at, updated_at, version
	` + where.String() + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, filter.Limit, filter.Offset)

		rows, err := r.db().Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		events = make([]domain.RfxEvent, 0)
		for rows.Next() {
			event, err := scanRfxEvent(rows)
			if err != nil {
				return mapDBError(err)
			}
			events = append(events, *event)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		return nil
	})
	return events, total, err
}

func (r *RfxRepository) UpdateEvent(ctx context.Context, id, tenantID uuid.UUID, in domain.UpdateRfxEventInput) (*domain.RfxEvent, error) {
	current, err := r.GetEventByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}

	title := current.Title
	if in.Title != nil {
		title = strings.TrimSpace(*in.Title)
	}
	description := current.Description
	if in.Description != nil {
		description = in.Description
	}
	deadline := current.ResponseDeadline
	if in.ResponseDeadline != nil {
		deadline = in.ResponseDeadline
	}

	const query = `
		UPDATE rfx.rfx_events SET
			title = $3,
			description = $4,
			response_deadline = $5,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = $6 AND version = $7
		RETURNING id, tenant_id, rfx_number, rfx_type, category, title, description,
			owner_company_id, status, currency_code, valid_from, valid_to, response_deadline,
			created_at, updated_at, version
	`
	row := r.db().QueryRow(ctx, query,
		id,
		tenantID,
		title,
		optionalString(description),
		optionalDate(deadline),
		domain.RfxStatusDraft,
		current.Version,
	)
	event, err := scanRfxEvent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("rfx event was updated by another request", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return event, nil
}

func (r *RfxRepository) CountLotsByEvent(ctx context.Context, eventID, tenantID uuid.UUID) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM rfx.rfx_lots
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	var count int
	if err := r.db().QueryRow(ctx, query, eventID, tenantID).Scan(&count); err != nil {
		return 0, mapDBError(err)
	}
	return count, nil
}

func (r *RfxRepository) CloseExpiredResponses(ctx context.Context, tenantID uuid.UUID, now time.Time) (int, error) {
	var affected int
	err := measureDB("rfx_repository", "close_expired_responses", func() error {
		const query = `
		UPDATE rfx.rfx_events SET
			status = $3,
			updated_at = now(),
			version = version + 1
		WHERE tenant_id = $1 AND deleted_at IS NULL
			AND status = $4
			AND response_deadline IS NOT NULL
			AND response_deadline <= $2
	`
		tag, err := r.db().Exec(ctx, query, tenantID, now.UTC(), domain.RfxStatusResponsesClosed, domain.RfxStatusResponsesOpen)
		if err != nil {
			return mapDBError(err)
		}
		affected = int(tag.RowsAffected())
		return nil
	})
	return affected, err
}

func (r *RfxRepository) ListExpiredResponseOpenEvents(ctx context.Context, now time.Time, limit int) ([]domain.ExpiredResponseOpenEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	var result []domain.ExpiredResponseOpenEvent
	err := measureDB("rfx_repository", "list_expired_response_open_events", func() error {
		const query = `
			SELECT id, tenant_id, status, response_deadline
			FROM rfx.rfx_events
			WHERE deleted_at IS NULL
				AND status = $2
				AND response_deadline IS NOT NULL
				AND response_deadline <= $1
			ORDER BY response_deadline ASC, id ASC
			LIMIT $3
		`
		rows, err := r.db().Query(ctx, query, now.UTC(), domain.RfxStatusResponsesOpen, limit)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()
		items := make([]domain.ExpiredResponseOpenEvent, 0)
		for rows.Next() {
			var item domain.ExpiredResponseOpenEvent
			var deadline *time.Time
			if err := rows.Scan(&item.ID, &item.TenantID, &item.Status, &deadline); err != nil {
				return mapDBError(err)
			}
			item.ResponseDeadline = deadline
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		result = items
		return nil
	})
	return result, err
}

func (r *RfxRepository) UpdateEventResponseDeadline(ctx context.Context, id, tenantID uuid.UUID, deadline *time.Time, expectedVersion int) (*domain.RfxEvent, error) {
	const query = `
		UPDATE rfx.rfx_events SET
			response_deadline = $3,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND version = $4
		RETURNING id, tenant_id, rfx_number, rfx_type, category, title, description,
			owner_company_id, status, currency_code, valid_from, valid_to, response_deadline,
			created_at, updated_at, version
	`
	row := r.db().QueryRow(ctx, query, id, tenantID, optionalDate(deadline), expectedVersion)
	event, err := scanRfxEvent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("rfx event was updated by another request", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return event, nil
}

func (r *RfxRepository) UpdateEventStatus(ctx context.Context, id, tenantID uuid.UUID, expectedStatus, newStatus string) (*domain.RfxEvent, error) {
	operation := "update_rfx_event_status"
	if newStatus == domain.RfxStatusPublished {
		operation = "publish_rfx_event"
	}

	var result *domain.RfxEvent
	err := measureDB("rfx_repository", operation, func() error {
		current, err := r.GetEventByID(ctx, id, tenantID)
		if err != nil {
			return err
		}

		const query = `
		UPDATE rfx.rfx_events SET
			status = $4,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = $3 AND version = $5
		RETURNING id, tenant_id, rfx_number, rfx_type, category, title, description,
			owner_company_id, status, currency_code, valid_from, valid_to, response_deadline,
			created_at, updated_at, version
	`
		row := r.db().QueryRow(ctx, query, id, tenantID, expectedStatus, newStatus, current.Version)
		event, err := scanRfxEvent(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperrors.Conflict("rfx event was updated by another request", map[string]any{"field": "version"})
			}
			return mapDBError(err)
		}
		result = event
		return nil
	})
	return result, err
}

func (r *RfxRepository) CreateLot(ctx context.Context, in domain.CreateRfxLotInput) (*domain.RfxLot, error) {
	const query = `
		INSERT INTO rfx.rfx_lots (
			tenant_id, rfx_event_id, lot_number, name, description,
			category, estimated_value, currency_code, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, tenant_id, rfx_event_id, lot_number, name, description,
			category, estimated_value, currency_code, status
	`
	row := r.db().QueryRow(ctx, query,
		in.TenantID,
		in.RfxEventID,
		strings.TrimSpace(in.LotNumber),
		strings.TrimSpace(in.Name),
		optionalString(in.Description),
		optionalString(in.Category),
		optionalFloat(in.EstimatedValue),
		optionalString(in.CurrencyCode),
		"ACTIVE",
	)
	lot, err := scanRfxLot(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return lot, nil
}

func (r *RfxRepository) GetLotOwnerContext(ctx context.Context, lotID, tenantID uuid.UUID) (*domain.LotOwnerContext, error) {
	const query = `
		SELECT l.id, l.tenant_id, l.rfx_event_id, e.owner_company_id
		FROM rfx.rfx_lots l
		INNER JOIN rfx.rfx_events e ON e.id = l.rfx_event_id AND e.tenant_id = l.tenant_id
		WHERE l.id = $1 AND l.tenant_id = $2 AND l.deleted_at IS NULL AND e.deleted_at IS NULL
	`
	var lotCtx domain.LotOwnerContext
	if err := r.db().QueryRow(ctx, query, lotID, tenantID).Scan(
		&lotCtx.LotID,
		&lotCtx.TenantID,
		&lotCtx.RfxEventID,
		&lotCtx.OwnerCompanyID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx lot not found")
		}
		return nil, mapDBError(err)
	}
	return &lotCtx, nil
}

func (r *RfxRepository) ListLotsByEvent(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxLot, error) {
	const query = `
		SELECT id, tenant_id, rfx_event_id, lot_number, name, description,
			category, estimated_value, currency_code, status
		FROM rfx.rfx_lots
		WHERE rfx_event_id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		ORDER BY lot_number
	`
	rows, err := r.db().Query(ctx, query, eventID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	lots := make([]domain.RfxLot, 0)
	for rows.Next() {
		lot, err := scanRfxLot(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		lots = append(lots, *lot)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return lots, nil
}

func (r *RfxRepository) CreateLane(ctx context.Context, in domain.CreateRfxLaneInput) (*domain.RfxLane, error) {
	const lotExistsQuery = `
		SELECT EXISTS (
			SELECT 1 FROM rfx.rfx_lots
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`
	var lotExists bool
	if err := r.db().QueryRow(ctx, lotExistsQuery, in.RfxLotID, in.TenantID).Scan(&lotExists); err != nil {
		return nil, mapDBError(err)
	}
	if !lotExists {
		return nil, apperrors.NotFound("rfx lot not found")
	}

	const query = `
		INSERT INTO rfx.rfx_lanes (
			tenant_id, rfx_lot_id, origin_location_id, destination_location_id,
			transport_mode, equipment_type, estimated_volume, volume_unit, required_service_level
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, tenant_id, rfx_lot_id, origin_location_id, destination_location_id,
			transport_mode, equipment_type, estimated_volume, volume_unit, required_service_level
	`
	row := r.db().QueryRow(ctx, query,
		in.TenantID,
		in.RfxLotID,
		in.OriginLocationID,
		in.DestinationLocationID,
		domain.NormalizeTransportMode(in.TransportMode),
		optionalString(in.EquipmentType),
		optionalFloat(in.EstimatedVolume),
		optionalString(in.VolumeUnit),
		optionalString(in.RequiredServiceLevel),
	)
	lane, err := scanRfxLane(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return lane, nil
}

func (r *RfxRepository) AddParticipant(ctx context.Context, in domain.AddRfxParticipantInput) (*domain.RfxParticipant, error) {
	var result *domain.RfxParticipant
	err := measureDB("rfx_repository", "add_rfx_participant", func() error {
		const query = `
		INSERT INTO rfx.rfx_participants (
			tenant_id, rfx_event_id, company_id, participant_type, status, invited_at
		) VALUES ($1, $2, $3, $4, $5, now())
		RETURNING id, tenant_id, rfx_event_id, company_id, participant_type, status, invited_at
	`
		row := r.db().QueryRow(ctx, query,
			in.TenantID,
			in.RfxEventID,
			in.CompanyID,
			strings.TrimSpace(in.ParticipantType),
			domain.ParticipantStatusInvited,
		)
		participant, err := scanRfxParticipant(row)
		if err != nil {
			return mapDBError(err)
		}
		result = participant
		return nil
	})
	return result, err
}

func (r *RfxRepository) ListParticipants(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxParticipant, error) {
	var participants []domain.RfxParticipant
	err := measureDB("rfx_repository", "list_rfx_participants", func() error {
		const query = `
		SELECT id, tenant_id, rfx_event_id, company_id, participant_type, status, invited_at
		FROM rfx.rfx_participants
		WHERE rfx_event_id = $1 AND tenant_id = $2
		ORDER BY created_at
	`
		rows, err := r.db().Query(ctx, query, eventID, tenantID)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		participants = make([]domain.RfxParticipant, 0)
		for rows.Next() {
			participant, err := scanRfxParticipant(rows)
			if err != nil {
				return mapDBError(err)
			}
			participants = append(participants, *participant)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}
		return nil
	})
	return participants, err
}

func (r *RfxRepository) ParticipantExists(ctx context.Context, eventID, companyID, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM rfx.rfx_participants
			WHERE rfx_event_id = $1 AND company_id = $2 AND tenant_id = $3
		)
	`
	var exists bool
	if err := r.db().QueryRow(ctx, query, eventID, companyID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *RfxRepository) CreateResponse(ctx context.Context, in domain.CreateRfxResponseInput) (*domain.RfxResponse, error) {
	const query = `
		INSERT INTO rfx.rfx_responses (
			tenant_id, rfx_event_id, participant_company_id, status
		) VALUES ($1, $2, $3, $4)
		RETURNING id, tenant_id, rfx_event_id, participant_company_id, status,
			submitted_at, created_at, updated_at, version
	`
	row := r.db().QueryRow(ctx, query,
		in.TenantID,
		in.RfxEventID,
		in.ParticipantCompanyID,
		domain.RfxResponseStatusDraft,
	)
	response, err := scanRfxResponse(row)
	if err != nil {
		return nil, mapDBError(err)
	}
	return response, nil
}

func (r *RfxRepository) GetResponseByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxResponse, error) {
	const query = `
		SELECT id, tenant_id, rfx_event_id, participant_company_id, status,
			submitted_at, created_at, updated_at, version
		FROM rfx.rfx_responses
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
	`
	row := r.db().QueryRow(ctx, query, id, tenantID)
	response, err := scanRfxResponse(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx response not found")
		}
		return nil, mapDBError(err)
	}
	return response, nil
}

func (r *RfxRepository) SubmitResponse(ctx context.Context, id, tenantID uuid.UUID, submittedBy *uuid.UUID) (*domain.RfxResponse, error) {
	current, err := r.GetResponseByID(ctx, id, tenantID)
	if err != nil {
		return nil, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer tx.Rollback(ctx)

	const updateResponse = `
		UPDATE rfx.rfx_responses SET
			status = $3,
			submitted_at = now(),
			submitted_by = $4,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = $5 AND version = $6
		RETURNING id, tenant_id, rfx_event_id, participant_company_id, status,
			submitted_at, created_at, updated_at, version
	`
	row := tx.QueryRow(ctx, updateResponse,
		id,
		tenantID,
		domain.RfxResponseStatusSubmitted,
		optionalUUID(submittedBy),
		domain.RfxResponseStatusDraft,
		current.Version,
	)
	response, err := scanRfxResponse(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("rfx response was updated by another request", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}

	const updateParticipant = `
		UPDATE rfx.rfx_participants SET
			status = $4,
			responded_at = now()
		WHERE rfx_event_id = $1 AND company_id = $2 AND tenant_id = $3
	`
	if _, err := tx.Exec(ctx, updateParticipant,
		response.RfxEventID,
		response.ParticipantCompanyID,
		tenantID,
		domain.ParticipantStatusResponseSubmitted,
	); err != nil {
		return nil, mapDBError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, mapDBError(err)
	}
	return response, nil
}

func (r *RfxRepository) GetResponseByEventAndCompany(ctx context.Context, eventID, companyID, tenantID uuid.UUID) (*domain.RfxResponse, error) {
	const query = `
		SELECT id, tenant_id, rfx_event_id, participant_company_id, status,
			submitted_at, created_at, updated_at, version
		FROM rfx.rfx_responses
		WHERE rfx_event_id = $1 AND participant_company_id = $2 AND tenant_id = $3 AND deleted_at IS NULL
	`
	row := r.db().QueryRow(ctx, query, eventID, companyID, tenantID)
	response, err := scanRfxResponse(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("rfx response not found")
		}
		return nil, mapDBError(err)
	}
	return response, nil
}

func (r *RfxRepository) GetParticipantByEventAndCompany(ctx context.Context, eventID, companyID, tenantID uuid.UUID) (*domain.RfxParticipant, error) {
	const query = `
		SELECT id, tenant_id, rfx_event_id, company_id, participant_type, status, invited_at
		FROM rfx.rfx_participants
		WHERE rfx_event_id = $1 AND company_id = $2 AND tenant_id = $3
	`
	row := r.db().QueryRow(ctx, query, eventID, companyID, tenantID)
	participant, err := scanRfxParticipant(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("participant not found")
		}
		return nil, mapDBError(err)
	}
	return participant, nil
}

func (r *RfxRepository) ListLanesByLot(ctx context.Context, lotID, tenantID uuid.UUID) ([]domain.RfxLane, error) {
	const query = `
		SELECT id, tenant_id, rfx_lot_id, origin_location_id, destination_location_id,
			transport_mode, equipment_type, estimated_volume, volume_unit, required_service_level
		FROM rfx.rfx_lanes
		WHERE rfx_lot_id = $1 AND tenant_id = $2
		ORDER BY created_at
	`
	rows, err := r.db().Query(ctx, query, lotID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	lanes := make([]domain.RfxLane, 0)
	for rows.Next() {
		lane, err := scanRfxLane(rows)
		if err != nil {
			return nil, mapDBError(err)
		}
		lanes = append(lanes, *lane)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}
	return lanes, nil
}

func (r *RfxRepository) ListCarrierInvitedEvents(ctx context.Context, filter domain.ListCarrierInvitedEventsFilter, now time.Time) ([]domain.CarrierInvitedRfxEvent, int, error) {
	var items []domain.CarrierInvitedRfxEvent
	var total int
	err := measureDB("rfx_repository", "list_carrier_invited_events", func() error {
		where := strings.Builder{}
		where.WriteString(`
		 FROM rfx.rfx_events e
		 INNER JOIN rfx.rfx_participants p ON p.rfx_event_id = e.id AND p.tenant_id = e.tenant_id AND p.company_id = $2
		 LEFT JOIN rfx.rfx_responses resp ON resp.rfx_event_id = e.id
		   AND resp.participant_company_id = p.company_id AND resp.tenant_id = e.tenant_id AND resp.deleted_at IS NULL
		 WHERE e.tenant_id = $1 AND e.deleted_at IS NULL`)
		args := []any{filter.TenantID, filter.CarrierCompanyID}
		argIdx := 3

		if filter.Status != nil && strings.TrimSpace(*filter.Status) != "" {
			where.WriteString(fmt.Sprintf(" AND e.status = $%d", argIdx))
			args = append(args, strings.TrimSpace(*filter.Status))
			argIdx++
		}
		if filter.Search != nil && strings.TrimSpace(*filter.Search) != "" {
			where.WriteString(fmt.Sprintf(" AND (e.title ILIKE $%d OR e.rfx_number ILIKE $%d)", argIdx, argIdx))
			pattern := "%" + strings.TrimSpace(*filter.Search) + "%"
			args = append(args, pattern)
			argIdx++
		}
		if filter.ResponseFilter != nil {
			switch strings.ToUpper(strings.TrimSpace(*filter.ResponseFilter)) {
			case domain.CarrierResponseFilterOpen:
				where.WriteString(fmt.Sprintf(" AND e.status IN ('PUBLISHED', 'RESPONSES_OPEN') AND (e.response_deadline IS NULL OR e.response_deadline > $%d)", argIdx))
				args = append(args, now.UTC())
				argIdx++
			case domain.CarrierResponseFilterResponded:
				where.WriteString(" AND (resp.status = 'SUBMITTED' OR p.status = 'RESPONSE_SUBMITTED')")
			case domain.CarrierResponseFilterNotResponded:
				where.WriteString(" AND (resp.id IS NULL OR resp.status = 'DRAFT') AND NOT (resp.status = 'SUBMITTED' OR p.status = 'RESPONSE_SUBMITTED')")
			case domain.CarrierResponseFilterClosed:
				where.WriteString(fmt.Sprintf(" AND (e.status IN ('RESPONSES_CLOSED', 'CANCELLED', 'ARCHIVED') OR (e.response_deadline IS NOT NULL AND e.response_deadline <= $%d))", argIdx))
				args = append(args, now.UTC())
				argIdx++
			}
		}

		countQuery := "SELECT COUNT(*)" + where.String()
		if err := r.db().QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}

		listQuery := `
		SELECT e.id, e.tenant_id, e.rfx_number, e.rfx_type, e.category, e.title, e.description,
			e.owner_company_id, e.status, e.currency_code, e.valid_from, e.valid_to, e.response_deadline,
			e.created_at, e.updated_at, e.version,
			p.status, p.company_id, resp.id, resp.status,
			(SELECT COUNT(*) FROM rfx.rfx_lots l WHERE l.rfx_event_id = e.id AND l.tenant_id = e.tenant_id AND l.deleted_at IS NULL)
	` + where.String() + fmt.Sprintf(" ORDER BY e.response_deadline NULLS LAST, e.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, filter.Limit, filter.Offset)

		rows, err := r.db().Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		items = make([]domain.CarrierInvitedRfxEvent, 0)
		for rows.Next() {
			var item domain.CarrierInvitedRfxEvent
			var responseID *uuid.UUID
			var responseStatus *string
			if err := rows.Scan(
				&item.Event.ID, &item.Event.TenantID, &item.Event.RfxNumber, &item.Event.RfxType, &item.Event.Category, &item.Event.Title, &item.Event.Description,
				&item.Event.OwnerCompanyID, &item.Event.Status, &item.Event.CurrencyCode, &item.Event.ValidFrom, &item.Event.ValidTo, &item.Event.ResponseDeadline,
				&item.Event.CreatedAt, &item.Event.UpdatedAt, &item.Event.Version,
				&item.ParticipantStatus, &item.ParticipantCompanyID, &responseID, &responseStatus, &item.LotCount,
			); err != nil {
				return mapDBError(err)
			}
			item.OwnResponseStatus = domain.DeriveOwnResponseStatus(responseStatus)
			item.OwnResponseID = responseID
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, total, err
}

func scanRfxEvent(row pgx.Row) (*domain.RfxEvent, error) {
	var event domain.RfxEvent
	err := row.Scan(
		&event.ID,
		&event.TenantID,
		&event.RfxNumber,
		&event.RfxType,
		&event.Category,
		&event.Title,
		&event.Description,
		&event.OwnerCompanyID,
		&event.Status,
		&event.CurrencyCode,
		&event.ValidFrom,
		&event.ValidTo,
		&event.ResponseDeadline,
		&event.CreatedAt,
		&event.UpdatedAt,
		&event.Version,
	)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func scanRfxLot(row pgx.Row) (*domain.RfxLot, error) {
	var lot domain.RfxLot
	err := row.Scan(
		&lot.ID,
		&lot.TenantID,
		&lot.RfxEventID,
		&lot.LotNumber,
		&lot.Name,
		&lot.Description,
		&lot.Category,
		&lot.EstimatedValue,
		&lot.CurrencyCode,
		&lot.Status,
	)
	if err != nil {
		return nil, err
	}
	return &lot, nil
}

func scanRfxLane(row pgx.Row) (*domain.RfxLane, error) {
	var lane domain.RfxLane
	err := row.Scan(
		&lane.ID,
		&lane.TenantID,
		&lane.RfxLotID,
		&lane.OriginLocationID,
		&lane.DestinationLocationID,
		&lane.TransportMode,
		&lane.EquipmentType,
		&lane.EstimatedVolume,
		&lane.VolumeUnit,
		&lane.RequiredServiceLevel,
	)
	if err != nil {
		return nil, err
	}
	return &lane, nil
}

func scanRfxParticipant(row pgx.Row) (*domain.RfxParticipant, error) {
	var participant domain.RfxParticipant
	var invitedAt *time.Time
	err := row.Scan(
		&participant.ID,
		&participant.TenantID,
		&participant.RfxEventID,
		&participant.CompanyID,
		&participant.ParticipantType,
		&participant.Status,
		&invitedAt,
	)
	if err != nil {
		return nil, err
	}
	if invitedAt != nil {
		formatted := invitedAt.Format(time.RFC3339)
		participant.InvitedAt = &formatted
	}
	return &participant, nil
}

func scanRfxResponse(row pgx.Row) (*domain.RfxResponse, error) {
	var response domain.RfxResponse
	err := row.Scan(
		&response.ID,
		&response.TenantID,
		&response.RfxEventID,
		&response.ParticipantCompanyID,
		&response.Status,
		&response.SubmittedAt,
		&response.CreatedAt,
		&response.UpdatedAt,
		&response.Version,
	)
	if err != nil {
		return nil, err
	}
	return &response, nil
}
