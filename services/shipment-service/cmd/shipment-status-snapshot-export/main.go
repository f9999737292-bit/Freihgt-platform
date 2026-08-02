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

	"github.com/freight-platform/shipment-service/internal/platform/database"
	"github.com/freight-platform/shipment-service/internal/statussnapshot"
)

func main() {
	os.Exit(run())
}

func run() int {
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
		fmt.Fprintln(os.Stderr, "file output is not implemented in v0.1 core infrastructure")
		return 2
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if helpRequested := flag.Args(); len(helpRequested) == 0 && os.Getenv("DATABASE_URL") == "" {
		// allow --help without DB
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	repo := statussnapshot.SnapshotRepository(statussnapshot.NotImplementedRepository{})
	if dbURL != "" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		db, err := database.Connect(ctx, dbURL)
		if err != nil {
			fmt.Fprintln(os.Stderr, "database connection failed")
			return 1
		}
		defer db.Close()
		_ = db
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exporter := statussnapshot.NewExporter(repo, out, os.Stderr, log)
	_, err = exporter.Export(ctx, cfg)
	if err != nil {
		if err == statussnapshot.ErrNotImplemented {
			log.Error("export query not implemented", slog.String("error_code", "NOT_IMPLEMENTED_EXPORT_QUERY"))
		} else {
			log.Error("snapshot export failed", slog.String("error_code", safeCode(err)))
		}
		return 1
	}
	return 0
}

func safeCode(err error) string {
	if err == statussnapshot.ErrUnknownStatus {
		return "UNKNOWN_STATUS"
	}
	return "EXPORT_FAILED"
}
