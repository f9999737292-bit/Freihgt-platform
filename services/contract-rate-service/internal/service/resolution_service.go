package service

import (
	"context"
	"time"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	"github.com/freight-platform/contract-rate-service/internal/observability"
	"github.com/freight-platform/contract-rate-service/internal/repository"
)

type ResolutionService struct {
	resolutions *repository.ResolutionRepository
	memberships *repository.MembershipRepository
	metrics     *observability.Metrics
}

func NewResolutionService(
	resolutions *repository.ResolutionRepository,
	memberships *repository.MembershipRepository,
	metrics *observability.Metrics,
) *ResolutionService {
	return &ResolutionService{
		resolutions: resolutions,
		memberships: memberships,
		metrics:     metrics,
	}
}

func (s *ResolutionService) Resolve(ctx context.Context, req domain.ResolveRateRequest, correlationID *string) (domain.ResolveRateResult, error) {
	start := time.Now()
	validated, err := domain.ValidateResolveRateRequest(req)
	if err != nil {
		s.observeFailure(start, "", reasonFromError(err))
		return domain.ResolveRateResult{}, err
	}
	if err := validated.Actor.CanReadContract(validated.BuyerCompanyID, validated.CarrierCompanyID); err != nil {
		s.observeFailure(start, "", domain.ReasonManualSpotForbidden)
		return domain.ResolveRateResult{}, err
	}

	candidates, err := s.resolutions.FindCandidates(ctx, validated)
	if err != nil {
		s.observeFailure(start, "", "INTERNAL")
		return domain.ResolveRateResult{}, err
	}

	result := domain.ResolveRateCandidates(validated, candidates)
	if result.Status == domain.ResolveStatusMatched || result.Status == domain.ResolveStatusAmbiguous {
		s.observeSuccess(start, result.Status, result.PricingSource, "")
		return result, nil
	}

	if validated.ManualSpotAmount == nil {
		reason := domain.ReasonRateNotFound
		if result.ReasonCode != nil {
			reason = *result.ReasonCode
		}
		result.ReasonCode = &reason
		s.observeSuccess(start, domain.ResolveStatusNoMatch, "", reason)
		return result, nil
	}

	roleCodes, err := s.memberships.ListUserGlobalRoleCodes(ctx, validated.TenantID, validated.Actor.ActorUserID)
	if err != nil {
		s.observeFailure(start, "", "INTERNAL")
		return domain.ResolveRateResult{}, err
	}
	if err := validated.Actor.RequireManualSpotPrice(roleCodes); err != nil {
		s.observeFailure(start, "", domain.ReasonManualSpotForbidden)
		return domain.ResolveRateResult{}, err
	}

	final, err := domain.ApplyManualSpotFallback(validated, result, true)
	if err != nil {
		s.observeFailure(start, "", domain.ReasonManualSpotForbidden)
		return domain.ResolveRateResult{}, err
	}

	metadata := map[string]any{
		"buyer_company_id":        validated.BuyerCompanyID.String(),
		"carrier_company_id":      validated.CarrierCompanyID.String(),
		"origin_location_id":      validated.OriginLocationID.String(),
		"destination_location_id": validated.DestinationLocationID.String(),
		"equipment_type":          validated.EquipmentType,
		"transport_mode":          validated.TransportMode,
		"pricing_date":            validated.PricingDate.Format("2006-01-02"),
	}
	if final.TotalAmount != nil {
		metadata["total_amount"] = *final.TotalAmount
	}
	if final.CurrencyCode != nil {
		metadata["currency_code"] = *final.CurrencyCode
	}
	if _, err := s.resolutions.RecordManualSpotAudit(ctx, validated.Actor, correlationID, metadata); err != nil {
		s.observeFailure(start, "", "INTERNAL")
		return domain.ResolveRateResult{}, err
	}

	s.observeSuccess(start, final.Status, final.PricingSource, "")
	return final, nil
}

func (s *ResolutionService) observeSuccess(start time.Time, status, sourceType, reason string) {
	if s.metrics == nil {
		return
	}
	s.metrics.ObserveResolution(start, status, sourceType, reason)
}

func (s *ResolutionService) observeFailure(start time.Time, status, reason string) {
	if s.metrics == nil {
		return
	}
	if status == "" {
		status = "FAILED"
	}
	s.metrics.ObserveResolution(start, status, "", reason)
}

func reasonFromError(err error) string {
	if err == nil {
		return "UNKNOWN"
	}
	return "VALIDATION"
}
