package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	apperrors "github.com/freight-platform/freight-cost-service/internal/platform/errors"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/repository"
)

const (
	IngestOutcomeApplied           = "applied"
	IngestOutcomeNoOpEvent         = "no_op_event"
	IngestOutcomeNoOpFact          = "no_op_fact"
	IngestOutcomeJournaledOutOfOrder = "journaled_out_of_order"
)

type SourceEventInput struct {
	EventID              uuid.UUID
	EventType            string
	SchemaVersion        int
	TenantID             uuid.UUID
	TransportOrderID     uuid.UUID
	ShipmentID           *uuid.UUID
	BuyerCompanyID       uuid.UUID
	CarrierCompanyID     uuid.UUID
	EntryKind            string
	SourceService        string
	SourceType           string
	SourceID             uuid.UUID
	SourceRevision       int64
	SourceRevisionSemantic string
	CurrencyCode         string
	TaxBasis             domain.TaxBasis
	AmountAvailability   string
	Amount               *decimal.Decimal
	OccurredAt           time.Time
	EventOrigin          string
	SettlementStatus     string
	OpenDisputeCount     int
	Metadata             map[string]any
}

type IngestResult struct {
	Outcome     string
	CostEntryID *uuid.UUID
}

type IngestService struct {
	pool         *pgxpool.Pool
	entries      *repository.CostEntryRepository
	cursors      *repository.SourceCursorRepository
	projections  *repository.CostSummaryProjectionRepository
	derived      *DerivedProjectionService
	analytics    *AnalyticsProjectionService
	metrics      *fcmetrics.Metrics
}

func NewIngestService(
	pool *pgxpool.Pool,
	entries *repository.CostEntryRepository,
	cursors *repository.SourceCursorRepository,
	projections *repository.CostSummaryProjectionRepository,
	derived *DerivedProjectionService,
	analytics *AnalyticsProjectionService,
	metrics *fcmetrics.Metrics,
) *IngestService {
	return &IngestService{
		pool:        pool,
		entries:     entries,
		cursors:     cursors,
		projections: projections,
		derived:     derived,
		analytics:   analytics,
		metrics:     metrics,
	}
}

func (s *IngestService) Ingest(ctx context.Context, input SourceEventInput) (IngestResult, error) {
	if err := validateSourceEventInput(input); err != nil {
		return IngestResult{}, err
	}

	revisionSemantic := input.SourceRevisionSemantic
	if revisionSemantic == "" {
		revisionSemantic = domain.SourceRevisionSemantic(input.SourceType, input.SourceRevision)
	}
	sourceFactID := domain.DeriveSourceFactID(
		input.TenantID, input.SourceService, input.SourceType, input.SourceID, revisionSemantic, input.EntryKind,
	)

	candidate := inputToCostEntry(input, sourceFactID)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return IngestResult{}, apperrors.Internal("begin transaction", err)
	}
	defer tx.Rollback(ctx)

	if existing, err := s.entries.FindBySourceEventID(ctx, tx, input.TenantID, input.EventID); err == nil {
		if domain.CostEntriesSemanticallyEqual(existing, candidate) {
			s.observeIngest(input.EntryKind, IngestOutcomeNoOpEvent)
			if err := tx.Commit(ctx); err != nil {
				return IngestResult{}, apperrors.Internal("commit transaction", err)
			}
			return IngestResult{Outcome: IngestOutcomeNoOpEvent, CostEntryID: &existing.ID}, nil
		}
		return IngestResult{}, apperrors.Integrity("source_event_id replay with conflicting payload", map[string]any{
			"source_event_id": input.EventID.String(),
		})
	} else if !isNotFoundErr(err) {
		return IngestResult{}, err
	}

	if existing, err := s.entries.FindBySourceFactID(ctx, tx, input.TenantID, sourceFactID); err == nil {
		if domain.CostEntriesSemanticallyEqual(existing, candidate) {
			s.observeIngest(input.EntryKind, IngestOutcomeNoOpFact)
			if err := tx.Commit(ctx); err != nil {
				return IngestResult{}, apperrors.Internal("commit transaction", err)
			}
			return IngestResult{Outcome: IngestOutcomeNoOpFact, CostEntryID: &existing.ID}, nil
		}
		return IngestResult{}, apperrors.Integrity("source_fact_id conflict with different financial snapshot", map[string]any{
			"source_fact_id": sourceFactID.String(),
		})
	} else if !isNotFoundErr(err) {
		return IngestResult{}, err
	}

	if prior, err := s.entries.FindLatestByDimension(ctx, tx, input.TenantID, input.TransportOrderID,
		input.SourceService, input.SourceType, input.SourceID, input.EntryKind); err == nil {
		candidate.SupersedesEntryID = &prior.ID
	} else if !isNotFoundErr(err) {
		return IngestResult{}, err
	}

	entryID, err := s.entries.Insert(ctx, tx, candidate)
	if err != nil {
		return IngestResult{}, err
	}
	candidate.ID = entryID

	cursorKey := domain.SourceCursorKey{
		TenantID:         input.TenantID,
		TransportOrderID: input.TransportOrderID,
		SourceService:    input.SourceService,
		SourceType:       input.SourceType,
		SourceID:         input.SourceID,
		EntryKind:        input.EntryKind,
	}
	cursor, err := s.cursors.GetOrZero(ctx, tx, cursorKey)
	if err != nil {
		return IngestResult{}, err
	}

	outcome := IngestOutcomeJournaledOutOfOrder
		if input.SourceRevision > cursor.LastSourceRevision {
		projection, err := s.projections.GetOrInit(ctx, tx, input.TenantID, input.TransportOrderID, input.BuyerCompanyID, input.CarrierCompanyID)
		if err != nil {
			return IngestResult{}, err
		}
		if isSettlementDimension(input.EntryKind) {
			if input.SettlementStatus != "" {
				projection.SettlementStatus = input.SettlementStatus
			}
			projection.OpenDisputeCount = input.OpenDisputeCount
		}
		if err := domain.ApplyCostEntryToProjection(projection, candidate); err != nil {
			if s.metrics != nil {
				s.metrics.ObserveCurrencyMismatch("ingest")
			}
			return IngestResult{}, apperrors.Validation("currency mismatch during projection update", map[string]any{
				"field": "currency_code",
			})
		}
		if s.derived != nil {
			if err := s.derived.RecomputeInTransaction(ctx, tx, projection, domain.ProposedAccessorialInput{
				SourceStatus: domain.ProposedSourceUnknown,
			}, domain.DriverAttributionContext{}); err != nil {
				return IngestResult{}, err
			}
		}
		if err := s.projections.Upsert(ctx, tx, projection); err != nil {
			return IngestResult{}, err
		}

		eventID := input.EventID
		cursor.LastSourceRevision = input.SourceRevision
		cursor.LastSourceEventID = &eventID
		cursor.LastCostEntryID = &entryID
		if err := s.cursors.Upsert(ctx, tx, cursor); err != nil {
			return IngestResult{}, err
		}
		if s.analytics != nil && projection.CurrencyCode != "" {
			summaryUpdatedAt := time.Now().UTC()
			if err := s.analytics.MarkCostSummaryChanged(ctx, tx, AnalyticsChangeInput{
				TenantID:         input.TenantID,
				TransportOrderID: input.TransportOrderID,
				BuyerCompanyID:   projection.BuyerCompanyID,
				CurrencyCode:     projection.CurrencyCode,
				SummaryUpdatedAt: summaryUpdatedAt,
				SourceEventID:    input.EventID,
			}); err != nil {
				return IngestResult{}, err
			}
		}
		outcome = IngestOutcomeApplied
		s.observeProjectionUpdate(input.EntryKind)
	} else {
		s.observeOutOfOrder(input.EntryKind)
	}

	s.observeIngest(input.EntryKind, outcome)
	if err := tx.Commit(ctx); err != nil {
		return IngestResult{}, apperrors.Internal("commit transaction", err)
	}
	if outcome == IngestOutcomeApplied && s.derived != nil {
		_ = s.derived.EnrichForecastFromBilling(ctx, input.TenantID, input.TransportOrderID)
	}
	return IngestResult{Outcome: outcome, CostEntryID: &entryID}, nil
}

func validateSourceEventInput(input SourceEventInput) error {
	if input.EventID == uuid.Nil || input.TenantID == uuid.Nil || input.TransportOrderID == uuid.Nil {
		return apperrors.Validation("missing required identity fields", map[string]any{"field": "event_id"})
	}
	if input.BuyerCompanyID == uuid.Nil || input.CarrierCompanyID == uuid.Nil || input.SourceID == uuid.Nil {
		return apperrors.Validation("missing required aggregate identity", map[string]any{"field": "source_id"})
	}
	if strings.TrimSpace(input.EntryKind) == "" || strings.TrimSpace(input.SourceService) == "" || strings.TrimSpace(input.SourceType) == "" {
		return apperrors.Validation("missing source metadata", map[string]any{"field": "entry_kind"})
	}
	if err := domain.ValidateCurrencyCode(input.CurrencyCode); err != nil {
		return apperrors.Validation("invalid currency code", map[string]any{"field": "currency_code"})
	}
	if input.AmountAvailability != domain.AmountAvailabilityAvailable && input.AmountAvailability != domain.AmountAvailabilityUnavailable {
		return apperrors.Validation("invalid amount_availability", map[string]any{"field": "amount_availability"})
	}
	if input.AmountAvailability == domain.AmountAvailabilityAvailable {
		if input.Amount == nil {
			return apperrors.Validation("amount required when available", map[string]any{"field": "amount"})
		}
		if input.Amount.IsNegative() {
			return apperrors.Validation("amount must be non-negative", map[string]any{"field": "amount"})
		}
	}
	if input.AmountAvailability == domain.AmountAvailabilityUnavailable && input.Amount != nil {
		return apperrors.Validation("amount must be null when unavailable", map[string]any{"field": "amount"})
	}
	if input.EventOrigin != domain.EventOriginLiveOutbox && input.EventOrigin != domain.EventOriginCanonicalRebuild {
		return apperrors.Validation("invalid event_origin", map[string]any{"field": "event_origin"})
	}
	if input.TaxBasis != domain.TaxBasisExVAT && input.TaxBasis != domain.TaxBasisWithVAT {
		return apperrors.Validation("invalid tax_basis", map[string]any{"field": "tax_basis"})
	}
	if input.OccurredAt.IsZero() {
		return apperrors.Validation("occurred_at is required", map[string]any{"field": "occurred_at"})
	}
	return nil
}

func inputToCostEntry(input SourceEventInput, sourceFactID uuid.UUID) *domain.CostEntry {
	var amount *decimal.Decimal
	if input.Amount != nil {
		value := input.Amount.Round(domain.MoneyScale)
		amount = &value
	}
	return &domain.CostEntry{
		TenantID:           input.TenantID,
		TransportOrderID:   input.TransportOrderID,
		ShipmentID:         input.ShipmentID,
		BuyerCompanyID:     input.BuyerCompanyID,
		CarrierCompanyID:   input.CarrierCompanyID,
		EntryKind:          input.EntryKind,
		Amount:             amount,
		CurrencyCode:       strings.ToUpper(strings.TrimSpace(input.CurrencyCode)),
		TaxBasis:           input.TaxBasis,
		AmountAvailability: input.AmountAvailability,
		SourceService:      input.SourceService,
		SourceType:         input.SourceType,
		SourceID:           input.SourceID,
		SourceRevision:     input.SourceRevision,
		SourceFactID:       sourceFactID,
		SourceEventID:      input.EventID,
		SourceOccurredAt:   input.OccurredAt.UTC(),
		EventOrigin:        input.EventOrigin,
		Metadata:           input.Metadata,
	}
}

func isNotFoundErr(err error) bool {
	var appErr *apperrors.AppError
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound
}

func (s *IngestService) observeIngest(entryKind, outcome string) {
	if s.metrics != nil {
		s.metrics.ObserveEventApplied(entryKind, outcome)
	}
}

func (s *IngestService) observeProjectionUpdate(entryKind string) {
	if s.metrics != nil {
		s.metrics.ObserveProjectionUpdate(entryKind)
	}
}

func (s *IngestService) observeOutOfOrder(entryKind string) {
	if s.metrics != nil {
		s.metrics.ObserveOutOfOrder(entryKind)
	}
}

func isSettlementDimension(entryKind string) bool {
	switch entryKind {
	case domain.EntryKindAccrualCostSnapshot,
		domain.EntryKindCurrentActualCostSnapshot,
		domain.EntryKindFinalActualCostSnapshot:
		return true
	default:
		return false
	}
}
