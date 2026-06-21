package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
	"github.com/freight-platform/billing-register-service/internal/repository"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

type mockRegisterStore struct {
	createFn                 func(ctx context.Context, in domain.CreateBillingRegisterInput) (*domain.BillingRegister, error)
	getByIDAndTenantFn       func(ctx context.Context, id, tenantID uuid.UUID) (*domain.BillingRegister, error)
	addItemFn                func(ctx context.Context, registerID uuid.UUID, amounts domain.ItemAmounts, in domain.CreateBillingRegisterItemInput) (*domain.BillingRegisterItem, error)
	calculateFn              func(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.BillingRegister, error)
	approveFn                func(ctx context.Context, id, tenantID, approvedBy uuid.UUID, expectedVersion int) (*domain.BillingRegister, error)
	updateStatusFn           func(ctx context.Context, id, tenantID uuid.UUID, status string, expectedVersion int) (*domain.BillingRegister, error)
	getShipmentStatusFn      func(ctx context.Context, shipmentID, tenantID uuid.UUID) (string, error)
}

func (m *mockRegisterStore) CompanyExists(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func (m *mockRegisterStore) GetShipmentStatus(ctx context.Context, shipmentID, tenantID uuid.UUID) (string, error) {
	return m.getShipmentStatusFn(ctx, shipmentID, tenantID)
}
func (m *mockRegisterStore) Create(ctx context.Context, in domain.CreateBillingRegisterInput) (*domain.BillingRegister, error) {
	return m.createFn(ctx, in)
}
func (m *mockRegisterStore) GetDetail(context.Context, uuid.UUID) (*repository.RegisterDetail, error) {
	return nil, nil
}
func (m *mockRegisterStore) List(context.Context, domain.ListBillingRegistersFilter) ([]domain.BillingRegister, int, error) {
	return nil, 0, nil
}
func (m *mockRegisterStore) GetByIDAndTenant(ctx context.Context, id, tenantID uuid.UUID) (*domain.BillingRegister, error) {
	return m.getByIDAndTenantFn(ctx, id, tenantID)
}
func (m *mockRegisterStore) AddItem(ctx context.Context, registerID uuid.UUID, amounts domain.ItemAmounts, in domain.CreateBillingRegisterItemInput) (*domain.BillingRegisterItem, error) {
	return m.addItemFn(ctx, registerID, amounts, in)
}
func (m *mockRegisterStore) ListItems(context.Context, uuid.UUID, uuid.UUID) ([]domain.BillingRegisterItem, error) {
	return nil, nil
}
func (m *mockRegisterStore) DeleteItem(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (m *mockRegisterStore) Calculate(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.BillingRegister, error) {
	return m.calculateFn(ctx, id, tenantID, expectedVersion)
}
func (m *mockRegisterStore) RecalculateAfterItemChange(context.Context, uuid.UUID, uuid.UUID, int) (*domain.BillingRegister, error) {
	return nil, nil
}
func (m *mockRegisterStore) Approve(ctx context.Context, id, tenantID, approvedBy uuid.UUID, expectedVersion int) (*domain.BillingRegister, error) {
	return m.approveFn(ctx, id, tenantID, approvedBy, expectedVersion)
}
func (m *mockRegisterStore) UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, status string, expectedVersion int) (*domain.BillingRegister, error) {
	return m.updateStatusFn(ctx, id, tenantID, status, expectedVersion)
}

type mockClosingStore struct {
	createInvoiceFn   func(ctx context.Context, register *domain.BillingRegister, in domain.CreateInvoiceInput) (*domain.Invoice, error)
	createActFn       func(ctx context.Context, register *domain.BillingRegister, in domain.CreateActInput) (*domain.Act, error)
	createVATInvoiceFn func(ctx context.Context, register *domain.BillingRegister, in domain.CreateVATInvoiceInput) (*domain.VATInvoice, error)
	createUPDFn       func(ctx context.Context, register *domain.BillingRegister, in domain.CreateUPDInput) (*domain.UPDDocument, error)
}

func (m *mockClosingStore) CreatePackage(context.Context, uuid.UUID, domain.CreateClosingDocumentPackageInput) (*domain.ClosingDocumentPackage, error) {
	return nil, nil
}
func (m *mockClosingStore) CreateInvoice(ctx context.Context, register *domain.BillingRegister, in domain.CreateInvoiceInput) (*domain.Invoice, error) {
	return m.createInvoiceFn(ctx, register, in)
}
func (m *mockClosingStore) CreateAct(ctx context.Context, register *domain.BillingRegister, in domain.CreateActInput) (*domain.Act, error) {
	return m.createActFn(ctx, register, in)
}
func (m *mockClosingStore) CreateVATInvoice(ctx context.Context, register *domain.BillingRegister, in domain.CreateVATInvoiceInput) (*domain.VATInvoice, error) {
	return m.createVATInvoiceFn(ctx, register, in)
}
func (m *mockClosingStore) CreateUPD(ctx context.Context, register *domain.BillingRegister, in domain.CreateUPDInput) (*domain.UPDDocument, error) {
	return m.createUPDFn(ctx, register, in)
}

func TestBillingRegisterServiceCreateValidation(t *testing.T) {
	t.Parallel()
	svc := NewBillingRegisterService(&mockRegisterStore{})
	_, err := svc.Create(context.Background(), domain.CreateBillingRegisterInput{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestBillingRegisterServiceAddItemCalculation(t *testing.T) {
	t.Parallel()
	svc := NewBillingRegisterService(&mockRegisterStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BillingRegister, error) {
			vat := 20.0
			return &domain.BillingRegister{Status: domain.RegisterStatusDraft, VATRate: &vat}, nil
		},
		getShipmentStatusFn: func(context.Context, uuid.UUID, uuid.UUID) (string, error) {
			return domain.ShipmentStatusReadyForBilling, nil
		},
		addItemFn: func(_ context.Context, _ uuid.UUID, amounts domain.ItemAmounts, _ domain.CreateBillingRegisterItemInput) (*domain.BillingRegisterItem, error) {
			if amounts.AmountWithVAT != 126000 {
				t.Fatalf("expected calculated amount 126000, got %v", amounts.AmountWithVAT)
			}
			return &domain.BillingRegisterItem{AmountWithVAT: amounts.AmountWithVAT, Status: domain.RegisterItemStatusDraft}, nil
		},
	})
	item, err := svc.AddItem(context.Background(), uuid.New(), domain.CreateBillingRegisterItemInput{
		TenantID: uuid.New(), ShipmentID: uuid.New(), BaseAmount: 100000, ExtraCharges: 5000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.AmountWithVAT != 126000 {
		t.Fatalf("unexpected amount")
	}
}

func TestBillingRegisterServiceCalculateTotals(t *testing.T) {
	t.Parallel()
	svc := NewBillingRegisterService(&mockRegisterStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BillingRegister, error) {
			return &domain.BillingRegister{Status: domain.RegisterStatusDraft, Version: 1}, nil
		},
		calculateFn: func(context.Context, uuid.UUID, uuid.UUID, int) (*domain.BillingRegister, error) {
			return &domain.BillingRegister{Status: domain.RegisterStatusCalculated, TotalWithVAT: 126000}, nil
		},
	})
	reg, err := svc.Calculate(context.Background(), uuid.New(), domain.TenantActionInput{TenantID: uuid.New()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.Status != domain.RegisterStatusCalculated {
		t.Fatalf("unexpected status")
	}
}

func TestBillingRegisterServiceApproveOnlyCalculated(t *testing.T) {
	t.Parallel()
	svc := NewBillingRegisterService(&mockRegisterStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BillingRegister, error) {
			return &domain.BillingRegister{Status: domain.RegisterStatusDraft}, nil
		},
	})
	_, err := svc.Approve(context.Background(), uuid.New(), domain.ApproveRegisterInput{TenantID: uuid.New(), ApprovedBy: uuid.New()})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestBillingRegisterServiceCannotAddItemToApproved(t *testing.T) {
	t.Parallel()
	svc := NewBillingRegisterService(&mockRegisterStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BillingRegister, error) {
			return &domain.BillingRegister{Status: domain.RegisterStatusApproved}, nil
		},
	})
	_, err := svc.AddItem(context.Background(), uuid.New(), domain.CreateBillingRegisterItemInput{
		TenantID: uuid.New(), ShipmentID: uuid.New(), BaseAmount: 100,
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestClosingDocumentServiceCreateInvoice(t *testing.T) {
	t.Parallel()
	svc := NewClosingDocumentService(&mockRegisterStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BillingRegister, error) {
			return &domain.BillingRegister{Status: domain.RegisterStatusApproved, TotalWithVAT: 126000, CurrencyCode: "RUB"}, nil
		},
	}, &mockClosingStore{
		createInvoiceFn: func(_ context.Context, reg *domain.BillingRegister, _ domain.CreateInvoiceInput) (*domain.Invoice, error) {
			return &domain.Invoice{TotalAmount: reg.TotalWithVAT, Status: domain.InvoiceStatusDraft}, nil
		},
	})
	inv, err := svc.CreateInvoice(context.Background(), uuid.New(), domain.CreateInvoiceInput{
		TenantID: uuid.New(), InvoiceNumber: "INV-1", InvoiceDate: time.Now(),
		SellerCompanyID: uuid.New(), BuyerCompanyID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.TotalAmount != 126000 {
		t.Fatalf("unexpected total")
	}
}

func TestClosingDocumentServiceCreateAct(t *testing.T) {
	t.Parallel()
	svc := NewClosingDocumentService(&mockRegisterStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BillingRegister, error) {
			return &domain.BillingRegister{Status: domain.RegisterStatusApproved, TotalWithVAT: 126000}, nil
		},
	}, &mockClosingStore{
		createActFn: func(_ context.Context, reg *domain.BillingRegister, _ domain.CreateActInput) (*domain.Act, error) {
			return &domain.Act{TotalAmount: reg.TotalWithVAT}, nil
		},
	})
	act, err := svc.CreateAct(context.Background(), uuid.New(), domain.CreateActInput{
		TenantID: uuid.New(), ActNumber: "ACT-1", ActDate: time.Now(),
		SellerCompanyID: uuid.New(), BuyerCompanyID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if act.TotalAmount != 126000 {
		t.Fatalf("unexpected total")
	}
}

func TestClosingDocumentServiceCreateVATInvoice(t *testing.T) {
	t.Parallel()
	svc := NewClosingDocumentService(&mockRegisterStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BillingRegister, error) {
			return &domain.BillingRegister{Status: domain.RegisterStatusApproved, TotalWithVAT: 126000, TotalWithoutVAT: 105000, VATAmount: 21000}, nil
		},
	}, &mockClosingStore{
		createVATInvoiceFn: func(_ context.Context, reg *domain.BillingRegister, _ domain.CreateVATInvoiceInput) (*domain.VATInvoice, error) {
			return &domain.VATInvoice{AmountWithVAT: reg.TotalWithVAT}, nil
		},
	})
	inv, err := svc.CreateVATInvoice(context.Background(), uuid.New(), domain.CreateVATInvoiceInput{
		TenantID: uuid.New(), VATInvoiceNumber: "SF-1", VATInvoiceDate: time.Now(),
		SellerCompanyID: uuid.New(), BuyerCompanyID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.AmountWithVAT != 126000 {
		t.Fatalf("unexpected total")
	}
}

func TestClosingDocumentServiceCreateUPD(t *testing.T) {
	t.Parallel()
	svc := NewClosingDocumentService(&mockRegisterStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BillingRegister, error) {
			return &domain.BillingRegister{Status: domain.RegisterStatusApproved, TotalWithVAT: 126000}, nil
		},
	}, &mockClosingStore{
		createUPDFn: func(_ context.Context, reg *domain.BillingRegister, _ domain.CreateUPDInput) (*domain.UPDDocument, error) {
			return &domain.UPDDocument{AmountWithVAT: reg.TotalWithVAT, Status: domain.UPDStatusDraft}, nil
		},
	})
	upd, err := svc.CreateUPD(context.Background(), uuid.New(), domain.CreateUPDInput{
		TenantID: uuid.New(), UPDNumber: "UPD-1", UPDDate: time.Now(), FunctionCode: "СЧФДОП",
		SellerCompanyID: uuid.New(), BuyerCompanyID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upd.AmountWithVAT != 126000 {
		t.Fatalf("unexpected total")
	}
}

func TestBillingRegisterServiceMarkPaidOnlyAfterSigned(t *testing.T) {
	t.Parallel()
	svc := NewBillingRegisterService(&mockRegisterStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BillingRegister, error) {
			return &domain.BillingRegister{Status: domain.RegisterStatusApproved, Version: 1}, nil
		},
	})
	_, err := svc.MarkPaid(context.Background(), uuid.New(), domain.TenantActionInput{TenantID: uuid.New()})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestBillingRegisterServiceCloseOnlyAfterPaid(t *testing.T) {
	t.Parallel()
	svc := NewBillingRegisterService(&mockRegisterStore{
		getByIDAndTenantFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.BillingRegister, error) {
			return &domain.BillingRegister{Status: domain.RegisterStatusApproved, Version: 1}, nil
		},
	})
	_, err := svc.Close(context.Background(), uuid.New(), domain.TenantActionInput{TenantID: uuid.New()})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}
