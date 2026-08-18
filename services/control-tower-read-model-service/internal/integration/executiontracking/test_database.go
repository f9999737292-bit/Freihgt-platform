//go:build integration

package executiontracking

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	ctprojection "github.com/freight-platform/control-tower-read-model-service/internal/projection"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

const maxMigrationNumber = 41

var testKafkaOffset int64

func nextTestKafkaOffset() int64 {
	return atomic.AddInt64(&testKafkaOffset, 1)
}

type ctEnv struct {
	pool *pgxpool.Pool
	repo *repository.ProjectionRepository
}

func setupCTEnv(t *testing.T) *ctEnv {
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
	if err := applyCTMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return &ctEnv{pool: pool, repo: repository.NewProjectionRepository(pool)}
}

func applyCTMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := locateCTMigrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		base := filepath.Base(file)
		num := 0
		if _, scanErr := fmt.Sscanf(base, "%d", &num); scanErr != nil {
			return scanErr
		}
		if num > maxMigrationNumber {
			continue
		}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(content)); execErr != nil {
			msg := execErr.Error()
			if strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate key") {
				continue
			}
			return fmt.Errorf("%s: %w", base, execErr)
		}
	}
	return nil
}

func locateCTMigrationsDir() (string, error) {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	return "", os.ErrNotExist
}

func processOutboxPayload(t *testing.T, env *ctEnv, payload []byte, tenantID, shipmentID uuid.UUID) {
	t.Helper()
	event, permErr := ctprojection.ParseAndValidate(payload, domain.KafkaRecordMeta{
		Topic: "shipment.status.v1", Key: shipmentID.String(),
	}, "shipment.status.v1")
	if permErr != nil {
		t.Fatalf("parse outbox: %v", permErr)
	}
	result, err := env.repo.ProcessEvent(context.Background(), repository.ProcessInput{
		Event: event,
		Meta: domain.KafkaRecordMeta{
			Topic: "shipment.status.v1", Partition: 0, Offset: nextTestKafkaOffset(), Key: shipmentID.String(),
		},
		ReceivedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("process event: %v", err)
	}
	if result.Outcome != domain.OutcomeApplied && result.Outcome != domain.OutcomeGapApplied {
		t.Fatalf("unexpected outcome: %s (duplicate=%v)", result.Outcome, result.Duplicate)
	}
	if !result.Applied && !result.Duplicate {
		t.Fatalf("event not applied: outcome=%s duplicate=%v", result.Outcome, result.Duplicate)
	}
	if event.TenantID != tenantID {
		t.Fatalf("tenant mismatch")
	}
}
