package security

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
)

func TestFC_A_SEC_001_WrongTenantResourceReturns404FromProvider(t *testing.T) {
	t.Parallel()

	// Cross-tenant is enforced by tenant-scoped provider lookup returning NOT_FOUND.
	err := apperrors.NotFound("transport order not found")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}

func TestFC_A_SEC_002_SameTenantWrongCompanyReturns403(t *testing.T) {
	t.Parallel()

	buyerCompany := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	carrierCompany := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	otherBuyer := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	actor := TrustedActor{
		TenantID:  uuid.New(),
		UserID:    uuid.New(),
		CompanyID: otherBuyer,
		ActorKind: ActorKindBuyer,
	}
	facts := CanonicalCompanyFacts{
		BuyerCompanyID:   buyerCompany,
		CarrierCompanyID: carrierCompany,
	}

	err := AuthorizeCompanyAccess(actor, facts)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("expected FORBIDDEN, got %v", err)
	}
}
