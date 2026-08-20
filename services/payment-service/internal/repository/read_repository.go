package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/domain"
)

func (r *PaymentRepository) ListPaymentsFiltered(
	ctx context.Context,
	tenantID uuid.UUID,
	actor domain.PaymentActorInput,
	query domain.PaymentListQuery,
) (domain.PaymentListResult, error) {
	query = domain.NormalizePaymentListQuery(query)
	where, args := paymentListWhereClause(tenantID, actor, query)
	countQuery := `SELECT COUNT(*) FROM billing.payments WHERE ` + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return domain.PaymentListResult{}, mapDBError(err)
	}

	listArgs := append(append([]any{}, args...), query.Limit, query.Offset)
	listQuery := fmt.Sprintf(
		`SELECT %s FROM billing.payments WHERE %s ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`,
		paymentSelectCols, where, len(args)+1, len(args)+2,
	)
	rows, err := r.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return domain.PaymentListResult{}, mapDBError(err)
	}
	defer rows.Close()

	items := make([]domain.Payment, 0)
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return domain.PaymentListResult{}, err
		}
		items = append(items, *p)
	}
	if err := rows.Err(); err != nil {
		return domain.PaymentListResult{}, mapDBError(err)
	}
	return domain.PaymentListResult{
		Items:  items,
		Total:  total,
		Limit:  query.Limit,
		Offset: query.Offset,
	}, nil
}

func paymentListWhereClause(tenantID uuid.UUID, actor domain.PaymentActorInput, query domain.PaymentListQuery) (string, []any) {
	args := []any{tenantID}
	clauses := []string{"tenant_id = $1"}
	switch actor.ActorKind {
	case domain.PaymentActorBuyer:
		args = append(args, actor.ActorCompanyID)
		clauses = append(clauses, fmt.Sprintf("payer_company_id = $%d", len(args)))
	case domain.PaymentActorCarrier:
		args = append(args, actor.ActorCompanyID)
		clauses = append(clauses, fmt.Sprintf("payee_company_id = $%d", len(args)))
	}
	if query.Status != "" {
		args = append(args, query.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if query.CurrencyCode != "" {
		args = append(args, query.CurrencyCode)
		clauses = append(clauses, fmt.Sprintf("currency_code = $%d", len(args)))
	}
	if query.FromDate != nil {
		args = append(args, *query.FromDate)
		clauses = append(clauses, fmt.Sprintf("payment_date >= $%d", len(args)))
	}
	if query.ToDate != nil {
		args = append(args, *query.ToDate)
		clauses = append(clauses, fmt.Sprintf("payment_date <= $%d", len(args)))
	}
	if query.Search != "" {
		pattern := "%" + strings.ReplaceAll(query.Search, "%", "") + "%"
		args = append(args, pattern)
		idx := len(args)
		clauses = append(clauses, fmt.Sprintf(
			"(payment_number ILIKE $%d OR COALESCE(external_id, '') ILIKE $%d OR COALESCE(external_reference, '') ILIKE $%d OR COALESCE(reference, '') ILIKE $%d)",
			idx, idx, idx, idx,
		))
	}
	return strings.Join(clauses, " AND "), args
}

const allocationReadSelectCols = `
	a.id, a.tenant_id, a.payment_id, a.obligation_id, a.allocated_amount, a.currency_code,
	a.created_by, a.created_at, a.voided_at, a.voided_by, a.void_reason,
	o.obligation_number, o.status, o.source_type, o.source_id, o.outstanding_amount`

func (r *PaymentRepository) ListAllocationsByPaymentID(
	ctx context.Context,
	tenantID, paymentID uuid.UUID,
	limit, offset int,
) (domain.AllocationListResult, error) {
	limit, offset = domain.NormalizeListPagination(limit, offset)
	countQuery := `SELECT COUNT(*) FROM billing.payment_allocations WHERE tenant_id = $1 AND payment_id = $2`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, tenantID, paymentID).Scan(&total); err != nil {
		return domain.AllocationListResult{}, mapDBError(err)
	}

	query := `SELECT ` + allocationReadSelectCols + `
		FROM billing.payment_allocations a
		LEFT JOIN billing.payment_obligations o
			ON o.id = a.obligation_id AND o.tenant_id = a.tenant_id
		WHERE a.tenant_id = $1 AND a.payment_id = $2
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT $3 OFFSET $4`
	rows, err := r.pool.Query(ctx, query, tenantID, paymentID, limit, offset)
	if err != nil {
		return domain.AllocationListResult{}, mapDBError(err)
	}
	defer rows.Close()
	result := make([]domain.PaymentAllocationRead, 0)
	for rows.Next() {
		alloc, err := scanAllocationRead(rows)
		if err != nil {
			return domain.AllocationListResult{}, err
		}
		result = append(result, *alloc)
	}
	if err := rows.Err(); err != nil {
		return domain.AllocationListResult{}, mapDBError(err)
	}
	return domain.AllocationListResult{
		Items:  result,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func scanAllocationRead(row pgx.Row) (*domain.PaymentAllocationRead, error) {
	var read domain.PaymentAllocationRead
	var amount string
	var obligationNumber, obligationStatus, obligationSourceType *string
	var obligationSourceID *uuid.UUID
	var outstanding *string
	if err := row.Scan(
		&read.ID, &read.TenantID, &read.PaymentID, &read.ObligationID, &amount, &read.CurrencyCode,
		&read.CreatedBy, &read.CreatedAt, &read.VoidedAt, &read.VoidedBy, &read.VoidReason,
		&obligationNumber, &obligationStatus, &obligationSourceType, &obligationSourceID, &outstanding,
	); err != nil {
		return nil, mapDBError(err)
	}
	parsedAmount, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}
	read.AllocatedAmount = parsedAmount
	read.ObligationNumber = obligationNumber
	read.ObligationStatus = obligationStatus
	read.ObligationSourceType = obligationSourceType
	read.ObligationSourceID = obligationSourceID
	if outstanding != nil && *outstanding != "" {
		parsedOutstanding, parseErr := decimal.NewFromString(*outstanding)
		if parseErr != nil {
			return nil, parseErr
		}
		read.ObligationOutstandingAmount = &parsedOutstanding
	}
	return &read, nil
}

func eligibleObligationsWhere(
	tenantID uuid.UUID,
	payment *domain.Payment,
	actor domain.PaymentActorInput,
) (string, []any) {
	clauses := []string{
		"tenant_id = $1",
		"payer_company_id = $2",
		"payee_company_id = $3",
		"currency_code = $4",
		"status IN ($5, $6)",
		"outstanding_amount > 0",
	}
	args := []any{
		tenantID,
		payment.PayerCompanyID,
		payment.PayeeCompanyID,
		payment.CurrencyCode,
		domain.ObligationStatusOpen,
		domain.ObligationStatusPartiallyPaid,
	}
	switch actor.ActorKind {
	case domain.PaymentActorBuyer:
		args = append(args, actor.ActorCompanyID)
		clauses = append(clauses, fmt.Sprintf("payer_company_id = $%d", len(args)))
	case domain.PaymentActorCarrier:
		args = append(args, actor.ActorCompanyID)
		clauses = append(clauses, fmt.Sprintf("payee_company_id = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func (r *PaymentRepository) ListEligibleObligationsForPayment(
	ctx context.Context,
	tenantID uuid.UUID,
	payment *domain.Payment,
	actor domain.PaymentActorInput,
	limit, offset int,
) (domain.ObligationListResult, error) {
	limit, offset = domain.NormalizeListPagination(limit, offset)
	where, args := eligibleObligationsWhere(tenantID, payment, actor)
	countQuery := `SELECT COUNT(*) FROM billing.payment_obligations WHERE ` + where
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return domain.ObligationListResult{}, mapDBError(err)
	}

	listArgs := append(append([]any{}, args...), limit, offset)
	listQuery := fmt.Sprintf(
		`SELECT %s FROM billing.payment_obligations WHERE %s ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`,
		obligationSelectCols, where, len(args)+1, len(args)+2,
	)
	rows, err := r.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return domain.ObligationListResult{}, mapDBError(err)
	}
	defer rows.Close()
	result := make([]domain.PaymentObligation, 0)
	for rows.Next() {
		o, err := scanObligation(rows)
		if err != nil {
			return domain.ObligationListResult{}, err
		}
		result = append(result, *o)
	}
	if err := rows.Err(); err != nil {
		return domain.ObligationListResult{}, mapDBError(err)
	}
	return domain.ObligationListResult{
		Items:  result,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func paymentAuditEventsScopeSQL() string {
	return `tenant_id = $1 AND (
			(entity_type = 'PAYMENT' AND entity_id = $2)
			OR (entity_type = 'PAYMENT_ALLOCATION' AND entity_id IN (
				SELECT id FROM billing.payment_allocations WHERE tenant_id = $1 AND payment_id = $2
			))
			OR (entity_type = 'PAYMENT_OBLIGATION' AND entity_id IN (
				SELECT DISTINCT obligation_id FROM billing.payment_allocations WHERE tenant_id = $1 AND payment_id = $2
			))
		)`
}

func (r *PaymentRepository) ListPaymentAuditEvents(
	ctx context.Context,
	tenantID, paymentID uuid.UUID,
	limit, offset int,
) (domain.PaymentAuditEventListResult, error) {
	limit, offset = domain.NormalizeListPagination(limit, offset)
	countQuery := `SELECT COUNT(*) FROM billing.payment_audit_events WHERE ` + paymentAuditEventsScopeSQL()
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, tenantID, paymentID).Scan(&total); err != nil {
		return domain.PaymentAuditEventListResult{}, mapDBError(err)
	}

	query := fmt.Sprintf(`
		SELECT id, tenant_id, entity_type, entity_id, event_type, actor_user_id, actor_company_id, payload, created_at
		FROM billing.payment_audit_events
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4`, paymentAuditEventsScopeSQL())
	rows, err := r.pool.Query(ctx, query, tenantID, paymentID, limit, offset)
	if err != nil {
		return domain.PaymentAuditEventListResult{}, mapDBError(err)
	}
	defer rows.Close()
	result := make([]domain.PaymentAuditEvent, 0)
	for rows.Next() {
		event, err := scanPaymentAuditEvent(rows)
		if err != nil {
			return domain.PaymentAuditEventListResult{}, err
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return domain.PaymentAuditEventListResult{}, mapDBError(err)
	}
	return domain.PaymentAuditEventListResult{
		Items:  result,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func scanPaymentAuditEvent(row pgx.Row) (domain.PaymentAuditEvent, error) {
	var event domain.PaymentAuditEvent
	var payloadRaw []byte
	var actorUserID, actorCompanyID *uuid.UUID
	if err := row.Scan(
		&event.ID, &event.TenantID, &event.EntityType, &event.EntityID, &event.EventType,
		&actorUserID, &actorCompanyID, &payloadRaw, &event.CreatedAt,
	); err != nil {
		return domain.PaymentAuditEvent{}, mapDBError(err)
	}
	if actorUserID != nil {
		s := actorUserID.String()
		event.ActorUserID = &s
	}
	if actorCompanyID != nil {
		s := actorCompanyID.String()
		event.ActorCompanyID = &s
	}
	event.Payload = map[string]any{}
	if len(payloadRaw) > 0 {
		_ = json.Unmarshal(payloadRaw, &event.Payload)
	}
	return event, nil
}
