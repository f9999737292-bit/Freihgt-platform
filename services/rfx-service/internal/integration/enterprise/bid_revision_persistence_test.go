//go:build integration

package enterprise

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

func TestBidRevisionPersistence_ResponseRevision(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "rev-resp")

	respRepo := repository.NewResponseRevisionRepository(env.pool)
	svc := service.NewResponseBidService(respRepo)
	carrier := fix.Carriers[0]

	idem := "rev-idem-" + uuid.NewString()
	rev, err := svc.SubmitRevision(ctx, domain.SubmitResponseRevisionInput{
		TenantID:             fix.TenantID,
		RfxEventID:           fix.EventID,
		RfxResponseID:        carrier.ResponseID,
		ParticipantCompanyID: carrier.CompanyID,
		PriceAmount:          95,
		CurrencyCode:         "RUB",
		CapacityUnits:        450,
		TransitHours:         22,
		SLAScoreInput:        92,
		CarrierKPIInput:      86,
		ReliabilityInput:     82,
		IdempotencyKey:       &idem,
	})
	if err != nil {
		t.Fatalf("submit revision: %v", err)
	}
	if rev.RevisionNumber != 2 {
		t.Fatalf("expected revision 2, got %d", rev.RevisionNumber)
	}

	dup, err := svc.SubmitRevision(ctx, domain.SubmitResponseRevisionInput{
		TenantID:             fix.TenantID,
		RfxEventID:           fix.EventID,
		RfxResponseID:        carrier.ResponseID,
		ParticipantCompanyID: carrier.CompanyID,
		PriceAmount:          95,
		CurrencyCode:         "RUB",
		CapacityUnits:        450,
		TransitHours:         22,
		SLAScoreInput:        92,
		CarrierKPIInput:      86,
		ReliabilityInput:     82,
		IdempotencyKey:       &idem,
	})
	if err != nil {
		t.Fatalf("idempotent submit: %v", err)
	}
	if dup.ID != rev.ID {
		t.Fatalf("idempotent revision id mismatch")
	}

	active, err := svc.GetActiveRevision(ctx, carrier.ResponseID, fix.TenantID, nil)
	if err != nil {
		t.Fatalf("get active revision: %v", err)
	}
	if active.RevisionNumber != 2 || active.PriceAmount == nil || *active.PriceAmount != 95 {
		t.Fatalf("active revision not updated: %+v", active)
	}

	history, err := svc.ListRevisions(ctx, carrier.ResponseID, fix.TenantID, nil)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(history) < 2 {
		t.Fatalf("expected at least 2 revisions, got %d", len(history))
	}
}

func TestBidRevisionPersistence_FreightRequestBid(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "rev-bid")
	carrierID := fix.Carriers[1].CompanyID
	_, bidID := createFreightRequestBid(t, env.pool, fix.TenantID, carrierID, "rev-bid")

	bidRepo := repository.NewBidRepository(env.pool)
	revRepo := repository.NewBidRevisionRepository(env.pool, bidRepo)
	svc := service.NewBidRevisionService(bidRepo, revRepo)

	rev, err := svc.SubmitRevision(ctx, domain.SubmitBidRevisionInput{
		TenantID:         fix.TenantID,
		BidID:            bidID,
		CarrierCompanyID: carrierID,
		TotalAmount:      900,
		CurrencyCode:     "RUB",
		CapacityUnits:    300,
		TransitHours:     20,
		SLAScoreInput:    88,
		CarrierKPIInput:  90,
		ReliabilityInput: 85,
	})
	if err != nil {
		t.Fatalf("submit bid revision: %v", err)
	}
	if rev.RevisionNumber != 1 {
		t.Fatalf("expected revision 1, got %d", rev.RevisionNumber)
	}

	active, err := svc.GetActiveRevision(ctx, bidID, fix.TenantID, &carrierID)
	if err != nil {
		t.Fatalf("get active bid revision: %v", err)
	}
	if active.TotalAmount == nil || *active.TotalAmount != 900 {
		t.Fatalf("unexpected active amount: %+v", active)
	}
}

func TestBidRevisionConcurrency_ResponseRevision(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	fix := seedEnterpriseFixture(t, env.pool, "rev-conc")
	respRepo := repository.NewResponseRevisionRepository(env.pool)
	svc := service.NewResponseBidService(respRepo)
	carrier := fix.Carriers[2]

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := uuid.NewString()
			_, err := svc.SubmitRevision(ctx, domain.SubmitResponseRevisionInput{
				TenantID:             fix.TenantID,
				RfxEventID:           fix.EventID,
				RfxResponseID:        carrier.ResponseID,
				ParticipantCompanyID: carrier.CompanyID,
				PriceAmount:          float64(80 + n),
				CurrencyCode:         "RUB",
				CapacityUnits:        350,
				TransitHours:         19,
				SLAScoreInput:        90,
				CarrierKPIInput:      91,
				ReliabilityInput:     88,
				IdempotencyKey:       &key,
			})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent revision failed: %v", err)
		}
	}

	active, err := svc.GetActiveRevision(ctx, carrier.ResponseID, fix.TenantID, nil)
	if err != nil {
		t.Fatalf("get active after concurrency: %v", err)
	}
	if active.RevisionNumber < 3 {
		t.Fatalf("expected revision >= 3 after concurrent submits, got %d", active.RevisionNumber)
	}
}
