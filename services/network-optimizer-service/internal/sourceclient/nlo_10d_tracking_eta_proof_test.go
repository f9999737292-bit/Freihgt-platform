package sourceclient

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/lowcode"
)

const harnessToken = "nlo-1.0d-harness-token"

var (
	callerTenant  = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1")
	foreignTenant = uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1")
	shipmentKnown = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	shipmentZero  = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	shipmentNull  = uuid.MustParse("00000000-0000-4000-8000-000000000003")
	shipmentMiss  = uuid.MustParse("00000000-0000-4000-8000-000000000004")
	shipmentNoETA = uuid.MustParse("00000000-0000-4000-8000-000000000005")
)

func TestNLO10D_TrackingAndETARealHandlers(t *testing.T) {
	h := startTrackingHarness(t)
	if !strings.HasPrefix(h.url, "http://127.0.0.1:") {
		t.Fatalf("harness is not loopback: %s", h.url)
	}
	client := New("", h.url, harnessToken)
	ctx := context.Background()

	t.Run("fresh coordinates", func(t *testing.T) {
		got, err := client.TrackingState(ctx, callerTenant, shipmentKnown)
		if err != nil {
			t.Fatal(err)
		}
		if got.TrackingStatus != "active" || got.Freshness != "fresh" {
			t.Fatalf("status %s freshness %s", got.TrackingStatus, got.Freshness)
		}
		near(t, got.Latitude, 55.751244)
		near(t, got.Longitude, 37.618423)
		if got.RecordedAt == nil || !got.RecordedAt.UTC().Equal(h.recordedAt) {
			t.Fatalf("recorded at %+v want %s", got.RecordedAt, h.recordedAt)
		}
		if got.AgeSeconds == nil || *got.AgeSeconds < 60 || *got.AgeSeconds > 600 {
			t.Fatalf("age was not recomputed from the timestamp: %+v", got.AgeSeconds)
		}
	})

	t.Run("zero latitude", func(t *testing.T) {
		got, err := client.TrackingState(ctx, callerTenant, shipmentZero)
		if err != nil {
			t.Fatal(err)
		}
		if got.Latitude == nil || *got.Latitude != 0 {
			t.Fatalf("zero latitude became %+v", got.Latitude)
		}
		near(t, got.Longitude, 37.618423)
	})

	t.Run("null coordinates", func(t *testing.T) {
		got, err := client.TrackingState(ctx, callerTenant, shipmentNull)
		if err != nil {
			t.Fatal(err)
		}
		if got.Latitude != nil || got.Longitude != nil || got.RecordedAt != nil {
			t.Fatalf("null coordinates became %+v %+v %+v", got.Latitude, got.Longitude, got.RecordedAt)
		}
		if got.Freshness != "unknown" {
			t.Fatalf("freshness %s", got.Freshness)
		}
	})

	t.Run("missing shipment", func(t *testing.T) {
		got, err := client.TrackingState(ctx, callerTenant, shipmentMiss)
		if err != nil {
			t.Fatal(err)
		}
		if got.TrackingStatus != "not_configured" || got.Freshness != "unknown" {
			t.Fatalf("missing row %+v", got)
		}
		if got.Latitude != nil || got.Longitude != nil || got.RecordedAt != nil {
			t.Fatalf("missing row became a position %+v", got)
		}
	})

	t.Run("eta arrival", func(t *testing.T) {
		got, err := client.TrackingETA(ctx, callerTenant, shipmentKnown)
		if err != nil {
			t.Fatal(err)
		}
		want := time.Date(2026, 11, 12, 18, 0, 0, 0, time.UTC)
		if got.Status != "available" || got.FreshnessStatus != "fresh" {
			t.Fatalf("eta status %s freshness %s", got.Status, got.FreshnessStatus)
		}
		if got.EstimatedArrivalAt == nil || !got.EstimatedArrivalAt.UTC().Equal(want) {
			t.Fatalf("arrival %+v", got.EstimatedArrivalAt)
		}
		if got.SourceObservedAt == nil || !got.SourceObservedAt.UTC().Equal(h.observedAt) {
			t.Fatalf("observed %+v", got.SourceObservedAt)
		}
	})

	t.Run("eta null arrival", func(t *testing.T) {
		got, err := client.TrackingETA(ctx, callerTenant, shipmentNoETA)
		if err != nil {
			t.Fatal(err)
		}
		if got.EstimatedArrivalAt != nil || got.SourceObservedAt != nil {
			t.Fatalf("null arrival became %+v %+v", got.EstimatedArrivalAt, got.SourceObservedAt)
		}
		if got.Status != "unavailable" || got.FreshnessStatus != "unknown" {
			t.Fatalf("null eta row %+v", got)
		}
	})

	t.Run("eta missing unavailable", func(t *testing.T) {
		got, err := client.TrackingETA(ctx, callerTenant, shipmentMiss)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != "unavailable" || got.FreshnessStatus != "unknown" || got.EstimatedArrivalAt != nil {
			t.Fatalf("missing eta %+v", got)
		}
	})

	t.Run("cross tenant tracking", func(t *testing.T) {
		caller, err := client.TrackingState(ctx, callerTenant, shipmentKnown)
		if err != nil {
			t.Fatal(err)
		}
		foreign, err := client.TrackingState(ctx, foreignTenant, shipmentKnown)
		if err != nil {
			t.Fatal(err)
		}
		near(t, caller.Latitude, 55.751244)
		near(t, caller.Longitude, 37.618423)
		near(t, foreign.Latitude, 59.939095)
		near(t, foreign.Longitude, 30.315868)
	})

	t.Run("cross tenant eta", func(t *testing.T) {
		caller, err := client.TrackingETA(ctx, callerTenant, shipmentKnown)
		if err != nil {
			t.Fatal(err)
		}
		foreign, err := client.TrackingETA(ctx, foreignTenant, shipmentKnown)
		if err != nil {
			t.Fatal(err)
		}
		callerWant := time.Date(2026, 11, 12, 18, 0, 0, 0, time.UTC)
		foreignWant := time.Date(2026, 11, 12, 23, 0, 0, 0, time.UTC)
		if caller.EstimatedArrivalAt == nil || !caller.EstimatedArrivalAt.UTC().Equal(callerWant) {
			t.Fatalf("caller arrival %+v", caller.EstimatedArrivalAt)
		}
		if foreign.EstimatedArrivalAt == nil || !foreign.EstimatedArrivalAt.UTC().Equal(foreignWant) {
			t.Fatalf("foreign arrival %+v", foreign.EstimatedArrivalAt)
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		status := postLookup(t, h.url, "not-the-harness-token", callerTenant.String())
		if status != http.StatusUnauthorized {
			t.Fatalf("status %d", status)
		}
	})

	t.Run("missing tenant", func(t *testing.T) {
		status := postLookup(t, h.url, harnessToken, "")
		if status != http.StatusBadRequest {
			t.Fatalf("status %d", status)
		}
	})
}

func near(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatal("coordinate is nil")
	}
	if math.Abs(*got-want) > 1e-5 {
		t.Fatalf("got %v want %v", *got, want)
	}
}

func postLookup(t *testing.T, baseURL, token, tenant string) int {
	t.Helper()
	body := bytes.NewBufferString(`{"shipmentIds":["00000000-0000-4000-8000-000000000001"]}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/internal/v1/tracking/states/lookup", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set(internalauth.HeaderName, token)
	}
	if tenant != "" {
		req.Header.Set(lowcode.HeaderTenantID, tenant)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

type trackingHarness struct {
	url        string
	recordedAt time.Time
	observedAt time.Time
}

func startTrackingHarness(t *testing.T) trackingHarness {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "tracking-service"))
	cmd := exec.Command("go", "run", "./cmd/nlo10d-contract-harness")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stop := func() {
		_ = stdin.Close()
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(20 * time.Second):
			_ = cmd.Process.Kill()
		}
	}
	t.Cleanup(stop)
	reader := bufio.NewReader(stdout)
	type lineResult struct {
		line string
		err  error
	}
	lines := make(chan lineResult, 8)
	go func() {
		for {
			line, err := reader.ReadString('\n')
			lines <- lineResult{line: line, err: err}
			if err != nil {
				return
			}
		}
	}()
	var url, recordedRaw, observedRaw string
	deadline := time.NewTimer(3 * time.Minute)
	defer deadline.Stop()
	for url == "" || recordedRaw == "" || observedRaw == "" {
		select {
		case res := <-lines:
			line := strings.TrimSpace(res.line)
			switch {
			case strings.HasPrefix(line, "NLO10D_URL="):
				url = strings.TrimPrefix(line, "NLO10D_URL=")
			case strings.HasPrefix(line, "NLO10D_RECORDED_AT="):
				recordedRaw = strings.TrimPrefix(line, "NLO10D_RECORDED_AT=")
			case strings.HasPrefix(line, "NLO10D_OBSERVED_AT="):
				observedRaw = strings.TrimPrefix(line, "NLO10D_OBSERVED_AT=")
			}
			if res.err != nil && (url == "" || recordedRaw == "" || observedRaw == "") {
				stop()
				t.Fatalf("harness exited: %v %s", res.err, stderr.String())
			}
		case <-deadline.C:
			stop()
			t.Fatalf("harness startup timed out: %s", stderr.String())
		}
	}
	go io.Copy(io.Discard, reader)
	recordedAt, err := time.Parse(time.RFC3339, recordedRaw)
	if err != nil {
		t.Fatal(err)
	}
	observedAt, err := time.Parse(time.RFC3339, observedRaw)
	if err != nil {
		t.Fatal(err)
	}
	return trackingHarness{url: url, recordedAt: recordedAt, observedAt: observedAt}
}
