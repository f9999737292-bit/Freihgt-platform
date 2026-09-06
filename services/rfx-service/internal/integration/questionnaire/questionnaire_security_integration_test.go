//go:build integration

package questionnaire

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func TestAuthorizedBuyerAllowed(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-AUTH-1")

	_, err := env.qSvc.GetStudio(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("authorized buyer denied: %v", err)
	}
}

func TestCrossTenantDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-TEN-1")

	foreignTenant, foreignActor := seedForeignTenantBuyer(t, env)
	foreignActor.TenantID = foreignTenant

	_, err := env.qSvc.GetQuestionnaire(ctx, foreignActor, event.ID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestCrossCompanyDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-CO-1")

	_, err := env.qSvc.CreateSection(ctx, fix.BuyerB, event.ID, domain.CreateSectionInput{
		SectionCode: "DENIED",
		Title:       "Denied",
	})
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestCarrierOnlyDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-CAR-1")

	_, err := env.qSvc.CreateSection(ctx, fix.CarrierAct, event.ID, domain.CreateSectionInput{
		SectionCode: "DENIED",
		Title:       "Denied",
	})
	if err == nil {
		t.Fatal("expected carrier mutation denied")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || (appErr.Code != apperrors.CodeForbidden && appErr.Code != apperrors.CodeNotFound) {
		t.Fatalf("expected forbidden/not found for carrier, got %v", err)
	}
}

func TestNoMembershipDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedBuyerFixture(t, env)
	ctx := context.Background()
	event := createDraftEvent(t, env, fix, "RFX-NOMEM-1")

	stranger := domain.ActorContext{TenantID: fix.TenantID, UserID: uuid.New()}
	_, err := env.qSvc.GetQuestionnaire(ctx, stranger, event.ID)
	if err == nil {
		t.Fatal("expected no-membership deny")
	}
}
