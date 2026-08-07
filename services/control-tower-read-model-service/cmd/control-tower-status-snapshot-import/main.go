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
	flag.BoolVar(&activate, "activate", false, "Activate validated snapshot (requires CONFIRM_PROJECTION_REBUILD_ACTIVATION=true)")
	flag.BoolVar(&status, "status", false, "Show rebuild job status")
	flag.BoolVar(&cleanup, "cleanup", false, "Cleanup staging/backup rows (requires CONFIRM_PROJECTION_REBUILD_CLEANUP=true)")
	flag.BoolVar(&rollback, "rollback", false, "Rollback activation (requires CONFIRM_PROJECTION_REBUILD_ROLLBACK=true)")
	flag.StringVar(&snapshotID, "snapshot-id", "", "Snapshot job UUID")
	flag.IntVar(&batchSize, "batch-size", rebuild.DefaultBatchSize, "Bounded batch insert size")
	flag.Parse()

	cfg, err := rebuild.LoadConfig(stdin, dryRun, activate, status, cleanup, rollback, snapshotID, batchSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid config")
		return 2
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var repo rebuild.RebuildRepository
	var activationRepo rebuild.ActivationRepository
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		dbURL = strings.TrimSpace(os.Getenv("CONTROL_TOWER_DATABASE_URL"))
	}
	if dbURL != "" {
		db, err := database.Connect(ctx, dbURL)
		if err != nil {
			fmt.Fprintln(os.Stderr, "database connection failed")
			return 1
		}
		defer db.Close()
		repo = rebuild.NewRepository(db.Pool)
		activationRepo = rebuild.NewActivationRepository(db.Pool)
	}

	if cfg.Activate {
		if !strings.EqualFold(os.Getenv("CONFIRM_PROJECTION_REBUILD_ACTIVATION"), "true") {
			fmt.Fprintln(os.Stderr, rebuild.CodeActivationConfirmationRequired)
			return 2
		}
		if activationRepo == nil {
			fmt.Fprintln(os.Stderr, "DATABASE_URL is required for --activate")
			return 2
		}
		id, err := uuid.Parse(cfg.SnapshotID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid snapshot-id")
			return 2
		}
		result, err := activationRepo.Activate(ctx, id)
		if err != nil {
			code := activationSafeCode(err)
			log.Error("activation failed", slog.String("operation", "activate"), slog.String("safe_error_code", code))
			fmt.Fprintln(os.Stderr, code)
			return 1
		}
		out := map[string]any{
			"state":            result.State,
			"scope":            result.Scope,
			"activatedRows":    result.ActivatedRows,
			"backupRows":       result.BackupRows,
			"rollbackEligible": result.RollbackEligible,
			"activatedAt":      result.ActivatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		log.Info("activation complete",
			slog.String("operation", "activate"),
			slog.String("scope", result.Scope),
			slog.String("state", result.State),
			slog.Int64("rows", result.ActivatedRows),
			slog.String("result", "success"),
		)
		return 0
	}

	if cfg.Rollback {
		if !strings.EqualFold(os.Getenv("CONFIRM_PROJECTION_REBUILD_ROLLBACK"), "true") {
			fmt.Fprintln(os.Stderr, rebuild.CodeRollbackConfirmationRequired)
			return 2
		}
		if activationRepo == nil {
			fmt.Fprintln(os.Stderr, "DATABASE_URL is required for --rollback")
			return 2
		}
		id, err := uuid.Parse(cfg.SnapshotID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid snapshot-id")
			return 2
		}
		result, err := activationRepo.Rollback(ctx, id)
		if err != nil {
			code := activationSafeCode(err)
			log.Error("rollback failed", slog.String("operation", "rollback"), slog.String("safe_error_code", code))
			fmt.Fprintln(os.Stderr, code)
			return 1
		}
		out := map[string]any{
			"state":        result.State,
			"restoredRows": result.RestoredRows,
			"rolledBackAt": result.RolledBackAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		log.Info("rollback complete",
			slog.String("operation", "rollback"),
			slog.String("state", result.State),
			slog.Int64("rows", result.RestoredRows),
			slog.String("result", "success"),
		)
		return 0
	}

	if cfg.Cleanup {
		if !strings.EqualFold(os.Getenv("CONFIRM_PROJECTION_REBUILD_CLEANUP"), "true") {
			fmt.Fprintln(os.Stderr, rebuild.CodeCleanupConfirmationRequired)
			return 2
		}
		if activationRepo == nil {
			fmt.Fprintln(os.Stderr, "DATABASE_URL is required for --cleanup")
			return 2
		}
		id, err := uuid.Parse(cfg.SnapshotID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid snapshot-id")
			return 2
		}
		result, err := activationRepo.Cleanup(ctx, id)
		if err != nil {
			code := activationSafeCode(err)
			log.Error("cleanup failed", slog.String("operation", "cleanup"), slog.String("safe_error_code", code))
			fmt.Fprintln(os.Stderr, code)
			return 1
		}
		out := map[string]any{
			"state":             result.State,
			"stageRowsRemoved":  result.StageRowsRemoved,
			"backupRowsRemoved": result.BackupRowsRemoved,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return 0
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
	if !strings.EqualFold(os.Getenv("CONFIRM_PROJECTION_REBUILD_IMPORT"), "true") {
		fmt.Fprintln(os.Stderr, "CONFIRM_PROJECTION_REBUILD_IMPORT=true is required for persistent import")
		return 2
	}
	if err := rebuild.NewImporter(repo).Import(ctx, os.Stdin, cfg.BatchSize); err != nil {
		log.Error("import failed", slog.String("error_code", rebuildSafeCode(err)))
		return 1
	}
	result := map[string]string{"state": rebuild.StateValidated, "result": "validated"}
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(result)
	log.Info("import validated")
	return 0
}

func rebuildSafeCode(err error) string {
	if code := statussnapshot.ValidationCode(err); code != "" {
		return code
	}
	if code := rebuild.ImportErrorCode(err); code != "" {
		return code
	}
	return "IMPORT_FAILED"
}

func activationSafeCode(err error) string {
	if code := rebuild.ActivationErrorCode(err); code != "" {
		return code
	}
	return "ACTIVATION_FAILED"
}

var _ = io.Discard
