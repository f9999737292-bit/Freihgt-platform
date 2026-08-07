package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/freight-platform/shipment-service/internal/platform/database"
	snap "github.com/freight-platform/statussnapshot"
	"github.com/freight-platform/shipment-service/internal/statussnapshot"
)

func main() {
	os.Exit(run())
}

func run() int {
	started := time.Now().UTC()
	var (
		scopeFlag string
		tenant    string
		batchSize int
		format    string
		output    string
	)
	flag.StringVar(&scopeFlag, "scope", "", "Snapshot scope: all")
	flag.StringVar(&tenant, "tenant", "", "Tenant UUID scope")
	flag.IntVar(&batchSize, "batch-size", statussnapshot.DefaultBatchSize, "Bounded batch size")
	flag.StringVar(&format, "format", statussnapshot.DefaultFormat, "Snapshot format")
	flag.StringVar(&output, "output", "-", "Output path (- for stdout)")
	flag.Parse()

	cfg, err := statussnapshot.LoadConfig(strings.EqualFold(scopeFlag, "all"), tenant, batchSize, format, output)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid config")
		return 2
	}

	var out io.Writer = os.Stdout
	if cfg.OutputPath != "-" {
		fmt.Fprintln(os.Stderr, "file output is not implemented")
		return 2
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, dbURL)
	if err != nil {
		log.Error("database connection failed", slog.String("error_code", statussnapshot.CodeDatabaseUnavailable))
		return 1
	}
	defer db.Close()

	repo := statussnapshot.NewPostgresSnapshotRepository(db.Pool)
	exporter := statussnapshot.NewExporter(repo, out, os.Stderr, log)
	result, err := exporter.Export(ctx, cfg)
	duration := time.Since(started)
	scope := string(cfg.Scope())
	if err != nil {
		code := statussnapshot.ExportErrorCode(err)
		if code == "" {
			code = snap.ValidationCode(err)
		}
		if code == "" {
			code = "EXPORT_FAILED"
		}
		log.Error("snapshot export failed",
			slog.String("scope", scope),
			slog.String("result", "failed"),
			slog.String("error_code", code),
			slog.Duration("duration", duration),
		)
		return 1
	}
	log.Info("snapshot export completed",
		slog.String("scope", scope),
		slog.String("result", "success"),
		slog.Int64("row_count", result.Stats.RowCount),
		slog.Int64("tenant_count", result.Stats.TenantCount),
		slog.Duration("duration", duration),
	)
	return 0
}
