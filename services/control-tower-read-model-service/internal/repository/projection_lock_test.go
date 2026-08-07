package repository

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProcessEventAcquiresSharedAdvisoryLockBeforeProjection(t *testing.T) {
	t.Parallel()
	src := readRepositoryGoSource(t)
	lockIdx := strings.Index(src, "AcquireProjectionSharedLock")
	inboxIdx := strings.Index(src, "findDuplicateInbox")
	projIdx := strings.Index(src, "lockProjection(ctx, tx")
	if lockIdx == -1 {
		t.Fatal("ProcessEvent must acquire shared advisory lock")
	}
	if inboxIdx == -1 || projIdx == -1 {
		t.Fatal("ProcessEvent must read inbox and projection")
	}
	if lockIdx > inboxIdx || lockIdx > projIdx {
		t.Fatal("shared advisory lock must be acquired before inbox dedupe and projection read")
	}
}

func TestUpsertProjectionClearsSnapshotProvenanceOnLiveWrite(t *testing.T) {
	t.Parallel()
	src := strings.ToLower(readRepositoryGoSource(t))
	if !strings.Contains(src, "snapshot_id = null") {
		t.Fatal("live projection write must clear snapshot_id")
	}
	if !strings.Contains(src, "authoritative_as_of = null") {
		t.Fatal("live projection write must clear authoritative_as_of")
	}
	if !strings.Contains(src, "projection_source") {
		t.Fatal("live projection write must set projection_source")
	}
}

func TestActivationPackageDoesNotImportKafkaClient(t *testing.T) {
	t.Parallel()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller failed")
	}
	rebuildDir := filepath.Join(filepath.Dir(thisFile), "..", "rebuild")
	entries, err := os.ReadDir(rebuildDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(rebuildDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		if strings.Contains(src, "franz-go") || strings.Contains(src, "kgo.") {
			t.Fatalf("rebuild package must not import Kafka client: %s", e.Name())
		}
	}
}

func readRepositoryGoSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller failed")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "projection_repository.go"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
