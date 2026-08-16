package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/outboxreplay"
	"github.com/freight-platform/shipment-service/internal/platform/database"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		tenantID      string
		aggregateIDs  multiFlag
		eventIDs      multiFlag
		eventIDsFile  string
		execute       bool
	)
	flag.StringVar(&tenantID, "tenant-id", "", "Required tenant UUID scope")
	flag.Var(&aggregateIDs, "aggregate-id", "Shipment aggregate UUID (repeatable)")
	flag.Var(&eventIDs, "event-id", "Outbox event UUID (repeatable)")
	flag.StringVar(&eventIDsFile, "event-ids-file", "", "Optional file with one outbox event UUID per line")
	flag.BoolVar(&execute, "execute", false, "Apply replay after validation (default dry-run)")
	flag.Parse()

	if strings.TrimSpace(tenantID) == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --tenant-id is required")
		return 2
	}
	parsedTenant, err := uuid.Parse(tenantID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: invalid --tenant-id")
		return 2
	}

	fileEventIDs, err := readEventIDsFile(eventIDsFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err.Error())
		return 2
	}
	allEventIDs := append([]uuid.UUID(nil), eventIDs...)
	allEventIDs = append(allEventIDs, fileEventIDs...)

	if len(allEventIDs) == 0 && len(aggregateIDs) == 0 {
		fmt.Fprintln(os.Stderr, "ERROR: at least one --aggregate-id, --event-id, or --event-ids-file entry is required")
		return 2
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "ERROR: DATABASE_URL is required")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	db, err := database.Connect(ctx, dbURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: database connection failed")
		return 1
	}
	defer db.Close()

	svc := outboxreplay.NewService(repository.NewShipmentRepository(db.Pool))
	result, err := svc.ReplayFailedOutbox(ctx, outboxreplay.Request{
		TenantID:     parsedTenant,
		EventIDs:     allEventIDs,
		AggregateIDs: aggregateIDs,
		Execute:      execute,
		Now:          time.Now().UTC(),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
		return 1
	}

	mode := "DRY_RUN"
	if execute {
		mode = "EXECUTE"
	}
	fmt.Printf("MODE=%s\n", mode)
	fmt.Printf("MATCHED_COUNT=%d\n", len(result.Preview))
	for _, row := range result.Preview {
		fmt.Printf(
			"EVENT_ID=%s TENANT_ID=%s AGGREGATE_ID=%s EVENT_TYPE=%s CURRENT_STATUS=%s ATTEMPT_COUNT=%d LAST_ERROR_CODE=%s\n",
			row.EventID,
			row.TenantID,
			row.AggregateID,
			row.EventType,
			row.Status,
			row.AttemptCount,
			emptyDash(row.LastErrorCode),
		)
	}
	if execute {
		fmt.Printf("AFFECTED_COUNT=%d\n", result.AffectedCount)
	}
	return 0
}

type multiFlag []uuid.UUID

func (m *multiFlag) String() string {
	return fmt.Sprintf("%d ids", len(*m))
}

func (m *multiFlag) Set(value string) error {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("invalid uuid %q", value)
	}
	*m = append(*m, parsed)
	return nil
}

func readEventIDsFile(path string) ([]uuid.UUID, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open event ids file: %w", err)
	}
	defer file.Close()

	var ids []uuid.UUID
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parsed, err := uuid.Parse(line)
		if err != nil {
			return nil, fmt.Errorf("invalid uuid on line %d", lineNo)
		}
		ids = append(ids, parsed)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read event ids file: %w", err)
	}
	return ids, nil
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
