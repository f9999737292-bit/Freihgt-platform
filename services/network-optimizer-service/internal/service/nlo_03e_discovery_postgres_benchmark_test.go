//go:build nlo03ediscovery

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

// Disposable local Postgres only. NOT_STAGING. NOT_PRODUCTION.
// Pairwise same-origin search. NLO-0.3E routing calls are not part of this path.

func TestNLO03EDiscoveryPostgresBenchmark(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	pool := startDiscoveryPostgres(t, ctx)
	applyDiscoveryMigrations(t, ctx, pool)
	t.Logf("ENV go=%s os=%s arch=%s cpus=%d store=embedded-postgres-16 disposable_local=YES NOT_STAGING=YES NOT_PRODUCTION=YES routing_calls_per_set=0",
		runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	for _, n := range []int{10, 50, 100, 500} {
		repeats := 5
		if n >= 500 {
			repeats = 1
		}
		sample := measurePairwisePostgres(t, ctx, pool, n, repeats)
		raw, _ := json.Marshal(sample)
		t.Logf("POSTGRES_PAIRWISE %s", raw)
	}
}

type pgBenchSample struct {
	Pool             int    `json:"pool"`
	Pairs            int    `json:"pairs"`
	CatalogLoads     int    `json:"catalog_loads"`
	GroupageCalls    int    `json:"groupage_calls"`
	RoutingCalls     int    `json:"routing_calls"`
	RunRows          int    `json:"run_rows"`
	CandidateRows    int    `json:"candidate_rows"`
	MemberRows       int    `json:"member_rows"`
	InsertedRows     int    `json:"inserted_rows"`
	ResponseBytes    int    `json:"response_bytes"`
	AllocBytes       uint64 `json:"alloc_bytes"`
	HeapAlloc        uint64 `json:"heap_alloc"`
	DBBytesBefore    int64  `json:"db_bytes_before"`
	DBBytesAfter     int64  `json:"db_bytes_after"`
	DBBytesGrowth    int64  `json:"db_bytes_growth"`
	P50Millis        int64  `json:"p50_ms"`
	ObservedMaxMillis int64 `json:"observed_max_ms"`
	Samples          int    `json:"samples"`
	Fingerprint      string `json:"fingerprint"`
}

func measurePairwisePostgres(t *testing.T, ctx context.Context, pool *pgxpool.Pool, n, repeats int) pgBenchSample {
	t.Helper()
	var durations []time.Duration
	var first pgBenchSample
	var firstHash string
	for run := 0; run < repeats; run++ {
		if _, err := pool.Exec(ctx, `
			TRUNCATE
				network_optimizer.consolidation_candidate_members,
				network_optimizer.consolidation_candidates,
				network_optimizer.consolidation_search_runs,
				network_optimizer.load_opportunities,
				network_optimizer.capacities
			CASCADE`); err != nil {
			t.Fatal(err)
		}
		store := repository.NewPostgres(pool)
		svc := New(store, nil)
		cat := &benchCatalog{}
		svc.UseCatalog(cat)
		carrier := benchID(0x71, 1)
		shipper := benchID(0x72, 1)
		at := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
		cap := domain.Capacity{
			ID: benchID(0x73, 1), OwnerTenantID: carrier, Latitude: f64(55), Longitude: f64(37),
			AvailableFrom: at, AvailableUntil: at.Add(24 * time.Hour), Source: domain.SourceManual,
			VisibilityScope: domain.CapVisPrivate, Status: domain.CapacityAvailable, Version: 1,
			PayloadRemainingKg: f64(20000), VolumeRemainingM3: f64(80), CreatedAt: at, UpdatedAt: at,
		}
		origin, dest := benchID(0x74, 1), benchID(0x75, 1)
		win := span(at, at.Add(4*time.Hour))
		if err := store.Within(ctx, func(tx repository.Tx) error {
			if err := tx.InsertCapacity(ctx, cap); err != nil {
				return err
			}
			for i := 1; i <= n; i++ {
				load := domain.LoadOpportunity{
					ID: benchID(0x76, i), OwnerTenantID: shipper, SourceType: domain.SourceTransportOrder, SourceID: benchID(0x77, i),
					Pickup: benchPlace(origin, "Origin", 55.1, 37.1), Delivery: benchPlace(dest, "Dest", 55.4, 37.4),
					PickupWindow: win, DeliveryWindow: win, VisibilityScope: domain.VisMarketplace,
					Status: domain.LoadPublished, Version: 1, ConsolidationAllowed: true, CrossShipperConsolidationAllowed: true,
					WeightKg: f64(1), VolumeM3: f64(1), CreatedAt: at, UpdatedAt: at,
				}
				if err := tx.InsertLoad(ctx, load); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		var beforeSize int64
		if err := pool.QueryRow(ctx, `SELECT pg_database_size(current_database())`).Scan(&beforeSize); err != nil {
			t.Fatal(err)
		}
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		started := time.Now()
		result, err := svc.SearchConsolidation(ctx, serviceActor(carrier, nil), ConsolidationCommand{CapacityID: cap.ID, Pattern: PatternSameOriginDestination})
		elapsed := time.Since(started)
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatal(err)
		}
		var afterSize int64
		if err := pool.QueryRow(ctx, `SELECT pg_database_size(current_database())`).Scan(&afterSize); err != nil {
			t.Fatal(err)
		}
		var doc ConsolidationResponse
		if err := json.Unmarshal(result.Body, &doc); err != nil {
			t.Fatal(err)
		}
		want := n * (n - 1) / 2
		if doc.PoolLoadCount != n || doc.EvaluatedPairCount != want {
			t.Fatalf("pool %d pairs %d want %d", doc.PoolLoadCount, doc.EvaluatedPairCount, want)
		}
		if cat.calls != want*2 {
			t.Fatalf("catalog %d pairs %d", cat.calls, want)
		}
		var runs, candidates, members int
		if err := pool.QueryRow(ctx, `
			SELECT
				(SELECT count(*) FROM network_optimizer.consolidation_search_runs),
				(SELECT count(*) FROM network_optimizer.consolidation_candidates),
				(SELECT count(*) FROM network_optimizer.consolidation_candidate_members)`).Scan(&runs, &candidates, &members); err != nil {
			t.Fatal(err)
		}
		if runs != 1 || candidates != want || members != want*2 {
			t.Fatalf("rows runs %d candidates %d members %d", runs, candidates, members)
		}
		var fingerprintList string
		if err := pool.QueryRow(ctx, `
			SELECT coalesce(string_agg(candidate_fingerprint, ',' ORDER BY candidate_fingerprint), '')
			FROM network_optimizer.consolidation_candidates
			WHERE search_run_id = $1`, doc.SearchID).Scan(&fingerprintList); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(fingerprintList))
		hash := hex.EncodeToString(sum[:])
		durations = append(durations, elapsed)
		if run == 0 {
			firstHash = hash
			first = pgBenchSample{
				Pool: n, Pairs: want, CatalogLoads: cat.calls, GroupageCalls: cat.calls, RoutingCalls: 0,
				RunRows: runs, CandidateRows: candidates, MemberRows: members, InsertedRows: runs + candidates + members,
				ResponseBytes: len(result.Body), AllocBytes: after.TotalAlloc - before.TotalAlloc, HeapAlloc: after.HeapAlloc,
				DBBytesBefore: beforeSize, DBBytesAfter: afterSize, DBBytesGrowth: afterSize - beforeSize,
				Samples: repeats, Fingerprint: hash,
			}
		} else if hash != firstHash {
			t.Fatalf("fingerprint changed at n=%d", n)
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	first.P50Millis = durations[len(durations)/2].Milliseconds()
	first.ObservedMaxMillis = durations[len(durations)-1].Milliseconds()
	return first
}

func applyDiscoveryMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	names := []string{
		"000001_create_schemas.up.sql",
		"000003_create_transport_tables.up.sql",
		"000075_bno_capacity_marketplace_foundation_v0_1a.up.sql",
		"000076_bno_predictive_capacity_v0_1b.up.sql",
		"000077_bno_cargo_equipment_compatibility_v0_1b2.up.sql",
		"000078_bno_geography_routing_foundation_v0_1c0.up.sql",
		"000079_bno_next_load_candidate_search_v0_1c1.up.sql",
		"000080_bno_match_score_topn_v0_1c2.up.sql",
		"000081_nlo_pairwise_consolidation_v0_3b.up.sql",
		"000082_nlo_onboard_evidence_current_trip_context_v0_3c.up.sql",
		"000083_nlo_current_trip_fill_v0_3d.up.sql",
	}
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infrastructure", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func startDiscoveryPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		RuntimePath(filepath.Join(t.TempDir(), "pg-runtime")).
		DataPath(filepath.Join(t.TempDir(), "pg-data")).
		Database("bno_test").
		Username("freight").Password("freight").
		Version(embeddedpostgres.V16))
	if err := pg.Start(); err != nil {
		t.Fatalf("embedded postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Stop() })
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://freight:freight@127.0.0.1:%d/bno_test?sslmode=disable", port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
