//go:build integration

package pricing

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

func TestCRFX001ValidAwardContext(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event, lotID := seedAwardedEvent(t, env, fix, 75000)

	repo := repository.NewPricingRepository(env.pool)
	out, err := repo.GetAwardScopeContext(ctx, fix.tenantID, event.ID, &lotID)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if out.SourceType != domain.PricingSourceRFQAward {
		t.Fatalf("expected source type %q, got %q", domain.PricingSourceRFQAward, out.SourceType)
	}
	if out.TenantID != fix.tenantID {
		t.Fatalf("unexpected tenant id")
	}
	if out.BuyerCompanyID != fix.buyerCompany || out.CarrierCompanyID != fix.carrierCompany {
		t.Fatalf("unexpected buyer/carrier companies")
	}
	if out.OriginLocationID != fix.originID || out.DestinationLocationID != fix.destID {
		t.Fatalf("unexpected origin/destination")
	}
	if out.EquipmentType != "TAUTLINER" {
		t.Fatalf("expected equipment TAUTLINER, got %q", out.EquipmentType)
	}
	if out.CurrencyCode != "RUB" {
		t.Fatalf("expected currency RUB, got %q", out.CurrencyCode)
	}
	if out.TotalAmount != "75000.00" {
		t.Fatalf("expected total 75000.00, got %q", out.TotalAmount)
	}
	if out.ComponentBreakdownStatus != domain.ComponentBreakdownUnavailable {
		t.Fatalf("expected aggregate-only breakdown status")
	}
	if out.RfxEventID == nil || *out.RfxEventID != event.ID {
		t.Fatalf("expected rfx event id on context")
	}
	if out.RfxLotID == nil || *out.RfxLotID != lotID {
		t.Fatalf("expected rfx lot id on context")
	}
}

func TestCRFX002AwardWrongTenantDenied(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event, lotID := seedAwardedEvent(t, env, fix, 1000)

	repo := repository.NewPricingRepository(env.pool)
	_, err := repo.GetAwardScopeContext(ctx, uuid.New(), event.ID, &lotID)
	if err == nil {
		t.Fatal("expected not found for wrong tenant")
	}
}

func TestCRFX003AwardExactDecimalString(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event, lotID := seedAwardedEvent(t, env, fix, 108000.50)

	repo := repository.NewPricingRepository(env.pool)
	out, err := repo.GetAwardScopeContext(ctx, fix.tenantID, event.ID, &lotID)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if out.TotalAmount != "108000.50" {
		t.Fatalf("expected exact decimal string 108000.50, got %q", out.TotalAmount)
	}
}

func TestCRFX004AwardAggregateOnly(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event, lotID := seedAwardedEvent(t, env, fix, 95000)

	repo := repository.NewPricingRepository(env.pool)
	out, err := repo.GetAwardScopeContext(ctx, fix.tenantID, event.ID, &lotID)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if out.BaseAmount != nil || len(out.Components) != 0 || out.ComponentBreakdownStatus != domain.ComponentBreakdownUnavailable {
		t.Fatalf("expected aggregate-only normalization")
	}
}

func TestCRFX005AcceptedBidValid(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	bidID := seedBidForPricing(t, env, fix, bidPricingSeed{})

	repo := repository.NewPricingRepository(env.pool)
	out, err := repo.GetAcceptedBidContext(ctx, fix.tenantID, bidID)
	if err != nil {
		t.Fatalf("bid context: %v", err)
	}
	if out.SourceType != domain.PricingSourceSpotBid {
		t.Fatalf("expected source type %q, got %q", domain.PricingSourceSpotBid, out.SourceType)
	}
	if out.BidID == nil || *out.BidID != bidID {
		t.Fatalf("expected bid id on context")
	}
	if out.BuyerCompanyID != fix.buyerCompany || out.CarrierCompanyID != fix.carrierCompany {
		t.Fatalf("unexpected buyer/carrier companies")
	}
	if out.OriginLocationID != fix.originID || out.DestinationLocationID != fix.destID {
		t.Fatalf("unexpected origin/destination")
	}
	if out.EquipmentType != "TAUTLINER" || out.CurrencyCode != "RUB" {
		t.Fatalf("unexpected equipment or currency")
	}
	if out.TotalAmount != "100.00" {
		t.Fatalf("expected total 100.00, got %q", out.TotalAmount)
	}
	if out.ComponentBreakdownStatus != domain.ComponentBreakdownUnavailable {
		t.Fatalf("expected aggregate-only breakdown status")
	}
}

func TestCRFX006NonAcceptedBidDeny(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	bidID := seedBidForPricing(t, env, fix, bidPricingSeed{status: domain.BidStatusSubmitted})

	repo := repository.NewPricingRepository(env.pool)
	_, err := repo.GetAcceptedBidContext(ctx, fix.tenantID, bidID)
	assertAppErrorCode(t, err, apperrors.CodeValidation)
	assertAppErrorDetailCode(t, err, "INVALID_PRICING_SOURCE")
}

func TestCRFX007PreVATTTotalAuthoritative(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	bidID := seedBidForPricing(t, env, fix, bidPricingSeed{
		totalAmount:        "100.00",
		totalAmountWithVAT: "120.00",
	})

	repo := repository.NewPricingRepository(env.pool)
	out, err := repo.GetAcceptedBidContext(ctx, fix.tenantID, bidID)
	if err != nil {
		t.Fatalf("bid context: %v", err)
	}
	if out.TotalAmount != "100.00" {
		t.Fatalf("expected pre-VAT total 100.00, got %q", out.TotalAmount)
	}
}

func TestCRFX008BidWrongTenantDeny(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	bidID := seedBidForPricing(t, env, fix, bidPricingSeed{})

	repo := repository.NewPricingRepository(env.pool)
	_, err := repo.GetAcceptedBidContext(ctx, uuid.New(), bidID)
	if err == nil {
		t.Fatal("expected not found for wrong tenant")
	}
}

func TestCRFX009NoFloatRoundTrip(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	bidID := seedBidForPricing(t, env, fix, bidPricingSeed{
		totalAmount:        "108000.51",
		totalAmountWithVAT: "129600.61",
	})

	repo := repository.NewPricingRepository(env.pool)
	out, err := repo.GetAcceptedBidContext(ctx, fix.tenantID, bidID)
	if err != nil {
		t.Fatalf("bid context: %v", err)
	}
	if out.TotalAmount != "108000.51" {
		t.Fatalf("expected exact decimal string 108000.51, got %q", out.TotalAmount)
	}
}

func TestCRFX010MissingSourceContextFailClosed(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event, lotID := seedAwardedEvent(t, env, fix, 5000)
	clearAwardScopeLocations(t, env, lotID)

	repo := repository.NewPricingRepository(env.pool)
	_, err := repo.GetAwardScopeContext(ctx, fix.tenantID, event.ID, &lotID)
	assertAppErrorCode(t, err, apperrors.CodeValidation)
	assertAppErrorDetailCode(t, err, "MISSING_PRICING_CONTEXT")
}

func TestCRFX011MissingCurrencyFailClosed(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event, lotID := seedAwardedEvent(t, env, fix, 5000)
	clearAwardScopeCurrency(t, env, event.ID)

	repo := repository.NewPricingRepository(env.pool)
	_, err := repo.GetAwardScopeContext(ctx, fix.tenantID, event.ID, &lotID)
	assertAppErrorCode(t, err, apperrors.CodeValidation)
	assertAppErrorDetailCode(t, err, "MISSING_PRICING_CONTEXT")
}

func TestCRFX012MissingEquipmentFailClosed(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event, lotID := seedAwardedEvent(t, env, fix, 5000)
	clearAwardScopeEquipment(t, env, lotID)

	repo := repository.NewPricingRepository(env.pool)
	_, err := repo.GetAwardScopeContext(ctx, fix.tenantID, event.ID, &lotID)
	assertAppErrorCode(t, err, apperrors.CodeValidation)
	assertAppErrorDetailCode(t, err, "MISSING_PRICING_CONTEXT")
}

func TestCRFX013AwardScopeMultipleLanesWithoutLotAmbiguous(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event := seedAwardedMultiLotEvent(t, env, fix, []float64{10000, 20000})

	repo := repository.NewPricingRepository(env.pool)
	_, err := repo.GetAwardScopeContext(ctx, fix.tenantID, event.ID, nil)
	assertAppErrorCode(t, err, apperrors.CodeValidation)
	assertAppErrorDetailCode(t, err, "PRICING_SOURCE_AMBIGUOUS")
}

func TestCRFX014EquipmentCaseMismatchPreserved(t *testing.T) {
	env := setupEnv(t)
	fix := seedFixture(t, env)
	ctx := context.Background()
	event, lotID := seedAwardedEventWithEquipment(t, env, fix, 42000, "Box")

	repo := repository.NewPricingRepository(env.pool)
	out, err := repo.GetAwardScopeContext(ctx, fix.tenantID, event.ID, &lotID)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if out.EquipmentType != "Box" {
		t.Fatalf("expected equipment preserved as Box, got %q", out.EquipmentType)
	}
}
