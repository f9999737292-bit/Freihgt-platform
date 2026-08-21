package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	"github.com/freight-platform/contract-rate-service/internal/observability"
	"github.com/freight-platform/contract-rate-service/internal/repository"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

type ResolutionService struct {
	resolutions *repository.ResolutionRepository
	memberships *repository.MembershipRepository
	rfx         domain.RFxPricingSourceProvider
	metrics     *observability.Metrics
}

func NewResolutionService(
	resolutions *repository.ResolutionRepository,
	memberships *repository.MembershipRepository,
	rfx domain.RFxPricingSourceProvider,
	metrics *observability.Metrics,
) *ResolutionService {
	return &ResolutionService{
		resolutions: resolutions,
		memberships: memberships,
		rfx:         rfx,
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

	if validated.AwardLinkID != nil || validated.AwardScopeEventID != nil {
		result, err := s.resolveAward(ctx, validated)
		if err != nil {
			s.observeFailure(start, "", reasonFromAppError(err))
			return domain.ResolveRateResult{}, err
		}
		s.observeSuccess(start, result.Status, result.PricingSource, "")
		return result, nil
	}
	if validated.BidID != nil {
		result, err := s.resolveBid(ctx, validated)
		if err != nil {
			s.observeFailure(start, "", reasonFromAppError(err))
			return domain.ResolveRateResult{}, err
		}
		s.observeSuccess(start, result.Status, result.PricingSource, "")
		return result, nil
	}
	if validated.PricingSource != nil {
		err := apperrors.Validation("explicit pricing source reference missing", map[string]any{"code": domain.ReasonInvalidPricingSource})
		s.observeFailure(start, "", domain.ReasonInvalidPricingSource)
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

	hasCompanyPermission, err := s.memberships.HasCompanyPermission(
		ctx, validated.TenantID, validated.Actor.ActorUserID, validated.Actor.ActorCompanyID, domain.PermissionManualSpotUse,
	)
	if err != nil {
		s.observeFailure(start, "", "INTERNAL")
		return domain.ResolveRateResult{}, err
	}
	tenantRoleCodes, err := s.memberships.ListUserTenantRoleCodes(ctx, validated.TenantID, validated.Actor.ActorUserID)
	if err != nil {
		s.observeFailure(start, "", "INTERNAL")
		return domain.ResolveRateResult{}, err
	}
	hasTenantPlatformAdmin := domain.HasPlatformAdminRole(tenantRoleCodes)
	if err := validated.Actor.RequireManualSpotPermission(hasCompanyPermission, hasTenantPlatformAdmin); err != nil {
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
	auditID, err := s.resolutions.RecordManualSpotAudit(ctx, validated.Actor, correlationID, metadata)
	if err != nil {
		s.observeFailure(start, "", "INTERNAL")
		return domain.ResolveRateResult{}, err
	}
	final.ManualSpotAuditID = &auditID

	s.observeSuccess(start, final.Status, final.PricingSource, "")
	return final, nil
}

func (s *ResolutionService) resolveAward(ctx context.Context, req domain.ResolveRateRequest) (domain.ResolveRateResult, error) {
	if s.rfx == nil {
		return domain.ResolveRateResult{}, apperrors.Validation("rfx pricing source unavailable", map[string]any{"code": domain.ReasonPricingSourceNotAvail})
	}
	var (
		rfxCtx domain.RFxPricingContext
		err    error
	)
	if req.AwardLinkID != nil {
		rfxCtx, err = s.rfx.GetAwardLinkPricingContext(ctx, req.TenantID, *req.AwardLinkID)
	} else {
		var lotID *uuid.UUID
		if req.AwardScopeLotID != nil && *req.AwardScopeLotID != uuid.Nil {
			lotID = req.AwardScopeLotID
		}
		rfxCtx, err = s.rfx.GetAwardScopePricingContext(ctx, req.TenantID, *req.AwardScopeEventID, lotID)
	}
	if err != nil {
		return domain.ResolveRateResult{}, err
	}
	return domain.BuildRFxResolveResult(req, rfxCtx, time.Now().UTC())
}

func (s *ResolutionService) resolveBid(ctx context.Context, req domain.ResolveRateRequest) (domain.ResolveRateResult, error) {
	if s.rfx == nil {
		return domain.ResolveRateResult{}, apperrors.Validation("rfx pricing source unavailable", map[string]any{"code": domain.ReasonPricingSourceNotAvail})
	}
	rfxCtx, err := s.rfx.GetAcceptedBidPricingContext(ctx, req.TenantID, *req.BidID)
	if err != nil {
		return domain.ResolveRateResult{}, err
	}
	return domain.BuildRFxResolveResult(req, rfxCtx, time.Now().UTC())
}

func reasonFromAppError(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) && ae.Details != nil {
		if code, ok := ae.Details["code"].(string); ok && code != "" {
			return code
		}
	}
	return reasonFromError(err)
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
	s.metrics.ObserveResolution(start, status, "", reason)
}

func reasonFromError(err error) string {
	var ae *apperrors.AppError
	if errors.As(err, &ae) && ae.Details != nil {
		if code, ok := ae.Details["code"].(string); ok && code != "" {
			return code
		}
	}
	return "VALIDATION"
}
