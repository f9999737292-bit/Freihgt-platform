package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/payment-service/internal/domain"
	"github.com/freight-platform/payment-service/internal/repository"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
)

type precisionGuardStore struct{}

func (precisionGuardStore) EnsureObligationForBillingRegister(context.Context, uuid.UUID, uuid.UUID, *repository.BillingRegisterSnapshot) (*domain.PaymentObligation, error) {
	return nil, nil
}
func (precisionGuardStore) GetObligationByID(context.Context, uuid.UUID, uuid.UUID) (*domain.PaymentObligation, error) {
	return nil, nil
}
func (precisionGuardStore) GetObligationBySource(context.Context, uuid.UUID, string, uuid.UUID) (*domain.PaymentObligation, error) {
	return nil, nil
}
func (precisionGuardStore) UpdateObligationDueDate(context.Context, uuid.UUID, uuid.UUID, *time.Time, domain.PaymentActorInput) (*domain.PaymentObligation, error) {
	return nil, nil
}
func (precisionGuardStore) CreateManualPayment(context.Context, domain.CreateManualPaymentInput) (*domain.Payment, error) {
	return &domain.Payment{}, nil
}
func (precisionGuardStore) GetPaymentByID(context.Context, uuid.UUID, uuid.UUID) (*domain.Payment, error) {
	return nil, nil
}
func (precisionGuardStore) ReconcilePayment(context.Context, uuid.UUID, uuid.UUID, domain.PaymentActorInput) (*domain.Payment, error) {
	return nil, nil
}
func (precisionGuardStore) Allocate(context.Context, domain.CreateAllocationInput) (*repository.AllocateResult, error) {
	return &repository.AllocateResult{}, nil
}
func (precisionGuardStore) ListObligations(context.Context, uuid.UUID, domain.PaymentActorInput, int, int) ([]domain.PaymentObligation, error) {
	return nil, nil
}
func (precisionGuardStore) ListPayments(context.Context, uuid.UUID, domain.PaymentActorInput, int, int) ([]domain.Payment, error) {
	return nil, nil
}

type alwaysMemberStore struct{}

func (alwaysMemberStore) CompanyExistsInTenant(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func testPaymentActor() domain.PaymentActorInput {
	companyID := uuid.New()
	return domain.PaymentActorInput{
		TenantID: uuid.New(), ActorCompanyID: companyID,
		ActorKind: domain.PaymentActorBuyer, ActorUserID: uuid.New(),
	}
}

func TestCreateManualPaymentRejectsOverPrecision(t *testing.T) {
	t.Parallel()
	svc := NewPaymentService(precisionGuardStore{}, nil, alwaysMemberStore{}, nil)
	actor := testPaymentActor()
	_, err := svc.CreateManualPayment(context.Background(), domain.CreateManualPaymentInput{
		Amount: decimal.RequireFromString("1.234"), CurrencyCode: "RUB", PaymentDate: time.Now().UTC(),
		PayerCompanyID: actor.ActorCompanyID, PayeeCompanyID: uuid.New(), Source: domain.PaymentSourceManual,
	}, actor)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("SERVICE_PAYMENT_OVER_PRECISION_REJECT expected validation, got %v", err)
	}
}

func TestCreateManualPaymentAllowsTrailingZeroPrecision(t *testing.T) {
	t.Parallel()
	svc := NewPaymentService(precisionGuardStore{}, nil, alwaysMemberStore{}, nil)
	actor := testPaymentActor()
	_, err := svc.CreateManualPayment(context.Background(), domain.CreateManualPaymentInput{
		Amount: decimal.RequireFromString("1.230"), CurrencyCode: "RUB", PaymentDate: time.Now().UTC(),
		PayerCompanyID: actor.ActorCompanyID, PayeeCompanyID: uuid.New(), Source: domain.PaymentSourceManual,
	}, actor)
	if err != nil {
		t.Fatalf("TRAILING_ZERO_PRECISION_ALLOWED expected pass at service boundary, got %v", err)
	}
}

func TestAllocateRejectsOverPrecision(t *testing.T) {
	t.Parallel()
	svc := NewPaymentService(precisionGuardStore{}, nil, alwaysMemberStore{}, nil)
	_, err := svc.Allocate(context.Background(), domain.CreateAllocationInput{
		PaymentID: uuid.New(), ObligationID: uuid.New(),
		AllocatedAmount: decimal.RequireFromString("1.234"),
	}, testPaymentActor())
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("SERVICE_ALLOCATION_OVER_PRECISION_REJECT expected validation, got %v", err)
	}
}
