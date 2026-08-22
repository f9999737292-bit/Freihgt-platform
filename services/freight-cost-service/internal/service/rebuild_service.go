package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/client/billing_register"
	"github.com/freight-platform/freight-cost-service/internal/client/payment"
	"github.com/freight-platform/freight-cost-service/internal/client/transport_order"
	"github.com/freight-platform/freight-cost-service/internal/domain"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/provider"
)

type RebuildResult struct {
	FactsProcessed int
	Outcomes       []IngestResult
}

type RebuildService struct {
	ingest    *IngestService
	derived   *DerivedProjectionService
	transport *transport_order.Client
	billing   *billing_register.Client
	payment   *payment.Client
	metrics   *fcmetrics.Metrics
}

func NewRebuildService(
	ingest *IngestService,
	derived *DerivedProjectionService,
	transport *transport_order.Client,
	billing *billing_register.Client,
	payment *payment.Client,
	metrics *fcmetrics.Metrics,
) *RebuildService {
	return &RebuildService{
		ingest:    ingest,
		derived:   derived,
		transport: transport,
		billing:   billing,
		payment:   payment,
		metrics:   metrics,
	}
}

func (s *RebuildService) RebuildTransportOrder(ctx context.Context, tenantID, transportOrderID uuid.UUID) (RebuildResult, error) {
	snapshot, err := s.transport.GetRateSnapshot(ctx, tenantID, transportOrderID)
	if err != nil {
		s.observeRebuild("error")
		return RebuildResult{}, err
	}

	events := []SourceEventInput{
		plannedEventFromSnapshot(snapshot),
	}

	settlement, settlementErr := s.billing.GetSettlementByTransportOrder(ctx, tenantID, transportOrderID)
	if settlementErr == nil {
		events = append(events, settlementEvents(settlement)...)
		billingLink, linkErr := s.billing.GetBillingLink(ctx, tenantID, settlement.SettlementID)
		if linkErr == nil {
			events = append(events, billingLinkEvent(settlement, billingLink))
			if billingLink.BillingRegisterID != nil {
				register, regErr := s.billing.GetRegisterPayable(ctx, tenantID, *billingLink.BillingRegisterID)
				if regErr == nil {
					events = append(events, registerPayableEvent(settlement, register))
				}
				obligation, payErr := s.payment.GetObligationByBillingRegister(ctx, tenantID, *billingLink.BillingRegisterID)
				if payErr == nil {
					events = append(events, paidEvent(settlement, obligation))
				}
			}
		}
	} else if !isNotFoundErr(settlementErr) {
		s.observeRebuild("error")
		return RebuildResult{}, settlementErr
	}

	result := RebuildResult{}
	for _, event := range events {
		event.TenantID = tenantID
		event.TransportOrderID = transportOrderID
		event.EventOrigin = domain.EventOriginCanonicalRebuild
		revisionSemantic := domain.SourceRevisionSemantic(event.SourceType, event.SourceRevision)
		sourceFactID := domain.DeriveSourceFactID(
			tenantID, event.SourceService, event.SourceType, event.SourceID, revisionSemantic, event.EntryKind,
		)
		event.EventID = domain.DeriveRebuildDeliveryID(tenantID, sourceFactID)
		event.SourceRevisionSemantic = revisionSemantic

		ingestResult, err := s.ingest.Ingest(ctx, event)
		if err != nil {
			s.observeRebuild("error")
			return result, err
		}
		result.Outcomes = append(result.Outcomes, ingestResult)
		result.FactsProcessed++
	}

	s.observeRebuild("success")
	if settlementErr == nil && s.derived != nil {
		_ = s.derived.EnrichForecastFromSettlement(ctx, tenantID, transportOrderID, settlement)
	}
	return result, nil
}

func plannedEventFromSnapshot(snapshot *provider.RateSnapshotFact) SourceEventInput {
	amount := snapshot.TotalAmount
	return SourceEventInput{
		BuyerCompanyID:       snapshot.BuyerCompanyID,
		CarrierCompanyID:     snapshot.CarrierCompanyID,
		EntryKind:            domain.EntryKindPlannedCostSnapshot,
		SourceService:        domain.SourceServiceTransportOrder,
		SourceType:           domain.SourceTypeTORateSnapshot,
		SourceID:             snapshot.SnapshotID,
		SourceRevision:       1,
		CurrencyCode:         snapshot.CurrencyCode,
		TaxBasis:             domain.TaxBasisExVAT,
		AmountAvailability:   domain.AmountAvailabilityAvailable,
		Amount:               &amount,
		OccurredAt:           snapshot.ResolvedAt,
	}
}

func settlementEvents(settlement *billing_register.SettlementFact) []SourceEventInput {
	financial := domain.SettlementFinancialInput{
		Status:           settlement.Status,
		OpenDisputeCount: settlement.OpenDisputeCount,
		TotalWithoutVAT:  settlement.TotalWithoutVAT,
	}
	currentActual := domain.CurrentActualAmount(financial)
	finalActual := domain.FinalActualAmount(financial)

	events := make([]SourceEventInput, 0, 3)
	if settlement.AccrualAmountExVAT != nil {
		amount := *settlement.AccrualAmountExVAT
		events = append(events, baseSettlementEvent(settlement, domain.EntryKindAccrualCostSnapshot, domain.AmountAvailabilityAvailable, &amount))
	} else {
		events = append(events, baseSettlementEvent(settlement, domain.EntryKindAccrualCostSnapshot, domain.AmountAvailabilityUnavailable, nil))
	}
	events = append(events, actualSnapshotEvent(settlement, domain.EntryKindCurrentActualCostSnapshot, currentActual))
	events = append(events, actualSnapshotEvent(settlement, domain.EntryKindFinalActualCostSnapshot, finalActual))
	return events
}

func baseSettlementEvent(
	settlement *billing_register.SettlementFact,
	entryKind, availability string,
	amount *decimal.Decimal,
) SourceEventInput {
	return SourceEventInput{
		ShipmentID:         settlement.ShipmentID,
		BuyerCompanyID:     settlement.BuyerCompanyID,
		CarrierCompanyID:   settlement.CarrierCompanyID,
		EntryKind:          entryKind,
		SourceService:      domain.SourceServiceBillingRegister,
		SourceType:         domain.SourceTypeFreightSettlement,
		SourceID:           settlement.SettlementID,
		SourceRevision:     settlement.Version,
		CurrencyCode:       settlement.CurrencyCode,
		TaxBasis:           domain.TaxBasisExVAT,
		AmountAvailability: availability,
		Amount:             amount,
		OccurredAt:         settlement.UpdatedAt,
		SettlementStatus:   settlement.Status,
		OpenDisputeCount:   settlement.OpenDisputeCount,
	}
}

func actualSnapshotEvent(
	settlement *billing_register.SettlementFact,
	entryKind string,
	amount *decimal.Decimal,
) SourceEventInput {
	availability := domain.AmountAvailabilityUnavailable
	if amount != nil {
		availability = domain.AmountAvailabilityAvailable
	}
	return baseSettlementEvent(settlement, entryKind, availability, amount)
}

func billingLinkEvent(settlement *billing_register.SettlementFact, link *billing_register.BillingLinkFact) SourceEventInput {
	availability := domain.AmountAvailabilityUnavailable
	var amount *decimal.Decimal
	if link.BillingLinkState == domain.BillingLinkStateLinked && link.AmountExVAT != nil {
		availability = domain.AmountAvailabilityAvailable
		value := *link.AmountExVAT
		amount = &value
	}
	return SourceEventInput{
		ShipmentID:         settlement.ShipmentID,
		BuyerCompanyID:     settlement.BuyerCompanyID,
		CarrierCompanyID:   settlement.CarrierCompanyID,
		EntryKind:          domain.EntryKindBilledCostSnapshot,
		SourceService:      domain.SourceServiceBillingRegister,
		SourceType:         domain.SourceTypeFreightSettlementBillingLink,
		SourceID:           link.SettlementID,
		SourceRevision:     link.BillingLinkRevision,
		CurrencyCode:       link.CurrencyCode,
		TaxBasis:           domain.TaxBasisExVAT,
		AmountAvailability: availability,
		Amount:             amount,
		OccurredAt:         settlement.UpdatedAt,
	}
}

func registerPayableEvent(settlement *billing_register.SettlementFact, register *billing_register.RegisterPayableFact) SourceEventInput {
	amount := register.TotalWithVAT
	return SourceEventInput{
		ShipmentID:         settlement.ShipmentID,
		BuyerCompanyID:     settlement.BuyerCompanyID,
		CarrierCompanyID:   settlement.CarrierCompanyID,
		EntryKind:          domain.EntryKindPayableAmountSnapshot,
		SourceService:      domain.SourceServiceBillingRegister,
		SourceType:         domain.SourceTypeBillingRegister,
		SourceID:           register.BillingRegisterID,
		SourceRevision:     register.Version,
		CurrencyCode:       register.CurrencyCode,
		TaxBasis:           domain.TaxBasisWithVAT,
		AmountAvailability: domain.AmountAvailabilityAvailable,
		Amount:             &amount,
		OccurredAt:         register.UpdatedAt,
	}
}

func paidEvent(settlement *billing_register.SettlementFact, obligation *payment.ObligationFact) SourceEventInput {
	availability := domain.AmountAvailabilityUnavailable
	var amount *decimal.Decimal
	if obligation.PaidAmount != nil {
		availability = domain.AmountAvailabilityAvailable
		value := *obligation.PaidAmount
		amount = &value
	}
	return SourceEventInput{
		ShipmentID:         settlement.ShipmentID,
		BuyerCompanyID:     settlement.BuyerCompanyID,
		CarrierCompanyID:   settlement.CarrierCompanyID,
		EntryKind:          domain.EntryKindPaidAmountSnapshot,
		SourceService:      domain.SourceServicePayment,
		SourceType:         domain.SourceTypePaymentObligation,
		SourceID:           obligation.ObligationID,
		SourceRevision:     obligation.Version,
		CurrencyCode:       obligation.CurrencyCode,
		TaxBasis:           domain.TaxBasisWithVAT,
		AmountAvailability: availability,
		Amount:             amount,
		OccurredAt:         obligation.UpdatedAt,
	}
}

func (s *RebuildService) observeRebuild(result string) {
	if s.metrics != nil {
		s.metrics.ObserveRebuild(result)
	}
}
