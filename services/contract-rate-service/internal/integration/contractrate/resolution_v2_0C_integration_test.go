//go:build integration

package contractrate

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
	"github.com/freight-platform/contract-rate-service/internal/service"
)

type stubRFxProvider struct {
	awardLinkContexts  map[uuid.UUID]domain.RFxPricingContext
	awardScopeContexts map[uuid.UUID]domain.RFxPricingContext
	bidContexts        map[uuid.UUID]domain.RFxPricingContext

	awardLinkErrors  map[uuid.UUID]error
	awardScopeErrors map[uuid.UUID]error
	bidErrors        map[uuid.UUID]error
}

func newStubRFxProvider() *stubRFxProvider {
	return &stubRFxProvider{
		awardLinkContexts:  make(map[uuid.UUID]domain.RFxPricingContext),
		awardScopeContexts: make(map[uuid.UUID]domain.RFxPricingContext),
		bidContexts:        make(map[uuid.UUID]domain.RFxPricingContext),
		awardLinkErrors:    make(map[uuid.UUID]error),
		awardScopeErrors:   make(map[uuid.UUID]error),
		bidErrors:          make(map[uuid.UUID]error),
	}
}

func (s *stubRFxProvider) SetAwardLinkContext(linkID uuid.UUID, ctx domain.RFxPricingContext) {
	s.awardLinkContexts[linkID] = ctx
}

func (s *stubRFxProvider) SetAwardScopeContext(eventID uuid.UUID, ctx domain.RFxPricingContext) {
	s.awardScopeContexts[eventID] = ctx
}

func (s *stubRFxProvider) SetBidContext(bidID uuid.UUID, ctx domain.RFxPricingContext) {
	s.bidContexts[bidID] = ctx
}

func (s *stubRFxProvider) SetAwardLinkError(linkID uuid.UUID, err error) {
	s.awardLinkErrors[linkID] = err
}

func (s *stubRFxProvider) SetAwardScopeError(eventID uuid.UUID, err error) {
	s.awardScopeErrors[eventID] = err
}

func (s *stubRFxProvider) SetBidError(bidID uuid.UUID, err error) {
	s.bidErrors[bidID] = err
}

func (s *stubRFxProvider) GetAwardLinkPricingContext(_ context.Context, _ uuid.UUID, linkID uuid.UUID) (domain.RFxPricingContext, error) {
	if err, ok := s.awardLinkErrors[linkID]; ok {
		return domain.RFxPricingContext{}, err
	}
	if ctx, ok := s.awardLinkContexts[linkID]; ok {
		return ctx, nil
	}
	return domain.RFxPricingContext{}, apperrors.NotFound("pricing source not found")
}

func (s *stubRFxProvider) GetAwardScopePricingContext(_ context.Context, _ uuid.UUID, eventID uuid.UUID, _ *uuid.UUID) (domain.RFxPricingContext, error) {
	if err, ok := s.awardScopeErrors[eventID]; ok {
		return domain.RFxPricingContext{}, err
	}
	if ctx, ok := s.awardScopeContexts[eventID]; ok {
		return ctx, nil
	}
	return domain.RFxPricingContext{}, apperrors.NotFound("pricing source not found")
}

func (s *stubRFxProvider) GetAcceptedBidPricingContext(_ context.Context, _ uuid.UUID, bidID uuid.UUID) (domain.RFxPricingContext, error) {
	if err, ok := s.bidErrors[bidID]; ok {
		return domain.RFxPricingContext{}, err
	}
	if ctx, ok := s.bidContexts[bidID]; ok {
		return ctx, nil
	}
	return domain.RFxPricingContext{}, apperrors.NotFound("pricing source not found")
}

func newResolutionSvc(env *testEnv, stub domain.RFxPricingSourceProvider) *service.ResolutionService {
	return service.NewResolutionService(env.Resolutions, env.Memberships, stub, nil)
}

func matchingRFxContext(env *testEnv, sourceType string, sourceID uuid.UUID, totalAmount string) domain.RFxPricingContext {
	return domain.RFxPricingContext{
		TenantID:                 env.TenantID,
		SourceType:               sourceType,
		SourceID:                 sourceID,
		BuyerCompanyID:           env.BuyerID,
		CarrierCompanyID:         env.CarrierID,
		OriginLocationID:         env.OriginID,
		DestinationLocationID:    env.DestID,
		EquipmentType:            "TAUTLINER",
		TransportMode:            domain.TransportModeRoad,
		CurrencyCode:             "RUB",
		TotalAmount:              totalAmount,
		ComponentBreakdownStatus: "UNAVAILABLE",
	}
}

func expectPricingSourceMismatch(t *testing.T, err error, field string) {
	t.Helper()
	if !isAppErrorCode(err, apperrors.CodeValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected app error, got %v", err)
	}
	if ae.Details["code"] != domain.ReasonPricingSourceMismatch {
		t.Fatalf("expected %s, got %v", domain.ReasonPricingSourceMismatch, ae.Details["code"])
	}
	if ae.Details["field"] != field {
		t.Fatalf("expected field %s, got %v", field, ae.Details["field"])
	}
}

func TestCRes001AwardBeatsContract(t *testing.T) {
	env := setupEnv(t)
	setupActiveRate(t, env, "CR-C-001", "TAUTLINER", "1000.00")

	eventID := uuid.New()
	stub := newStubRFxProvider()
	awardCtx := matchingRFxContext(env, domain.PricingSourceRFQAward, eventID, "7500.00")
	awardCtx.RfxEventID = &eventID
	stub.SetAwardScopeContext(eventID, awardCtx)

	svc := newResolutionSvc(env, stub)
	req := env.resolveReq("TAUTLINER")
	req.AwardScopeEventID = &eventID

	result, err := svc.Resolve(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Status != domain.ResolveStatusMatched || result.PricingSource != domain.PricingSourceRFQAward {
		t.Fatalf("expected RFQ_AWARD match, got status=%s source=%s", result.Status, result.PricingSource)
	}
	if result.TotalAmount == nil || *result.TotalAmount != "7500.00" {
		t.Fatalf("expected award total 7500.00, got %v", result.TotalAmount)
	}
}

func TestCRes002BidBeatsContract(t *testing.T) {
	env := setupEnv(t)
	setupActiveRate(t, env, "CR-C-002", "TAUTLINER", "1000.00")

	bidID := uuid.New()
	stub := newStubRFxProvider()
	bidCtx := matchingRFxContext(env, domain.PricingSourceSpotBid, bidID, "6200.00")
	bidCtx.BidID = &bidID
	stub.SetBidContext(bidID, bidCtx)

	svc := newResolutionSvc(env, stub)
	req := env.resolveReq("TAUTLINER")
	req.BidID = &bidID

	result, err := svc.Resolve(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Status != domain.ResolveStatusMatched || result.PricingSource != domain.PricingSourceSpotBid {
		t.Fatalf("expected SPOT_BID match, got status=%s source=%s", result.Status, result.PricingSource)
	}
	if result.TotalAmount == nil || *result.TotalAmount != "6200.00" {
		t.Fatalf("expected bid total 6200.00, got %v", result.TotalAmount)
	}
}

func TestCRes003InvalidAwardDoesNotFallThrough(t *testing.T) {
	env := setupEnv(t)
	setupActiveRate(t, env, "CR-C-003", "TAUTLINER", "1000.00")

	eventID := uuid.New()
	stub := newStubRFxProvider()
	stub.SetAwardScopeError(eventID, apperrors.NotFound("pricing source not found"))

	svc := newResolutionSvc(env, stub)
	req := env.resolveReq("TAUTLINER")
	req.AwardScopeEventID = &eventID

	result, err := svc.Resolve(context.Background(), req, nil)
	if err == nil {
		t.Fatalf("expected error, got contract-like match status=%s source=%s", result.Status, result.PricingSource)
	}
	if !isAppErrorCode(err, apperrors.CodeNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCRes004InvalidBidDoesNotFallThrough(t *testing.T) {
	env := setupEnv(t)
	setupActiveRate(t, env, "CR-C-004", "TAUTLINER", "1000.00")

	bidID := uuid.New()
	stub := newStubRFxProvider()
	stub.SetBidError(bidID, apperrors.NotFound("pricing source not found"))

	svc := newResolutionSvc(env, stub)
	req := env.resolveReq("TAUTLINER")
	req.BidID = &bidID

	result, err := svc.Resolve(context.Background(), req, nil)
	if err == nil {
		t.Fatalf("expected error, got contract-like match status=%s source=%s", result.Status, result.PricingSource)
	}
	if !isAppErrorCode(err, apperrors.CodeNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCRes005BuyerMismatchDeny(t *testing.T) {
	env := setupEnv(t)
	eventID := uuid.New()
	stub := newStubRFxProvider()
	awardCtx := matchingRFxContext(env, domain.PricingSourceRFQAward, eventID, "7500.00")
	awardCtx.BuyerCompanyID = uuid.New()
	stub.SetAwardScopeContext(eventID, awardCtx)

	svc := newResolutionSvc(env, stub)
	req := env.resolveReq("TAUTLINER")
	req.AwardScopeEventID = &eventID

	_, err := svc.Resolve(context.Background(), req, nil)
	expectPricingSourceMismatch(t, err, "buyer_company_id")
}

func TestCRes006CarrierMismatchDeny(t *testing.T) {
	env := setupEnv(t)
	bidID := uuid.New()
	stub := newStubRFxProvider()
	bidCtx := matchingRFxContext(env, domain.PricingSourceSpotBid, bidID, "6200.00")
	bidCtx.CarrierCompanyID = uuid.New()
	stub.SetBidContext(bidID, bidCtx)

	svc := newResolutionSvc(env, stub)
	req := env.resolveReq("TAUTLINER")
	req.BidID = &bidID

	_, err := svc.Resolve(context.Background(), req, nil)
	expectPricingSourceMismatch(t, err, "carrier_company_id")
}

func TestCRes007LaneMismatchDeny(t *testing.T) {
	env := setupEnv(t)
	eventID := uuid.New()
	stub := newStubRFxProvider()
	awardCtx := matchingRFxContext(env, domain.PricingSourceRFQAward, eventID, "7500.00")
	awardCtx.DestinationLocationID = uuid.New()
	stub.SetAwardScopeContext(eventID, awardCtx)

	svc := newResolutionSvc(env, stub)
	req := env.resolveReq("TAUTLINER")
	req.AwardScopeEventID = &eventID

	_, err := svc.Resolve(context.Background(), req, nil)
	expectPricingSourceMismatch(t, err, "destination_location_id")
}

func TestCRes008EquipmentCaseMismatchDeny(t *testing.T) {
	env := setupEnv(t)
	bidID := uuid.New()
	stub := newStubRFxProvider()
	bidCtx := matchingRFxContext(env, domain.PricingSourceSpotBid, bidID, "6200.00")
	bidCtx.EquipmentType = "BOX"
	stub.SetBidContext(bidID, bidCtx)

	svc := newResolutionSvc(env, stub)
	req := env.resolveReq("Box")
	req.BidID = &bidID

	_, err := svc.Resolve(context.Background(), req, nil)
	expectPricingSourceMismatch(t, err, "equipment_type")
}

func TestCRes009ContractRegression(t *testing.T) {
	env := setupEnv(t)
	setupActiveRate(t, env, "CR-C-009", "TAUTLINER", "1000.00")

	stub := newStubRFxProvider()
	svc := newResolutionSvc(env, stub)

	result, err := svc.Resolve(context.Background(), env.resolveReq("TAUTLINER"), nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Status != domain.ResolveStatusMatched || result.PricingSource != domain.PricingSourceContractRate {
		t.Fatalf("expected contract match, got status=%s source=%s", result.Status, result.PricingSource)
	}
	if result.TotalAmount == nil || *result.TotalAmount != "1000.00" {
		t.Fatalf("expected contract total 1000.00, got %v", result.TotalAmount)
	}
}

func TestCRes010RateAmbiguousRegression(t *testing.T) {
	env := setupEnv(t)
	for i, number := range []string{"CR-C-010-A", "CR-C-010-B"} {
		contract := env.createActiveContract(t, number)
		card, _ := env.RateCards.Create(context.Background(), domain.CreateRateCardInput{
			TenantID: env.TenantID, ContractID: contract.ID, Name: "Card", Actor: env.Actor,
		}, nil)
		version, _ := env.RateCards.CreateDraftVersion(context.Background(), domain.CreateRateVersionInput{
			TenantID: env.TenantID, RateCardID: card.ID,
			ValidFrom: env.Today.AddDate(0, 0, -i), Actor: env.Actor,
		}, nil)
		line := env.createRateLine(t, version.ID, "TAUTLINER")
		env.addBaseFreight(t, line.ID, "1000.00")
		env.activateVersion(t, version.ID)
	}

	stub := newStubRFxProvider()
	svc := newResolutionSvc(env, stub)

	result, err := svc.Resolve(context.Background(), env.resolveReq("TAUTLINER"), nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if result.Status != domain.ResolveStatusAmbiguous {
		t.Fatalf("expected AMBIGUOUS, got %s", result.Status)
	}
}
