package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/domain"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
	"github.com/freight-platform/payment-service/internal/platform/respond"
	"github.com/freight-platform/payment-service/internal/service"
)

type PaymentHandler struct {
	payments *service.PaymentService
	actor    *PaymentActorResolver
}

func NewPaymentHandler(payments *service.PaymentService, actor *PaymentActorResolver) *PaymentHandler {
	return &PaymentHandler{payments: payments, actor: actor}
}

func (h *PaymentHandler) EnsureObligation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TenantID   string `json:"tenant_id"`
		RegisterID string `json:"register_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	tenantID, err := domain.ParseUUID(body.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	registerID, err := domain.ParseUUID(body.RegisterID, "register_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	obligation, err := h.payments.EnsurePaymentObligationForBillingRegister(r.Context(), tenantID, registerID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toObligationResponse(obligation))
}

func (h *PaymentHandler) ListObligations(w http.ResponseWriter, r *http.Request) {
	h.withActor(w, r, func(actor domain.PaymentActorInput) (any, error) {
		limit, offset := parsePagination(r)
		items, err := h.payments.ListObligations(r.Context(), actor, limit, offset)
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, len(items))
		for i := range items {
			out = append(out, toObligationResponse(&items[i]))
		}
		return map[string]any{"items": out}, nil
	})
}

func (h *PaymentHandler) GetObligation(w http.ResponseWriter, r *http.Request) {
	h.withActor(w, r, func(actor domain.PaymentActorInput) (any, error) {
		id, err := parseID(r)
		if err != nil {
			return nil, err
		}
		o, err := h.payments.GetObligation(r.Context(), id, actor)
		return toObligationResponse(o), err
	})
}

func (h *PaymentHandler) PatchDueDate(w http.ResponseWriter, r *http.Request) {
	h.withActor(w, r, func(actor domain.PaymentActorInput) (any, error) {
		id, err := parseID(r)
		if err != nil {
			return nil, err
		}
		var body struct {
			DueDate *string `json:"due_date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, domainValidation("invalid request body")
		}
		var dueDate *time.Time
		if body.DueDate != nil && *body.DueDate != "" {
			parsed, parseErr := time.Parse("2006-01-02", *body.DueDate)
			if parseErr != nil {
				return nil, domainValidation("invalid due_date")
			}
			dueDate = &parsed
		}
		o, err := h.payments.UpdateDueDate(r.Context(), id, dueDate, actor)
		return toObligationResponse(o), err
	})
}

func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body struct {
		Amount            string `json:"amount"`
		CurrencyCode      string `json:"currency_code"`
		PaymentDate       string `json:"payment_date"`
		PayerCompanyID    string `json:"payer_company_id"`
		PayeeCompanyID    string `json:"payee_company_id"`
		Reference         string `json:"reference"`
		ExternalReference string `json:"external_reference"`
		ExternalID        string `json:"external_id"`
		Source            string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	amount, err := domain.ParseMoney(body.Amount, "amount")
	if err != nil {
		respond.Error(w, err)
		return
	}
	payerID, err := domain.ParseUUID(body.PayerCompanyID, "payer_company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	payeeID, err := domain.ParseUUID(body.PayeeCompanyID, "payee_company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	paymentDate, err := time.Parse("2006-01-02", body.PaymentDate)
	if err != nil {
		respond.Error(w, domainValidation("invalid payment_date"))
		return
	}
	in := domain.CreateManualPaymentInput{
		Amount: amount, CurrencyCode: body.CurrencyCode, PaymentDate: paymentDate,
		PayerCompanyID: payerID, PayeeCompanyID: payeeID, Source: body.Source,
	}
	if body.Reference != "" {
		in.Reference = &body.Reference
	}
	if body.ExternalReference != "" {
		in.ExternalReference = &body.ExternalReference
	}
	if body.ExternalID != "" {
		in.ExternalID = &body.ExternalID
	}
	p, err := h.payments.CreateManualPayment(r.Context(), in, actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toPaymentResponse(p))
}

func (h *PaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	h.withActor(w, r, func(actor domain.PaymentActorInput) (any, error) {
		limit, offset := parsePagination(r)
		items, err := h.payments.ListPayments(r.Context(), actor, limit, offset)
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, len(items))
		for i := range items {
			out = append(out, toPaymentResponse(&items[i]))
		}
		return map[string]any{"items": out}, nil
	})
}

func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	h.withActor(w, r, func(actor domain.PaymentActorInput) (any, error) {
		id, err := parseID(r)
		if err != nil {
			return nil, err
		}
		p, err := h.payments.GetPayment(r.Context(), id, actor)
		return toPaymentResponse(p), err
	})
}

func (h *PaymentHandler) ReconcilePayment(w http.ResponseWriter, r *http.Request) {
	h.withActor(w, r, func(actor domain.PaymentActorInput) (any, error) {
		id, err := parseID(r)
		if err != nil {
			return nil, err
		}
		p, err := h.payments.ReconcilePayment(r.Context(), id, actor)
		return toPaymentResponse(p), err
	})
}

func (h *PaymentHandler) CreateAllocation(w http.ResponseWriter, r *http.Request) {
	h.withActor(w, r, func(actor domain.PaymentActorInput) (any, error) {
		paymentID, err := parseID(r)
		if err != nil {
			return nil, err
		}
		var body struct {
			ObligationID    string `json:"obligation_id"`
			AllocatedAmount string `json:"allocated_amount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, domainValidation("invalid request body")
		}
		obligationID, err := domain.ParseUUID(body.ObligationID, "obligation_id")
		if err != nil {
			return nil, err
		}
		amount, err := domain.ParseMoney(body.AllocatedAmount, "allocated_amount")
		if err != nil {
			return nil, err
		}
		result, err := h.payments.Allocate(r.Context(), domain.CreateAllocationInput{
			PaymentID: paymentID, ObligationID: obligationID, AllocatedAmount: amount,
		}, actor)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"payment": toPaymentResponse(result.Payment),
			"obligation": toObligationResponse(result.Obligation),
			"allocation": map[string]any{
				"id": result.Allocation.ID.String(),
				"allocated_amount": result.Allocation.AllocatedAmount.StringFixed(domain.MoneyScale),
			},
		}, nil
	})
}

func (h *PaymentHandler) withActor(w http.ResponseWriter, r *http.Request, fn func(domain.PaymentActorInput) (any, error)) {
	actor, err := h.actor.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	payload, err := fn(actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, payload)
}

func parseID(r *http.Request) (uuid.UUID, error) {
	return domain.ParseUUID(chi.URLParam(r, "id"), "id")
}

func parsePagination(r *http.Request) (int, int) {
	limit := 20
	offset := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	return limit, offset
}

func toObligationResponse(o *domain.PaymentObligation) map[string]any {
	resp := map[string]any{
		"id": o.ID.String(), "tenant_id": o.TenantID.String(),
		"obligation_number": o.ObligationNumber,
		"payer_company_id": o.PayerCompanyID.String(), "payee_company_id": o.PayeeCompanyID.String(),
		"source_type": o.SourceType, "source_id": o.SourceID.String(),
		"currency_code": o.CurrencyCode,
		"original_amount": o.OriginalAmount.StringFixed(domain.MoneyScale),
		"paid_amount": o.PaidAmount.StringFixed(domain.MoneyScale),
		"outstanding_amount": o.OutstandingAmount.StringFixed(domain.MoneyScale),
		"status": o.Status, "version": o.Version,
		"created_at": o.CreatedAt, "updated_at": o.UpdatedAt,
	}
	if o.DueDate != nil {
		resp["due_date"] = o.DueDate.Format("2006-01-02")
	}
	return resp
}

func toPaymentResponse(p *domain.Payment) map[string]any {
	return map[string]any{
		"id": p.ID.String(), "tenant_id": p.TenantID.String(),
		"payment_number": p.PaymentNumber,
		"payer_company_id": p.PayerCompanyID.String(), "payee_company_id": p.PayeeCompanyID.String(),
		"amount": p.Amount.StringFixed(domain.MoneyScale),
		"currency_code": p.CurrencyCode, "payment_date": p.PaymentDate.Format("2006-01-02"),
		"source": p.Source, "status": p.Status,
		"allocated_amount": p.AllocatedAmount.StringFixed(domain.MoneyScale),
		"unallocated_amount": p.UnallocatedAmount.StringFixed(domain.MoneyScale),
		"version": p.Version, "created_at": p.CreatedAt, "updated_at": p.UpdatedAt,
	}
}

func domainValidation(message string) error {
	return apperrors.Validation(message, nil)
}
