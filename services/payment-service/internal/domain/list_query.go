package domain

import (
	"strings"
	"time"

	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

const MaxPaymentListLimit = 100

type PaymentListQuery struct {
	Status       string
	CurrencyCode string
	FromDate     *time.Time
	ToDate       *time.Time
	Search       string
	Limit        int
	Offset       int
}

type PaymentListResult struct {
	Items  []Payment
	Total  int
	Limit  int
	Offset int
}

type AllocationListResult struct {
	Items  []PaymentAllocationRead
	Total  int
	Limit  int
	Offset int
}

type ObligationListResult struct {
	Items  []PaymentObligation
	Total  int
	Limit  int
	Offset int
}

type PaymentAuditEventListResult struct {
	Items  []PaymentAuditEvent
	Total  int
	Limit  int
	Offset int
}

func NormalizeListPagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > MaxPaymentListLimit {
		limit = MaxPaymentListLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

type PaymentAuditEvent struct {
	ID             string
	TenantID       string
	EntityType     string
	EntityID       string
	EventType      string
	ActorUserID    *string
	ActorCompanyID *string
	Payload        map[string]any
	CreatedAt      time.Time
}

func NormalizePaymentListQuery(q PaymentListQuery) PaymentListQuery {
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Limit > MaxPaymentListLimit {
		q.Limit = MaxPaymentListLimit
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	q.Status = strings.TrimSpace(strings.ToUpper(q.Status))
	q.CurrencyCode = NormalizeCurrencyCode(strings.TrimSpace(q.CurrencyCode))
	q.Search = strings.TrimSpace(q.Search)
	return q
}

func ValidatePaymentListQuery(q PaymentListQuery) error {
	q = NormalizePaymentListQuery(q)
	if q.Status != "" && !isValidPaymentStatus(q.Status) {
		return apperrors.Validation("invalid status filter", map[string]any{"field": "status"})
	}
	if q.CurrencyCode != "" {
		if err := ValidateCurrencyCode(q.CurrencyCode); err != nil {
			return apperrors.Validation("invalid currency_code filter", map[string]any{"field": "currency_code"})
		}
	}
	if q.FromDate != nil && q.ToDate != nil && q.FromDate.After(*q.ToDate) {
		return apperrors.Validation("from_date must be before to_date", map[string]any{"field": "from_date"})
	}
	return nil
}

func isValidPaymentStatus(status string) bool {
	switch status {
	case PaymentStatusReceived,
		PaymentStatusPartiallyAllocated,
		PaymentStatusFullyAllocated,
		PaymentStatusReconciled,
		PaymentStatusVoided:
		return true
	default:
		return false
	}
}
