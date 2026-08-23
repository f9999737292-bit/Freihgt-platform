//go:build integration

package analytics

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/freight-platform/freight-cost-service/internal/domain"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

const maxMigrationFile = "000061_freight_cost_analytics_projection_v2.2B.up.sql"

type analyticsEnv struct {
	pool      *pgxpool.Pool
	analytics *service.AnalyticsProjectionService
	summaries *repository.CostSummaryProjectionRepository
	periods   *repository.AnalyticsPeriodProjectionRepository
	ingest    *service.IngestService
}

func setupAnalyticsEnv(t *testing.T) *analyticsEnv {
	t.Helper()
	ctx := context.Background()
	url := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := applyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	metrics := fcmetrics.New()
	entries := repository.NewCostEntryRepository(pool)
	cursors := repository.NewSourceCursorRepository(pool)
	summaries := repository.NewCostSummaryProjectionRepository(pool)
	orderFacts := repository.NewAnalyticsOrderFactRepository(pool)
	periods := repository.NewAnalyticsPeriodProjectionRepository(pool)
	state := repository.NewAnalyticsProjectionStateRepository(pool)
	dirty := repository.NewAnalyticsDirtyQueueRepository(pool)
	attributions := repository.NewVarianceAttributionRepository()
	findings := repository.NewReconciliationFindingRepository()
	mappings := repository.NewChargeCodeMappingRepository(pool)
	derived := service.NewDerivedProjectionService(pool, summaries, attributions, findings, mappings, cursors, nil, nil, metrics)
	analytics := service.NewAnalyticsProjectionService(pool, summaries, orderFacts, periods, state, dirty, metrics)
	ingest := service.NewIngestService(pool, entries, cursors, summaries, derived, analytics, metrics)
	return &analyticsEnv{pool: pool, analytics: analytics, summaries: summaries, periods: periods, ingest: ingest}
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "infrastructure", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		if strings.Compare(filepath.Base(file), maxMigrationFile) > 0 {
			break
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(raw)); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
				return err
			}
		}
	}
	return nil
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "infrastructure", "migrations")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func upsertSummary(t *testing.T, env *analyticsEnv, tenantID, buyerID, carrierID, orderID uuid.UUID, currency string, planned, current string, updatedAt time.Time) {
	t.Helper()
	plannedDec := decimal.RequireFromString(planned)
	currentDec := decimal.RequireFromString(current)
	projection := &domain.CostSummaryProjection{
		TenantID:            tenantID,
		TransportOrderID:    orderID,
		BuyerCompanyID:      buyerID,
		CarrierCompanyID:    carrierID,
		CurrencyCode:        currency,
		PlannedAmount:       &plannedDec,
		CurrentActualAmount: &currentDec,
		DataStage:           domain.DataStageCurrentActualAvailable,
		FinancialFinality:   domain.FinancialFinalityCurrentActual,
		BillingReconciliationStatus: domain.BillingReconciliationUnlinked,
		SourcesAvailable:    []string{"FREIGHT_SETTLEMENT"},
		ProjectionRevision:  1,
	}
	tx, err := env.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(context.Background())
	if err := env.summaries.Upsert(context.Background(), tx, projection); err != nil {
		t.Fatalf("upsert summary: %v", err)
	}
	if _, err := tx.Exec(context.Background(), `
		UPDATE freight_cost.cost_summary_projection
		SET updated_at = $3
		WHERE tenant_id = $1 AND transport_order_id = $2`, tenantID, orderID, updatedAt); err != nil {
		t.Fatalf("set updated_at: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
