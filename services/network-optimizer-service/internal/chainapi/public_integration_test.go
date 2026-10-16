//go:build integration

package chainapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/chainapi"
	"github.com/freight-platform/network-optimizer-service/internal/chainorch"
	httpserver "github.com/freight-platform/network-optimizer-service/internal/http"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
	"github.com/freight-platform/network-optimizer-service/internal/routeplan"
	"github.com/freight-platform/network-optimizer-service/internal/routing"
	"github.com/freight-platform/network-optimizer-service/internal/service"
	"github.com/freight-platform/shared-go/lowcode"
)

func TestI2C_PublicChainSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool := startPostgres(t, ctx)
	applyThrough000097(t, ctx, pool)
	store := repository.NewPostgres(pool)
	seeds := &memSeeds{}
	loads := &memLoads{}
	prov := &scriptRoute{}
	api := newAPI(store, seeds, loads, prov)
	handler := httpserver.NewRouterWithChain(nil, nil, nil, nil, api)

	tenant := uuid.New()
	company := uuid.New()
	user := uuid.New()
	actor := service.Actor{TenantID: tenant, UserID: user, CompanyID: &company}
	capID := uuid.New()
	from := time.Now().UTC().Add(time.Hour)
	until := from.Add(24 * time.Hour)
	version := 3
	seeds.manual[capID] = chainapi.ResolvedSeed{
		InternalSource: repository.ChainSeedManualAvailableCapacity, CapacityID: &capID, CapacityVersion: &version,
		AvailableFrom: &from, AvailableUntil: &until,
		ChainSeed: chain.Seed{Source: chain.SeedManualAvailable, AvailableFrom: &from, AvailableUntil: &until,
			Start:    routeplan.Point{Kind: routeplan.PointAnchor, Latitude: 0, Longitude: 10, Source: routeplan.SourceCanonical},
			Capacity: chain.Capacity{Payload: chain.Dimension{Known: true, Value: 1000}, Volume: chain.Dimension{Known: true, Value: 1000}, Pallets: chain.Dimension{Known: true, Value: 1000}, Linear: chain.Dimension{Known: true, Value: 1000}, Height: chain.Dimension{Known: true, Value: 1000}},
		},
	}
	shipID := uuid.New()
	shipVersion := 4
	seeds.trips[shipID] = chainapi.ResolvedSeed{
		InternalSource: repository.ChainSeedCurrentTripResidual, ShipmentID: &shipID, ShipmentVersion: &shipVersion,
		HorizonPolicy: chainorch.HorizonAnchorSearchClock,
		ChainSeed:     chain.Seed{Source: chain.SeedCurrentTripResidual, Start: routeplan.Point{Kind: routeplan.PointAnchor, Latitude: 1, Longitude: 2, Source: routeplan.SourceTracking}, Capacity: chain.Capacity{Payload: chain.Dimension{Known: true, Value: 1000}}},
	}
	loads.rows = twoLoads(tenant, time.Now().UTC())

	t.Run("I2C-POST-01 manual capacity 201", func(t *testing.T) {
		before := prov.calls.Load()
		rec := post(handler, actor, fmt.Sprintf(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"%s"}`, capID), "post-01")
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		body := decodeSearch(t, rec.Body.Bytes())
		if body.Status != repository.ChainSearchCompleted || !body.SearchComplete || body.ExecutionSupported || body.GlobalOptimum {
			t.Fatalf("%+v", body)
		}
		if body.Seed.Type != chainapi.PublicSeedManualCapacity || body.Seed.CapacityVersion == nil || *body.Seed.CapacityVersion != 3 {
			t.Fatalf("seed %+v", body.Seed)
		}
		if prov.calls.Load() <= before {
			t.Fatal("expected provider work on first create")
		}
		if strings.Contains(rec.Body.String(), "owner_token") || strings.Contains(rec.Body.String(), "request_fingerprint") || strings.Contains(rec.Body.String(), "claim_expires") {
			t.Fatal("private field leaked")
		}
	})

	t.Run("I2C-POST-02 current trip 201", func(t *testing.T) {
		rec := post(handler, actor, fmt.Sprintf(`{"seed_type":"CURRENT_TRIP","shipment_id":"%s"}`, shipID), "post-02")
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		body := decodeSearch(t, rec.Body.Bytes())
		if body.Seed.Type != chainapi.PublicSeedCurrentTrip || body.Seed.ShipmentID == nil || *body.Seed.ShipmentID != shipID {
			t.Fatalf("seed %+v", body.Seed)
		}
	})

	t.Run("I2C-POST-03 predicted 422", func(t *testing.T) {
		rec := post(handler, actor, `{"seed_type":"PREDICTED_CAPACITY","capacity_id":"`+capID.String()+`"}`, "")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("I2C-POST-04 candidate_load_ids rejected", func(t *testing.T) {
		rec := post(handler, actor, `{"seed_type":"MANUAL_CAPACITY","capacity_id":"`+capID.String()+`","candidate_load_ids":[]}`, "")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("I2C-POST-05 L1 rejected", func(t *testing.T) {
		rec := post(handler, actor, `{"seed_type":"MANUAL_CAPACITY","capacity_id":"`+capID.String()+`","L1":"x"}`, "")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	t.Run("I2C-POST-06 chain_search_id rejected", func(t *testing.T) {
		rec := post(handler, actor, `{"seed_type":"MANUAL_CAPACITY","capacity_id":"`+capID.String()+`","chain_search_id":"`+uuid.NewString()+`"}`, "")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("I2C-POST-07 terminal replay 200", func(t *testing.T) {
		calls := api.ComputeInvocations()
		rec := post(handler, actor, fmt.Sprintf(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"%s"}`, capID), "post-01")
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if api.ComputeInvocations() != calls {
			t.Fatal("replay recomputed")
		}
	})

	t.Run("I2C-POST-15 no duplicate provider compute", func(t *testing.T) {
		before := prov.calls.Load()
		rec := post(handler, actor, fmt.Sprintf(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"%s"}`, capID), "post-01")
		if rec.Code != http.StatusOK || prov.calls.Load() != before {
			t.Fatalf("status=%d calls before=%d after=%d", rec.Code, before, prov.calls.Load())
		}
	})

	t.Run("I2C-POST-08 fingerprint conflict", func(t *testing.T) {
		other := uuid.New()
		seeds.manual[other] = seeds.manual[capID]
		id := other
		seeds.manual[other] = cloneSeed(seeds.manual[capID], &id)
		rec := post(handler, actor, fmt.Sprintf(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"%s"}`, other), "post-01")
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "FINGERPRINT_CONFLICT") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("I2C-POST-09-10 in-flight 409", func(t *testing.T) {
		pendingCap := uuid.New()
		seeds.manual[pendingCap] = cloneSeed(seeds.manual[capID], &pendingCap)
		resolved := seeds.manual[pendingCap]
		fp, err := api.Fingerprint(ctx, actor, resolved)
		if err != nil {
			t.Fatal(err)
		}
		claim, err := api.Orch.Claim(ctx, chainorch.ClaimRequest{TenantID: tenant, IdempotencyKey: "inflight", Fingerprint: fp})
		if err != nil {
			t.Fatal(err)
		}
		rec := post(handler, actor, fmt.Sprintf(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"%s"}`, pendingCap), "inflight")
		if rec.Code != http.StatusConflict || rec.Header().Get("Retry-After") != "1" || !strings.Contains(rec.Body.String(), claim.Search.ID.String()) {
			t.Fatalf("status=%d retry=%q body=%s", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
		}
	})

	t.Run("I2C-POST-11 expired claim interrupted replay", func(t *testing.T) {
		expiredCap := uuid.New()
		seeds.manual[expiredCap] = cloneSeed(seeds.manual[capID], &expiredCap)
		fp, err := api.Fingerprint(ctx, actor, seeds.manual[expiredCap])
		if err != nil {
			t.Fatal(err)
		}
		claim, err := api.Orch.Claim(ctx, chainorch.ClaimRequest{TenantID: tenant, IdempotencyKey: "expired", Fingerprint: fp})
		if err != nil {
			t.Fatal(err)
		}
		expireClaim(t, ctx, pool, claim.Search.ID)
		calls := api.ComputeInvocations()
		rec := post(handler, actor, fmt.Sprintf(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"%s"}`, expiredCap), "expired")
		body := decodeSearch(t, rec.Body.Bytes())
		if rec.Code != http.StatusOK || body.Status != repository.ChainSearchFailed || body.FailureCode == nil || *body.FailureCode != repository.ChainInterruptedFailureCode {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if api.ComputeInvocations() != calls {
			t.Fatal("expired replay recomputed")
		}
	})

	t.Run("I2C-POST-12-13 omitted key distinct searches", func(t *testing.T) {
		one := post(handler, actor, fmt.Sprintf(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"%s"}`, capID), "")
		two := post(handler, actor, fmt.Sprintf(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"%s"}`, capID), "")
		if one.Code != http.StatusCreated || two.Code != http.StatusCreated {
			t.Fatalf("one=%d two=%d", one.Code, two.Code)
		}
		a := decodeSearch(t, one.Body.Bytes())
		b := decodeSearch(t, two.Body.Bytes())
		if a.ChainSearchID == b.ChainSearchID {
			t.Fatal("omitted keys replayed")
		}
	})

	t.Run("I2C-POST-14 same key other tenant", func(t *testing.T) {
		otherTenant := uuid.New()
		otherActor := service.Actor{TenantID: otherTenant, UserID: uuid.New(), CompanyID: &company}
		if seeds.owners[capID] == nil {
			seeds.owners[capID] = map[uuid.UUID]struct{}{}
		}
		seeds.owners[capID][tenant] = struct{}{}
		seeds.owners[capID][otherTenant] = struct{}{}
		rec := post(handler, otherActor, fmt.Sprintf(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"%s"}`, capID), "post-01")
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	var completedID uuid.UUID
	t.Run("I2C-GET-01 completed readable", func(t *testing.T) {
		rec := post(handler, actor, fmt.Sprintf(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"%s"}`, capID), "post-01")
		body := decodeSearch(t, rec.Body.Bytes())
		completedID = body.ChainSearchID
		got := getSearch(handler, actor, completedID)
		if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), completedID.String()) {
			t.Fatalf("status=%d body=%s", got.Code, got.Body.String())
		}
	})

	t.Run("I2C-GET-02 readable after new repository handle", func(t *testing.T) {
		restarted := newAPI(store, seeds, loads, prov)
		h := httpserver.NewRouterWithChain(nil, nil, nil, nil, restarted)
		got := getSearch(h, actor, completedID)
		if got.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", got.Code, got.Body.String())
		}
	})

	t.Run("I2C-GET-03 ordinal order", func(t *testing.T) {
		rec := getCandidates(handler, actor, completedID)
		var list chainapi.CandidateList
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatal(err)
		}
		if len(list.Candidates) == 0 {
			t.Fatalf("empty shortlist body=%s", rec.Body.String())
		}
		for i := 1; i < len(list.Candidates); i++ {
			if list.Candidates[i].Ordinal <= list.Candidates[i-1].Ordinal {
				t.Fatalf("ordinals %+v", list.Candidates)
			}
		}
	})

	t.Run("I2C-GET-04 pending candidates empty", func(t *testing.T) {
		pendingCap := uuid.New()
		seeds.manual[pendingCap] = cloneSeed(seeds.manual[capID], &pendingCap)
		fp, err := api.Fingerprint(ctx, actor, seeds.manual[pendingCap])
		if err != nil {
			t.Fatal(err)
		}
		claim, err := api.Orch.Claim(ctx, chainorch.ClaimRequest{TenantID: tenant, IdempotencyKey: "pending-cands", Fingerprint: fp})
		if err != nil {
			t.Fatal(err)
		}
		rec := getCandidates(handler, actor, claim.Search.ID)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"candidates":[]`) && !strings.Contains(rec.Body.String(), `"candidates": []`) {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("I2C-GET-05 expired pending recovered", func(t *testing.T) {
		id := insertStatus(t, ctx, pool, tenant, repository.ChainSearchPending)
		calls := prov.calls.Load()
		rec := getSearch(handler, actor, id)
		body := decodeSearch(t, rec.Body.Bytes())
		if body.Status != repository.ChainSearchFailed || body.FailureCode == nil || *body.FailureCode != repository.ChainInterruptedFailureCode || prov.calls.Load() != calls {
			t.Fatalf("status=%s calls=%d body=%s", body.Status, prov.calls.Load(), rec.Body.String())
		}
	})

	t.Run("I2C-GET-06 expired running recovered", func(t *testing.T) {
		id := insertStatus(t, ctx, pool, tenant, repository.ChainSearchRunning)
		rec := getSearch(handler, actor, id)
		body := decodeSearch(t, rec.Body.Bytes())
		if body.Status != repository.ChainSearchFailed || body.FailureCode == nil || *body.FailureCode != repository.ChainInterruptedFailureCode {
			t.Fatalf("%s", rec.Body.String())
		}
	})

	t.Run("I2C-GET-07 recovery zero provider calls", func(t *testing.T) {
		id := insertStatus(t, ctx, pool, tenant, repository.ChainSearchPending)
		before := prov.calls.Load()
		_ = getSearch(handler, actor, id)
		if prov.calls.Load() != before {
			t.Fatal("recovery called provider")
		}
	})

	t.Run("I2C-GET-08 terminal immutable", func(t *testing.T) {
		first := getSearch(handler, actor, completedID)
		second := getSearch(handler, actor, completedID)
		if first.Body.String() != second.Body.String() || !strings.Contains(first.Body.String(), `"status":"COMPLETED"`) {
			t.Fatal("terminal body changed")
		}
	})

	t.Run("I2C-GET-09 cancelled candidates empty", func(t *testing.T) {
		id := insertStatus(t, ctx, pool, tenant, repository.ChainSearchCancelled)
		rec := getCandidates(handler, actor, id)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "chain_candidate_id") {
			t.Fatalf("%s", rec.Body.String())
		}
	})

	t.Run("I2C-GET-10 failed candidates empty", func(t *testing.T) {
		id := insertStatus(t, ctx, pool, tenant, repository.ChainSearchFailed)
		rec := getCandidates(handler, actor, id)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "chain_candidate_id") {
			t.Fatalf("%s", rec.Body.String())
		}
	})

	t.Run("I2C-SEC-10-11 foreign and unknown 404 match", func(t *testing.T) {
		foreign := insertStatus(t, ctx, pool, uuid.New(), repository.ChainSearchCompleted)
		unknown := uuid.New()
		a := getSearch(handler, actor, foreign)
		b := getSearch(handler, actor, unknown)
		if a.Code != http.StatusNotFound || a.Body.String() != b.Body.String() {
			t.Fatalf("search a=%d %s b=%d %s", a.Code, a.Body.String(), b.Code, b.Body.String())
		}
		c := getCandidates(handler, actor, foreign)
		d := getCandidates(handler, actor, unknown)
		if c.Code != http.StatusNotFound || c.Body.String() != d.Body.String() {
			t.Fatalf("cand %s vs %s", c.Body.String(), d.Body.String())
		}
	})

	t.Run("I2C-SEC-12 no private fields", func(t *testing.T) {
		rec := getSearch(handler, actor, completedID)
		for _, forbidden := range []string{"owner_token", "tenant_id", "request_fingerprint", "claim_expires_at", "shipper", "customer", "contract"} {
			if strings.Contains(strings.ToLower(rec.Body.String()), forbidden) {
				t.Fatalf("leaked %s", forbidden)
			}
		}
	})

	t.Run("I2C recovery startup and periodic", func(t *testing.T) {
		id := insertStatus(t, ctx, pool, tenant, repository.ChainSearchPending)
		sweepCtx, stop := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			chainapi.RunRecovery(sweepCtx, api.Orch, chainapi.RecoveryConfig{Interval: 30 * time.Millisecond, Limit: 32})
		}()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			rec := getSearch(handler, actor, id)
			if strings.Contains(rec.Body.String(), repository.ChainInterruptedFailureCode) {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		stop()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("sweep did not stop")
		}
		rec := getSearch(handler, actor, id)
		if !strings.Contains(rec.Body.String(), repository.ChainInterruptedFailureCode) {
			t.Fatalf("not recovered %s", rec.Body.String())
		}
	})
}

type memSeeds struct {
	manual map[uuid.UUID]chainapi.ResolvedSeed
	trips  map[uuid.UUID]chainapi.ResolvedSeed
	owners map[uuid.UUID]map[uuid.UUID]struct{}
}

func (m *memSeeds) ResolveManual(_ context.Context, actor service.Actor, id uuid.UUID) (chainapi.ResolvedSeed, error) {
	row, ok := m.manual[id]
	if !ok {
		return chainapi.ResolvedSeed{}, chainapi.ErrSeedIneligible
	}
	if owners := m.owners[id]; owners != nil {
		if _, ok := owners[actor.TenantID]; !ok {
			return chainapi.ResolvedSeed{}, chainapi.ErrSeedIneligible
		}
	}
	return row, nil
}

func (m *memSeeds) ResolveTrip(_ context.Context, _ service.Actor, id uuid.UUID) (chainapi.ResolvedSeed, error) {
	row, ok := m.trips[id]
	if !ok {
		return chainapi.ResolvedSeed{}, chainapi.ErrSeedIneligible
	}
	return row, nil
}

type memLoads struct{ rows []chain.VisibleLoad }

func (m *memLoads) Loads(context.Context, service.Actor) ([]chain.VisibleLoad, error) {
	return m.rows, nil
}

type scriptRoute struct{ calls atomic.Int32 }

func (p *scriptRoute) Route(context.Context, routing.RouteRequest) (routing.RouteResult, error) {
	p.calls.Add(1)
	return routing.RouteResult{Provider: "script", DistanceM: 1000, DurationSeconds: 60, CalculatedAt: time.Unix(1, 0).UTC(), ExpiresAt: time.Unix(1, 0).UTC().Add(time.Hour)}, nil
}
func (p *scriptRoute) Matrix(context.Context, routing.MatrixRequest) (routing.MatrixResult, error) {
	return routing.MatrixResult{}, routing.ErrInvalidResponse
}

func newAPI(store *repository.Postgres, seeds *memSeeds, loads *memLoads, route routing.Provider) *chainapi.API {
	if seeds.manual == nil {
		seeds.manual = map[uuid.UUID]chainapi.ResolvedSeed{}
	}
	if seeds.trips == nil {
		seeds.trips = map[uuid.UUID]chainapi.ResolvedSeed{}
	}
	if seeds.owners == nil {
		seeds.owners = map[uuid.UUID]map[uuid.UUID]struct{}{}
	}
	return &chainapi.API{
		Orch:  &chainorch.Orchestrator{Store: store, DurationResolver: chainorch.FixedDurationResolver{Snapshot: chainorch.KnownZeroDurationSnapshot(uuid.New(), 1)}},
		Seeds: seeds, Pool: loads, Route: route, RoutingProviderVersion: "script",
	}
}

func cloneSeed(in chainapi.ResolvedSeed, id *uuid.UUID) chainapi.ResolvedSeed {
	in.CapacityID = id
	return in
}

func post(h http.Handler, actor service.Actor, body, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/network/chain-searches", strings.NewReader(body))
	stamp(req, actor)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getSearch(h http.Handler, actor service.Actor, id uuid.UUID) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/v1/network/chain-searches/"+id.String(), nil)
	stamp(req, actor)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getCandidates(h http.Handler, actor service.Actor, id uuid.UUID) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/v1/network/chain-searches/"+id.String()+"/candidates", nil)
	stamp(req, actor)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func stamp(req *http.Request, actor service.Actor) {
	req.Header.Set(lowcode.HeaderTenantID, actor.TenantID.String())
	req.Header.Set(lowcode.HeaderUserID, actor.UserID.String())
	if actor.CompanyID != nil {
		req.Header.Set("X-Company-ID", actor.CompanyID.String())
	}
}

func decodeSearch(t *testing.T, raw []byte) chainapi.SearchResource {
	t.Helper()
	var body chainapi.SearchResource
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode %v %s", err, raw)
	}
	return body
}

func twoLoads(tenant uuid.UUID, clock time.Time) []chain.VisibleLoad {
	stack := true
	out := make([]chain.VisibleLoad, 2)
	for i := 0; i < 2; i++ {
		pid, did := uuid.New(), uuid.New()
		out[i] = chain.VisibleLoad{
			ID: uuid.New(), Version: 1, OwnerTenantID: tenant, ConsolidationAllowed: true, Stackable: &stack,
			Pickup:      routeplan.Point{Kind: routeplan.PointCanonical, LocationID: &pid, Latitude: float64(i + 1), Longitude: 10, Source: routeplan.SourceCanonical},
			Delivery:    routeplan.Point{Kind: routeplan.PointCanonical, LocationID: &did, Latitude: float64(i + 2), Longitude: 11, Source: routeplan.SourceCanonical},
			PickupStart: clock.Add(time.Hour), PickupEnd: clock.Add(10 * time.Hour),
			DeliveryStart: clock.Add(2 * time.Hour), DeliveryEnd: clock.Add(30 * time.Hour),
			WeightKg: 1, PublicationLabel: "VISIBLE_AREA",
		}
	}
	return out
}

func expireClaim(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(ctx, `UPDATE network_optimizer.chain_search_runs SET created_at = now() - interval '26 seconds', deadline_at = now() - interval '6 seconds', claim_expires_at = now() - interval '1 second' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
}

func insertStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenant uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	owner := uuid.New()
	capID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO network_optimizer.chain_search_runs (
			id, tenant_id, status, created_at, started_at, completed_at, cancelled_at,
			request_fingerprint, policy_version, algorithm_version,
			catalog_fingerprint, routing_provider_version, deadline_at, owner_token, claim_expires_at,
			service_duration_snapshot_json, horizon_start, horizon_end, seed_source,
			seed_capacity_id, seed_capacity_version,
			candidate_count, evaluated_pair_count, provider_call_count, search_complete, global_optimum, execution_supported,
			failure_code, reason_counts_json
		) VALUES (
			$1,$2,$3, now() - interval '26 seconds',
			CASE WHEN $3 IN ('RUNNING','COMPLETED','FAILED','CANCELLED') THEN now() - interval '26 seconds' ELSE NULL END,
			CASE WHEN $3 = 'FAILED' OR $3 = 'COMPLETED' THEN now() - interval '1 second' ELSE NULL END,
			CASE WHEN $3 = 'CANCELLED' THEN now() - interval '1 second' ELSE NULL END,
			$4, 'p','a','c','UNKNOWN',
			(now() - interval '26 seconds') + interval '20 seconds', $5,
			(now() - interval '26 seconds') + interval '25 seconds',
			'{"state":"UNKNOWN","pickup_known":false,"delivery_known":false}'::jsonb,
			now(), now() + interval '48 hours', 'MANUAL_AVAILABLE_CAPACITY',
			$6, 1,
			0,0,0, $3 = 'COMPLETED', false, false,
			CASE WHEN $3 = 'FAILED' THEN 'CHAIN_INTERRUPTED' ELSE NULL END,
			'{}'::jsonb
		)`, id, tenant, status, "fp-"+id.String(), owner, capID); err != nil {
		t.Fatal(err)
	}
	return id
}

func startPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).RuntimePath(filepath.Join(t.TempDir(), "pg-runtime")).DataPath(filepath.Join(t.TempDir(), "pg-data")).
		Database("bno_test").Username("freight").Password("freight").Version(embeddedpostgres.V16))
	if err := pg.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.Stop() })
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://freight:freight@127.0.0.1:%d/bno_test?sslmode=disable", port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func applyThrough000097(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "..", "infrastructure", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, file := range files {
		base := filepath.Base(file)
		if len(base) < 6 {
			continue
		}
		n, err := strconv.Atoi(base[:6])
		if err != nil || n > 96 {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			t.Fatalf("%s: %v", base, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "000097_nlo_chain_search_persistence_v0_9c_i2a.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
}
