package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/platform/database"
	"github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
	"github.com/freight-platform/statussnapshot"
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		stdin      bool
		dryRun     bool
		activate   bool
		status     bool
		cleanup    bool
		rollback   bool
		snapshotID string
		batchSize  int
	)
	flag.BoolVar(&stdin, "stdin", false, "Read snapshot from stdin")
	flag.BoolVar(&dryRun, "dry-run", false, "Validate snapshot without persisting")
	flag.BoolVar(&activate, "activate", false, "Activate validated snapshot")
	flag.BoolVar(&status, "status", false, "Show rebuild job status")
	flag.BoolVar(&cleanup, "cleanup", false, "Cleanup staging rows")
	flag.BoolVar(&rollback, "rollback", false, "Rollback activation")
	flag.StringVar(&snapshotID, "snapshot-id", "", "Snapshot job UUID")
	flag.IntVar(&batchSize, "batch-size", rebuild.DefaultBatchSize, "Bounded batch insert size")
	flag.Parse()

	if activate {
		if !strings.EqualFold(os.Getenv("CONFIRM_PROJECTION_REBUILD_ACTIVATION"), "true") {
			fmt.Fprintln(os.Stderr, "ACTIVATION_CONFIRMATION_REQUIRED")
			return 2
		}
		fmt.Fprintln(os.Stderr, rebuild.ErrActivationForbidden.Error())
		return 2
	}

	cfg, err := rebuild.LoadConfig(stdin, dryRun, activate, status, cleanup, rollback, snapshotID, batchSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid config")
		return 2
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var repo rebuild.RebuildRepository
	if dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL")); dbURL != "" {
		db, err := database.Connect(ctx, dbURL)
		if err != nil {
			fmt.Fprintln(os.Stderr, "database connection failed")
			return 1
		}
		defer db.Close()
		repo = rebuild.NewRepository(db.Pool)
	}

	if cfg.Status {
		if repo == nil {
			fmt.Fprintln(os.Stderr, "DATABASE_URL is required for --status")
			return 2
		}
		id, err := uuid.Parse(cfg.SnapshotID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid snapshot-id")
			return 2
		}
		job, err := repo.GetJobStatus(ctx, id)
		if err != nil {
			log.Error("status lookup failed", slog.String("error_code", "STATUS_FAILED"))
			return 1
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rebuild.JobStatusToResponse(job)); err != nil {
			return 1
		}
		return 0
	}

	if cfg.DryRun {
		report, err := rebuild.NewImporter(repo).DryRun(ctx, os.Stdin)
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(report)
		if err != nil {
			log.Error("dry-run failed", slog.String("error_code", rebuildSafeCode(err)))
			return 1
		}
		return 0
	}

	if repo == nil {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required for import")
		return 2
	}
	if err := rebuild.NewImporter(repo).Import(ctx, os.Stdin, cfg.BatchSize); err != nil {
		log.Error("import failed", slog.String("error_code", rebuildSafeCode(err)))
		return 1
	}
	log.Info("import validated")
	return 0
}

func rebuildSafeCode(err error) string {
	if code := statussnapshot.ValidationCode(err); code != "" {
		return code
	}
	return "IMPORT_FAILED"
}

var _ = io.Discard
