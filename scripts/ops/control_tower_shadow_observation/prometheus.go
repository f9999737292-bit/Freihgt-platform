package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type promInstant struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func promQueryInstant(ctx context.Context, client *http.Client, baseURL, query string) (float64, error) {
	u, err := url.Parse(baseURL + "/api/v1/query")
	if err != nil {
		return 0, err
	}
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus query HTTP %d", resp.StatusCode)
	}
	var parsed promInstant
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, err
	}
	if parsed.Status != "success" || len(parsed.Data.Result) == 0 {
		return 0, nil
	}
	raw := parsed.Data.Result[0].Value[1]
	switch v := raw.(type) {
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, err
		}
		return f, nil
	case float64:
		return v, nil
	default:
		return 0, fmt.Errorf("unexpected prometheus value type")
	}
}

type MetricSnapshot struct {
	MatchTotal              float64
	MismatchTotal           float64
	LegacyRequestsTotal     float64
	ReadModelRequestsTotal  float64
	DeadLetterTotal         float64
	OffsetCommitErrorsTotal float64
	PrimaryRequestTotal     float64
	LegacyP95Seconds        float64
	ReadModelP95Seconds     float64
	Gateway5xxTotal         float64
	IncompletePartialTotal  float64
}

func collectPrometheusMetrics(ctx context.Context, client *http.Client, promURL string) (MetricSnapshot, error) {
	var snap MetricSnapshot
	var err error
	queries := []struct {
		name  string
		query string
		dest  *float64
	}{
		{"match", `sum(control_tower_read_model_shadow_comparison_total{comparison="MATCH",mode="shadow"})`, &snap.MatchTotal},
		{"mismatch", `sum(control_tower_read_model_shadow_comparison_total{comparison=~"TOTAL_MISMATCH|STATUS_COUNT_MISMATCH",mode="shadow"})`, &snap.MismatchTotal},
		{"legacy_requests", `sum(control_tower_legacy_status_aggregate_requests_total{mode="shadow"})`, &snap.LegacyRequestsTotal},
		{"read_model_requests", `sum(control_tower_read_model_requests_total{mode="shadow"})`, &snap.ReadModelRequestsTotal},
		{"dead_letter", `sum(control_tower_shipment_dead_letter_total)`, &snap.DeadLetterTotal},
		{"offset_commit_errors", `sum(control_tower_shipment_consumer_offset_commit_errors_total)`, &snap.OffsetCommitErrorsTotal},
		{"primary_requests", `sum(control_tower_read_model_requests_total{mode="primary"})`, &snap.PrimaryRequestTotal},
		{"legacy_p95", `histogram_quantile(0.95, sum by (le) (rate(control_tower_legacy_status_aggregate_duration_seconds_bucket{mode="shadow"}[5m])))`, &snap.LegacyP95Seconds},
		{"read_model_p95", `histogram_quantile(0.95, sum by (le) (rate(control_tower_read_model_request_duration_seconds_bucket{mode="shadow"}[5m])))`, &snap.ReadModelP95Seconds},
		{"gateway_5xx", `sum(increase(http_requests_total{service="api-gateway",status=~"5.."}[1h]))`, &snap.Gateway5xxTotal},
		{"partial", `sum(control_tower_read_model_partial_response_total{mode="shadow"})`, &snap.IncompletePartialTotal},
	}
	for _, q := range queries {
		*q.dest, err = promQueryInstant(ctx, client, promURL, q.query)
		if err != nil {
			return snap, fmt.Errorf("prometheus %s: %w", q.name, err)
		}
	}
	return snap, nil
}

func promAvailable(ctx context.Context, client *http.Client, promURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, promURL+"/api/v1/status/config", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus unavailable HTTP %d", resp.StatusCode)
	}
	return nil
}

func metricPresent(ctx context.Context, client *http.Client, metricsURL, name string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL+"/metrics", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	text := string(body)
	if !strings.Contains(text, name) {
		return fmt.Errorf("missing metric %s", name)
	}
	return nil
}
