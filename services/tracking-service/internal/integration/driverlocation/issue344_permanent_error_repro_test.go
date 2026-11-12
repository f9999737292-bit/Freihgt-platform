//go:build integration && issue344repro

package driverlocation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/tracking-service/internal/config"
	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/repository"
	"github.com/freight-platform/tracking-service/internal/service"
)

// TestIssue344PermanentInboxSequenceConflict records the current consumer.
// It does not implement the quarantine or generation fence proposed in PR #357.
func TestIssue344PermanentInboxSequenceConflict(t *testing.T) {
	brokers := strings.TrimSpace(os.Getenv("ISSUE344_REPRO_KAFKA_BROKERS"))
	container := strings.TrimSpace(os.Getenv("ISSUE344_REPRO_DOCKER_CONTAINER"))
	if brokers == "" || container == "" {
		t.Fatal("ISSUE344_REPRO_KAFKA_BROKERS and ISSUE344_REPRO_DOCKER_CONTAINER are required; this test does not skip or substitute a mock broker")
	}

	env := setupTestEnv(t)
	if err := applyMigrationNumbers(env.pool, 27, 91); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	execRepo := repository.NewExecutionTrackingRepository(env.pool)
	operating, execution, revision, stop := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	client := &fakeStopContext{views: map[uuid.UUID]service.StopContext{
		stop: {
			ExecutionID: execution, RevisionID: revision, ExecutionStopID: stop, Ordinal: 1,
			Status: "PLANNED", PointKind: "CANONICAL_LOCATION", StopRole: "CARGO",
		},
	}}
	apply := service.NewExecutionConsumer(execRepo, client)
	acceptedID := uuid.NewString()
	accepted := issue344Payload(t, acceptedID, operating, execution, revision, stop, 1, domain.EventRouteStopCurrent)
	if err := apply.Apply(ctx, accepted); err != nil {
		t.Fatalf("seed apply: %v", err)
	}
	before := mustIssue344Snapshot(t, ctx, env.pool, execution)
	if before.inboxCount != 1 || before.inboxEventID != acceptedID || before.sequence != 1 || before.stopID != stop {
		t.Fatalf("seed snapshot = %+v", before)
	}

	poisonID := uuid.NewString()
	poison := issue344Payload(t, poisonID, operating, execution, revision, stop, 1, domain.EventRouteStopArrived)
	firstConflict := apply.Apply(ctx, poison)
	assertSequenceConflict(t, firstConflict)
	secondConflict := apply.Apply(ctx, poison)
	assertSequenceConflict(t, secondConflict)
	afterDirect := mustIssue344Snapshot(t, ctx, env.pool, execution)
	if afterDirect != before {
		t.Fatalf("direct 23505 changed snapshot\nbefore=%+v\nafter=%+v", before, afterDirect)
	}
	var quarantine string
	if err := env.pool.QueryRow(ctx, `SELECT COALESCE(to_regclass('tracking.execution_event_quarantine')::text, 'ABSENT')`).Scan(&quarantine); err != nil {
		t.Fatal(err)
	}
	if quarantine != "ABSENT" {
		t.Fatalf("quarantine relation = %s", quarantine)
	}
	t.Logf("direct 23505 on uq_execution_event_inbox_sequence; repeated apply still conflicts; quarantine=%s", quarantine)

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	topic := "issue344-repro-" + suffix
	group := "issue344-repro-" + suffix
	rpkOrFatal(t, container, "topic", "create", topic, "-p", "1", "-r", "1")
	produceIssue344(t, brokers, topic, execution, poison)

	var attemptsA atomic.Int64
	var attemptsB atomic.Int64
	newConsumer := func(clientID string, attempts *atomic.Int64) *service.ExecutionEventConsumer {
		t.Helper()
		consumer, err := service.NewExecutionEventConsumer(config.KafkaConfig{
			Brokers:        strings.Split(brokers, ","),
			ExecutionTopic: topic,
			GroupID:        group,
			ClientID:       clientID,
		}, func(ctx context.Context, raw []byte) error {
			if string(raw) != string(poison) {
				return fmt.Errorf("consumer received unexpected payload")
			}
			attempts.Add(1)
			return apply.Apply(ctx, raw)
		})
		if err != nil {
			t.Fatal(err)
		}
		return consumer
	}

	ctxA, cancelA := context.WithCancel(context.Background())
	consumerA := newConsumer("issue344-a", &attemptsA)
	doneA := make(chan struct{})
	go func() {
		consumerA.Run(ctxA)
		close(doneA)
	}()
	t.Cleanup(func() {
		cancelA()
		<-doneA
		consumerA.Close()
	})
	waitIssue344(t, 30*time.Second, "consumer A retries the permanent error", func() (bool, string) {
		described := rpkOrLog(t, container, "group", "describe", group)
		return attemptsA.Load() >= 3 && described.offset == "-" && described.found, fmt.Sprintf("attempts=%d describe=%s", attemptsA.Load(), described.raw)
	})
	midA := mustIssue344Snapshot(t, ctx, env.pool, execution)
	if midA != before {
		t.Fatalf("consumer retry changed snapshot\nbefore=%+v\nafter=%+v", before, midA)
	}

	ctxB, cancelB := context.WithCancel(context.Background())
	consumerB := newConsumer("issue344-b", &attemptsB)
	doneB := make(chan struct{})
	go func() {
		consumerB.Run(ctxB)
		close(doneB)
	}()
	t.Cleanup(func() {
		cancelB()
		<-doneB
		consumerB.Close()
	})
	time.Sleep(3 * time.Second)
	whileBoth := rpkOrLog(t, container, "group", "describe", group)
	t.Logf("while A is blocked and B has joined: attemptsA=%d attemptsB=%d\n%s", attemptsA.Load(), attemptsB.Load(), whileBoth.raw)

	stoppedA := attemptsA.Load()
	cancelA()
	select {
	case <-doneA:
	case <-time.After(20 * time.Second):
		t.Fatal("consumer A did not return after cancel")
	}
	consumerA.Close()
	time.Sleep(300 * time.Millisecond)
	if attemptsA.Load() > stoppedA+1 {
		t.Fatalf("consumer A kept applying after cancel: before=%d after=%d", stoppedA, attemptsA.Load())
	}

	waitIssue344(t, 45*time.Second, "consumer B receives the uncommitted record", func() (bool, string) {
		described := rpkOrLog(t, container, "group", "describe", group)
		return attemptsB.Load() >= 2 && described.offset == "-" && strings.Contains(described.client, "issue344-b"),
			fmt.Sprintf("attemptsB=%d client=%q offset=%q\n%s", attemptsB.Load(), described.client, described.offset, described.raw)
	})
	finalDescribe := rpkOrLog(t, container, "group", "describe", group)
	finalSnap := mustIssue344Snapshot(t, ctx, env.pool, execution)
	if finalSnap != before {
		t.Fatalf("rebalance changed snapshot\nbefore=%+v\nafter=%+v", before, finalSnap)
	}
	if finalDescribe.logEnd == "" || finalDescribe.logEnd == "0" {
		t.Fatalf("record is not in the log: %s", finalDescribe.raw)
	}
	t.Logf("after A left: attemptsA=%d attemptsB=%d offset=%s log_end=%s client=%s", attemptsA.Load(), attemptsB.Load(), finalDescribe.offset, finalDescribe.logEnd, finalDescribe.client)
	t.Log("observed: no quarantine row, offset stayed uncommitted, and the next member retried the same payload. PR #357 would open a fence and then commit; that path is not implemented here.")
}

func issue344Payload(t *testing.T, eventID string, operating, execution, revision, stop uuid.UUID, sequence int64, eventType string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"event_id": eventID, "event_type": eventType, "operating_tenant_id": operating.String(),
		"execution_id": execution.String(), "revision_id": revision.String(), "revision_version": 1,
		"event_sequence": sequence, "stop_id": stop.String(), "occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assertSequenceConflict(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected pg error, got %v", err)
	}
	if pgErr.Code != "23505" || pgErr.ConstraintName != "uq_execution_event_inbox_sequence" {
		t.Fatalf("code=%s constraint=%s", pgErr.Code, pgErr.ConstraintName)
	}
}

type issue344Snapshot struct {
	sequence     int64
	stopID       uuid.UUID
	updatedAt    time.Time
	inboxCount   int
	inboxEventID string
}

func mustIssue344Snapshot(t *testing.T, ctx context.Context, pool *pgxpool.Pool, execution uuid.UUID) issue344Snapshot {
	t.Helper()
	var snap issue344Snapshot
	if err := pool.QueryRow(ctx, `
SELECT last_execution_event_sequence, current_stop_id, updated_at
FROM tracking.execution_tracking_state
WHERE execution_id=$1`, execution).Scan(&snap.sequence, &snap.stopID, &snap.updatedAt); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*), COALESCE(MIN(event_id::text), '')
FROM tracking.execution_event_inbox
WHERE execution_id=$1`, execution).Scan(&snap.inboxCount, &snap.inboxEventID); err != nil {
		t.Fatal(err)
	}
	return snap
}

func produceIssue344(t *testing.T, brokers, topic string, execution uuid.UUID, payload []byte) {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(brokers, ",")...))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte(execution.String()), Value: payload}).FirstErr(); err != nil {
		t.Fatal(err)
	}
}

type issue344Group struct {
	raw     string
	offset  string
	logEnd  string
	client  string
	found   bool
	members string
	state   string
}

func rpkOrFatal(t *testing.T, container string, args ...string) issue344Group {
	t.Helper()
	described, err := runRPK(container, args...)
	if err != nil {
		t.Fatalf("rpk %s: %v\n%s", strings.Join(args, " "), err, described.raw)
	}
	return described
}

func rpkOrLog(t *testing.T, container string, args ...string) issue344Group {
	t.Helper()
	described, err := runRPK(container, args...)
	if err != nil {
		described.raw = fmt.Sprintf("rpk error: %v\n%s", err, described.raw)
	}
	return described
}

func runRPK(container string, args ...string) (issue344Group, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmdArgs := append([]string{"exec", container, "rpk"}, args...)
	cmdArgs = append(cmdArgs, "-X", "brokers=127.0.0.1:9092")
	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	out, err := cmd.CombinedOutput()
	return parseRPKGroup(string(out)), err
}

func parseRPKGroup(output string) issue344Group {
	view := issue344Group{raw: output}
	lines := strings.Split(output, "\n")
	header := -1
	var headers []string
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && header < 0 {
			switch fields[0] {
			case "STATE":
				view.state = fields[1]
			case "MEMBERS":
				view.members = fields[1]
			}
		}
		if strings.Contains(line, "CURRENT-OFFSET") {
			header = i
			headers = fields
			break
		}
	}
	if header < 0 {
		return view
	}
	col := map[string]int{}
	for i, name := range headers {
		col[name] = i
	}
	for _, line := range lines[header+1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 || !strings.HasPrefix(fields[0], "issue344-repro-") {
			continue
		}
		view.found = true
		view.offset = rpkField(fields, col["CURRENT-OFFSET"])
		view.logEnd = rpkField(fields, col["LOG-END-OFFSET"])
		view.client = rpkField(fields, col["CLIENT-ID"])
	}
	return view
}

func rpkField(fields []string, index int) string {
	if index < 0 || index >= len(fields) {
		return ""
	}
	return fields[index]
}

func waitIssue344(t *testing.T, timeout time.Duration, what string, ready func() (bool, string)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	last := ""
	for time.Now().Before(deadline) {
		ok, detail := ready()
		last = detail
		if ok {
			t.Logf("%s\n%s", what, detail)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("%s timed out\n%s", what, last)
}
