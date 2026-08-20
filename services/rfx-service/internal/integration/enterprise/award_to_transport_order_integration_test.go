//go:build integration

package enterprise

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type conversionFixture struct {
	buyerFixture
	OriginID uuid.UUID
	DestID   uuid.UUID
}

func seedConversionFixture(t *testing.T, env *testEnv) conversionFixture {
	t.Helper()
	fix := seedBuyerFixture(t, env)
	originID := uuid.New()
	destID := uuid.New()
	ctx := context.Background()
	for _, loc := range []struct {
		id   uuid.UUID
		name string
	}{
		{originID, "Origin WH"},
		{destID, "Dest DC"},
	} {
		_, err := env.pool.Exec(ctx, `
			INSERT INTO transport.locations (id, tenant_id, company_id, location_type, name, country_code, city)
			VALUES ($1, $2, $3, 'WAREHOUSE', $4, 'RU', 'Moscow')
		`, loc.id, fix.TenantID, fix.CompanyA, loc.name)
		if err != nil {
			t.Fatalf("seed location: %v", err)
		}
	}
	return conversionFixture{buyerFixture: fix, OriginID: originID, DestID: destID}
}

func seedAwardedMultiLotEvent(t *testing.T, env *testEnv, fix conversionFixture, amountA, amountB float64) (*domain.RfxEvent, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	currency := "RUB"
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Conversion Tender",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-C-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline, CurrencyCode: &currency,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := env.rfxSvc.AddParticipant(ctx, fix.BuyerA, event.ID, domain.AddRfxParticipantInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, CompanyID: fix.CarrierID, ParticipantType: "CARRIER",
	}); err != nil {
		t.Fatalf("add participant: %v", err)
	}
	lotA, err := env.rfxSvc.CreateLot(ctx, fix.BuyerA, event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, LotNumber: "LOT-A", Name: "Lot A",
	})
	if err != nil {
		t.Fatalf("create lot A: %v", err)
	}
	lotB, err := env.rfxSvc.CreateLot(ctx, fix.BuyerA, event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, LotNumber: "LOT-B", Name: "Lot B",
	})
	if err != nil {
		t.Fatalf("create lot B: %v", err)
	}
	for _, lot := range []*domain.RfxLot{lotA, lotB} {
		if _, err := env.rfxSvc.CreateLane(ctx, fix.BuyerA, lot.ID, domain.CreateRfxLaneInput{
			TenantID: fix.TenantID, RfxLotID: lot.ID,
			OriginLocationID: fix.OriginID, DestinationLocationID: fix.DestID, TransportMode: "ROAD",
		}); err != nil {
			t.Fatalf("create lane: %v", err)
		}
	}
	if _, err := env.rfxSvc.PublishEvent(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	resp, err := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	if err != nil {
		t.Fatalf("create response: %v", err)
	}
	if _, err := env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, resp.ID, []domain.UpsertOfferLineInput{
		{RfxLotID: lotA.ID, Amount: amountA, CurrencyCode: "RUB"},
		{RfxLotID: lotB.ID, Amount: amountB, CurrencyCode: "RUB"},
	}); err != nil {
		t.Fatalf("update commercial: %v", err)
	}
	if _, err := env.rfxSvc.SubmitResponse(ctx, fix.CarrierAct, resp.ID); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := env.rfxSvc.AwardResponse(ctx, fix.BuyerA, event.ID, resp.ID); err != nil {
		t.Fatalf("award: %v", err)
	}
	return event, lotA.ID, lotB.ID
}

func TestBuyerOwnAwardedConversionAllowed(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 100000, 95000)

	result, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	if err != nil || !result.Created || len(result.Items) != 2 {
		t.Fatalf("convert: err=%v created=%v len=%d", err, result.Created, len(result.Items))
	}
	var orderCount, snapshotCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.transport_orders WHERE tenant_id = $1`, fix.TenantID).Scan(&orderCount); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM transport.transport_order_rate_snapshots s
		JOIN transport.transport_orders o ON o.id = s.transport_order_id
		WHERE s.tenant_id = $1 AND o.pricing_model_version = 'SNAPSHOT_V1'`, fix.TenantID).Scan(&snapshotCount); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if orderCount != 2 || snapshotCount != 2 {
		t.Fatalf("expected 2 orders and 2 snapshots, got orders=%d snapshots=%d", orderCount, snapshotCount)
	}
}

func TestSameTenantForeignBuyerCompanyConversionDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 100000, 95000)

	_, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerB, event.ID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestCrossTenantConversionDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 100000, 95000)

	foreignBuyer := domain.ActorContext{TenantID: uuid.New(), UserID: uuid.New()}
	_, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, foreignBuyer, event.ID)
	assertAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestConversionDeniedForCarrierActor(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 100000, 95000)

	_, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.CarrierAct, event.ID)
	if err == nil {
		t.Fatal("expected carrier conversion denied")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected app error, got %v", err)
	}
	if appErr.Code != apperrors.CodeForbidden && appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected FORBIDDEN or NOT_FOUND, got %s", appErr.Code)
	}
}

func TestNonAwardedConversionDenied(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	deadline := time.Now().UTC().Add(24 * time.Hour)
	currency := "RUB"
	event, err := env.rfxSvc.CreateEvent(ctx, fix.BuyerA, domain.CreateRfxEventInput{
		TenantID: fix.TenantID, OwnerCompanyID: fix.CompanyA, Title: "Open Tender",
		RfxType: "SPOT_RFQ", Category: "FREIGHT", RfxNumber: "RFX-O-" + uuid.NewString()[:8],
		ResponseDeadline: &deadline, CurrencyCode: &currency,
	})
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	_, err = env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	assertAppErrorCode(t, err, apperrors.CodeValidation)
}

func TestRepeatedConversionIdempotent(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 100000, 95000)

	first, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	if err != nil || !first.Created {
		t.Fatalf("first convert: %v", err)
	}
	second, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	if err != nil || second.Created {
		t.Fatalf("second convert: err=%v created=%v", err, second.Created)
	}
	if len(first.Items) != len(second.Items) {
		t.Fatalf("item count mismatch")
	}
	for i := range first.Items {
		if first.Items[i].TransportOrderID != second.Items[i].TransportOrderID {
			t.Fatalf("transport order id changed on retry")
		}
	}
	var orderCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.transport_orders WHERE tenant_id = $1`, fix.TenantID).Scan(&orderCount); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if orderCount != 2 {
		t.Fatalf("expected 2 orders after idempotent retry, got %d", orderCount)
	}
}

func TestConcurrentConversionNoDuplicate(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 120000, 80000)

	var wg sync.WaitGroup
	results := make([]*domain.ConvertAwardTransportOrdersResult, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
		}(i)
	}
	wg.Wait()
	success := 0
	for i, err := range errs {
		if err == nil {
			success++
			if len(results[i].Items) != 2 {
				t.Fatalf("expected 2 items, got %d", len(results[i].Items))
			}
		}
	}
	if success == 0 {
		t.Fatalf("both conversions failed: %v %v", errs[0], errs[1])
	}
	var orderCount int
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.transport_orders WHERE tenant_id = $1`, fix.TenantID).Scan(&orderCount); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if orderCount != 2 {
		t.Fatalf("expected exactly 2 orders after concurrent conversion, got %d", orderCount)
	}
}

func TestMultiLotPriceMappingPreserved(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, lotA, lotB := seedAwardedMultiLotEvent(t, env, fix, 111111, 222222)

	result, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	amounts := map[uuid.UUID]float64{}
	for _, item := range result.Items {
		amounts[item.RfxLotID] = item.Amount
	}
	if amounts[lotA] != 111111 || amounts[lotB] != 222222 {
		t.Fatalf("amounts not preserved: lotA=%v lotB=%v", amounts[lotA], amounts[lotB])
	}
}

func TestConversionAuditRecorded(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, _, _ := seedAwardedMultiLotEvent(t, env, fix, 100000, 95000)

	if _, err := env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID); err != nil {
		t.Fatalf("convert: %v", err)
	}
	count, err := countAuditEvents(ctx, env.pool, fix.TenantID, event.ID, "convert_award_transport_orders")
	if err != nil || count != 1 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
}

func TestCompetitorDataIsolationAfterConversion(t *testing.T) {
	env := setupTestEnv(t)
	fix := seedConversionFixture(t, env)
	ctx := context.Background()
	event, carrierB, carrierBAct := seedEvaluationEvent(t, env, fix.buyerFixture)

	lot, err := env.rfxSvc.CreateLot(ctx, fix.BuyerA, event.ID, domain.CreateRfxLotInput{
		TenantID: fix.TenantID, RfxEventID: event.ID, LotNumber: "LOT-1", Name: "Lot 1",
	})
	if err != nil {
		t.Fatalf("create lot: %v", err)
	}
	if _, err := env.rfxSvc.CreateLane(ctx, fix.BuyerA, lot.ID, domain.CreateRfxLaneInput{
		TenantID: fix.TenantID, RfxLotID: lot.ID,
		OriginLocationID: fix.OriginID, DestinationLocationID: fix.DestID, TransportMode: "ROAD",
	}); err != nil {
		t.Fatalf("create lane: %v", err)
	}

	respA, _ := env.rfxSvc.CreateResponse(ctx, fix.CarrierAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: fix.CarrierID,
	})
	_, _ = env.rfxSvc.UpdateResponseCommercial(ctx, fix.CarrierAct, respA.ID, []domain.UpsertOfferLineInput{
		{RfxLotID: lot.ID, Amount: 100000, CurrencyCode: "RUB"},
	})
	_, _ = env.rfxSvc.SubmitResponse(ctx, fix.CarrierAct, respA.ID)

	respB, _ := env.rfxSvc.CreateResponse(ctx, carrierBAct, event.ID, domain.CreateRfxResponseInput{
		TenantID: fix.TenantID, ParticipantCompanyID: carrierB,
	})
	_, _ = env.rfxSvc.UpdateResponseCommercial(ctx, carrierBAct, respB.ID, []domain.UpsertOfferLineInput{
		{RfxLotID: lot.ID, Amount: 80000, CurrencyCode: "RUB"},
	})
	_, _ = env.rfxSvc.SubmitResponse(ctx, carrierBAct, respB.ID)
	_, _ = env.rfxSvc.AwardResponse(ctx, fix.BuyerA, event.ID, respA.ID)
	_, _ = env.rfxSvc.ConvertAwardToTransportOrders(ctx, fix.BuyerA, event.ID)

	_, err = env.rfxSvc.ListEvaluationResponses(ctx, fix.CarrierAct, event.ID)
	if err == nil {
		t.Fatal("carrier must not read buyer evaluation list after conversion")
	}
	_, err = env.rfxSvc.GetResponse(ctx, fix.CarrierAct, respB.ID)
	if err == nil {
		t.Fatal("carrier must not read competitor response")
	}
}
