//go:build integration

package studio

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const scoringModelGatewayReadyTimeout = 120 * time.Second

func verifyScoringGatewayProbe(t *testing.T, stack *browserScoringLiveStack) {
	t.Helper()
	if err := scoringModelGatewayReady(t.Context(), stack.gatewayURL, stack.fixture, scoringModelGatewayReadyTimeout); err != nil {
		t.Fatal(err)
	}
}

func waitForScoringModelGatewayReady(t *testing.T, gatewayURL string, fix browserScoringFixture, timeout time.Duration) {
	t.Helper()
	if err := scoringModelGatewayReady(t.Context(), gatewayURL, fix, timeout); err != nil {
		t.Fatal(err)
	}
}

func scoringModelGatewayReady(ctx context.Context, gatewayURL string, fix browserScoringFixture, timeout time.Duration) error {
	url := strings.TrimRight(gatewayURL, "/") + "/api/v1/rfx-events/" + fix.EventID.String() + "/score-model"
	deadline := time.Now().Add(timeout)
	var lastStatus int
	var lastBody string
	var lastErr error
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("scoring model gateway readiness cancelled: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("scoring model readiness request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+fix.JWT)
		req.Header.Set("X-Company-ID", fix.CompanyID.String())
		req.Header.Set("Accept", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			if waitErr := waitRetryInterval(ctx, 500*time.Millisecond); waitErr != nil {
				return fmt.Errorf("scoring model gateway readiness cancelled: %w", waitErr)
			}
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
		lastStatus = resp.StatusCode
		lastBody = truncateProbeBody(string(body))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("scoring model gateway readiness failed (non-retryable): status=%d url=%s body=%s", lastStatus, url, lastBody)
		}
		if waitErr := waitRetryInterval(ctx, 500*time.Millisecond); waitErr != nil {
			return fmt.Errorf("scoring model gateway readiness cancelled: %w", waitErr)
		}
	}
	if lastErr != nil {
		return fmt.Errorf("scoring model gateway readiness failed after %s: lastErr=%v lastStatus=%d url=%s body=%s", timeout, lastErr, lastStatus, url, lastBody)
	}
	return fmt.Errorf("scoring model gateway readiness failed after %s: lastStatus=%d url=%s body=%s", timeout, lastStatus, url, lastBody)
}

func waitRetryInterval(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func truncateProbeBody(body string) string {
	body = strings.TrimSpace(body)
	if len(body) <= 240 {
		return body
	}
	return body[:240] + "..."
}
