// Command nlo10d-contract-harness is a test-only loopback process.
// It is not a production server and is not registered in the tracking runtime.
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/tracking-service/internal/config"
	"github.com/freight-platform/tracking-service/internal/http/handlers"
	"github.com/freight-platform/tracking-service/internal/repository"
	"github.com/freight-platform/tracking-service/internal/service"
)

const token = "nlo-1.0d-harness-token"

var (
	callerTenant  = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1")
	foreignTenant = uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1")
	shipmentKnown = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	shipmentZero  = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	shipmentNull  = uuid.MustParse("00000000-0000-4000-8000-000000000003")
	shipmentMiss  = uuid.MustParse("00000000-0000-4000-8000-000000000004")
	shipmentNoETA = uuid.MustParse("00000000-0000-4000-8000-000000000005")
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root, err := os.MkdirTemp("", "nlo10d-pg-")
	if err != nil {
		exitErr(err)
	}
	defer os.RemoveAll(root)

	port, err := freePort()
	if err != nil {
		exitErr(err)
	}
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		RuntimePath(filepath.Join(root, "runtime")).
		DataPath(filepath.Join(root, "data")).
		Database("postgres").
		Username("nlo10d").
		Password("nlo10d").
		Version(embeddedpostgres.V16).
		Logger(io.Discard))
	if err := pg.Start(); err != nil {
		exitErr(err)
	}
	defer pg.Stop()

	adminURL := fmt.Sprintf("postgres://nlo10d:nlo10d@127.0.0.1:%d/postgres?sslmode=disable", port)
	dbName := "nlo10d_" + uuid.NewString()[:8]
	pool, drop, err := openTestDB(ctx, adminURL, dbName)
	if err != nil {
		exitErr(err)
	}
	defer drop()
	if err := applyLookupMigrations(ctx, pool); err != nil {
		exitErr(err)
	}
	recordedAt := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)
	observedAt := recordedAt
	if err := seed(ctx, pool, recordedAt, observedAt); err != nil {
		exitErr(err)
	}

	cfg := config.Config{
		FreshnessPolicy:      config.FreshnessConfig{FreshThreshold: 10 * time.Minute, StaleThreshold: 30 * time.Minute},
		ETAFreshnessPolicy:   config.FreshnessConfig{FreshThreshold: 15 * time.Minute, StaleThreshold: 60 * time.Minute},
		InternalServiceToken: token,
	}
	trackingRepo := repository.NewTrackingRepository(pool)
	etaRepo := repository.NewETARepository(pool)
	query := service.NewTrackingQueryService(trackingRepo, service.NewStateEvaluator(trackingRepo, cfg), cfg)
	etaQuery := service.NewETAQueryService(etaRepo, service.NewETAStateEvaluator(etaRepo, cfg))
	lookup := handlers.NewInternalHandler(query, token)
	etaLookup := handlers.NewETAInternalHandler(etaQuery, token)

	router := chi.NewRouter()
	router.Post("/internal/v1/tracking/states/lookup", lookup.LookupStates)
	router.Post("/internal/v1/tracking/eta/lookup", etaLookup.LookupDelivery)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		exitErr(err)
	}
	server := &http.Server{Handler: router}
	go server.Serve(listener)

	fmt.Printf("NLO10D_URL=http://%s\n", listener.Addr().String())
	fmt.Printf("NLO10D_RECORDED_AT=%s\n", recordedAt.Format(time.RFC3339))
	fmt.Printf("NLO10D_OBSERVED_AT=%s\n", observedAt.Format(time.RFC3339))
	os.Stdout.Sync()
	_, _ = io.Copy(io.Discard, os.Stdin)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
	listener.Close()
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func openTestDB(ctx context.Context, adminURL, dbName string) (*pgxpool.Pool, func(), error) {
	adminCfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		return nil, nil, err
	}
	adminCfg.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		return nil, nil, err
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		admin.Close()
		return nil, nil, err
	}
	testCfg := adminCfg.Copy()
	testCfg.ConnConfig.Database = dbName
	pool, err := pgxpool.NewWithConfig(ctx, testCfg)
	if err != nil {
		admin.Close()
		return nil, nil, err
	}
	drop := func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
		admin.Close()
	}
	return pool, drop, nil
}

func applyLookupMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	for _, name := range []string{
		"000026_add_shipment_telemetry_foundation_v0.7.0.up.sql",
		"000027_add_shipment_eta_intelligence_v0.7.1.up.sql",
	} {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func migrationsDir() (string, error) {
	for _, candidate := range []string{
		filepath.Join("..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "infrastructure", "migrations"),
	} {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("migrations directory not found")
}

func seed(ctx context.Context, pool *pgxpool.Pool, recordedAt, observedAt time.Time) error {
	arrival := time.Date(2026, 11, 12, 18, 0, 0, 0, time.UTC)
	foreignArrival := time.Date(2026, 11, 12, 23, 0, 0, 0, time.UTC)
	if err := insertBinding(ctx, pool, callerTenant, shipmentKnown, "dev-known"); err != nil {
		return err
	}
	if err := insertBinding(ctx, pool, callerTenant, shipmentZero, "dev-zero"); err != nil {
		return err
	}
	if err := insertBinding(ctx, pool, callerTenant, shipmentNull, "dev-null"); err != nil {
		return err
	}
	if err := insertBinding(ctx, pool, foreignTenant, shipmentKnown, "dev-foreign"); err != nil {
		return err
	}
	if err := insertState(ctx, pool, callerTenant, shipmentKnown, f64(55.751244), f64(37.618423), &recordedAt); err != nil {
		return err
	}
	if err := insertState(ctx, pool, callerTenant, shipmentZero, f64(0), f64(37.618423), &recordedAt); err != nil {
		return err
	}
	if err := insertState(ctx, pool, callerTenant, shipmentNull, nil, nil, nil); err != nil {
		return err
	}
	if err := insertState(ctx, pool, foreignTenant, shipmentKnown, f64(59.939095), f64(30.315868), &recordedAt); err != nil {
		return err
	}
	if err := insertETA(ctx, pool, callerTenant, shipmentKnown, &arrival, &observedAt); err != nil {
		return err
	}
	if err := insertETA(ctx, pool, callerTenant, shipmentNoETA, nil, nil); err != nil {
		return err
	}
	return insertETA(ctx, pool, foreignTenant, shipmentKnown, &foreignArrival, &observedAt)
}

func insertBinding(ctx context.Context, pool *pgxpool.Pool, tenant, shipment uuid.UUID, device string) error {
	_, err := pool.Exec(ctx, `
INSERT INTO tracking.shipment_tracking_binding
	(tenant_id, shipment_id, provider_code, provider_device_id, status)
VALUES ($1,$2,'nlo10d',$3,'active')`, tenant, shipment, device)
	return err
}

func insertState(ctx context.Context, pool *pgxpool.Pool, tenant, shipment uuid.UUID, lat, lon *float64, recordedAt *time.Time) error {
	_, err := pool.Exec(ctx, `
INSERT INTO tracking.shipment_tracking_state
	(tenant_id, shipment_id, tracking_status, provider_code, last_latitude, last_longitude,
	 last_recorded_at, last_received_at, freshness_status, quality_status, age_seconds)
VALUES ($1,$2,'lost','nlo10d',$3,$4,$5,$5,'lost','poor',999999)`, tenant, shipment, lat, lon, recordedAt)
	return err
}

func insertETA(ctx context.Context, pool *pgxpool.Pool, tenant, shipment uuid.UUID, arrival, observed *time.Time) error {
	_, err := pool.Exec(ctx, `
INSERT INTO tracking.shipment_eta_state
	(tenant_id, shipment_id, target_type, status, estimated_arrival_at, source_type, provider_code,
	 source_observed_at, received_at, freshness_status, quality_status)
VALUES ($1,$2,'delivery','expired',$3,'provider_eta','nlo10d',$4,$4,'expired','poor')`,
		tenant, shipment, arrival, observed)
	return err
}

func f64(v float64) *float64 { return &v }

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	panic(err)
}
