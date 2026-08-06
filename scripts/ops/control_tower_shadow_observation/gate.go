package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func runPreflight(ctx context.Context, cfg Config, client *http.Client) error {
	checks := []struct {
		name string
		fn   func() error
	}{
		{"gateway_health", func() error { return healthCheck(ctx, client, cfg.GatewayURL) }},
		{"read_model_health", func() error { return healthCheck(ctx, client, cfg.ReadModelURL) }},
		{"prometheus", func() error { return promAvailable(ctx, client, cfg.PrometheusURL) }},
		{"metric_shadow_comparison", func() error {
			return metricPresent(ctx, client, cfg.GatewayURL, "control_tower_read_model_shadow_comparison_total")
		}},
		{"metric_dead_letter", func() error {
			return metricPresent(ctx, client, cfg.ReadModelURL, "control_tower_shipment_dead_letter_total")
		}},
		{"metric_offset_commit_errors", func() error {
			return metricPresent(ctx, client, cfg.ReadModelURL, "control_tower_shipment_consumer_offset_commit_errors_total")
		}},
		{"cohort_manifest", func() error {
			_, err := loadCohort(cfg.CohortManifest)
			return err
		}},
	}
	for _, c := range checks {
		if err := c.fn(); err != nil {
			return fmt.Errorf("preflight %s: %w", c.name, err)
		}
		fmt.Fprintf(os.Stderr, "observation-preflight: %s OK\n", c.name)
	}
	metrics, err := collectPrometheusMetrics(ctx, client, cfg.PrometheusURL, cfg.SustainedMismatchMinutes)
	if err != nil {
		return err
	}
	if metrics.PrimaryRequestTotal > 0 {
		return fmt.Errorf("preflight: primary mode activity detected (primary request counter > 0)")
	}
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("CONTROL_TOWER_READ_MODEL_MODE")))
	if mode == "primary" {
		return fmt.Errorf("preflight: CONTROL_TOWER_READ_MODEL_MODE=primary is forbidden")
	}
	fmt.Fprintln(os.Stderr, "observation-preflight: PASS")
	return nil
}

func runSnapshot(ctx context.Context, cfg Config, client *http.Client) error {
	cohort, err := loadCohort(cfg.CohortManifest)
	if err != nil {
		return err
	}
	metrics, err := collectPrometheusMetrics(ctx, client, cfg.PrometheusURL, cfg.SustainedMismatchMinutes)
	if err != nil {
		return err
	}
	var baselines []TenantBaseline
	for _, entry := range cohort {
		jwt, err := loginForEntry(cfg, client, entry)
		if err != nil {
			return fmt.Errorf("alias %s login: %w", entry.Alias, err)
		}
		tenantID, err := entry.resolveTenantID()
		if err != nil {
			return fmt.Errorf("alias %s tenant: %w", entry.Alias, err)
		}
		bl, err := fetchTenantBaseline(ctx, client, cfg, jwt, tenantID, entry.Alias, entry.Category)
		if err != nil {
			return fmt.Errorf("alias %s baseline: %w", entry.Alias, err)
		}
		baselines = append(baselines, bl)
	}
	var kafkaParts []PartitionOffset
	if parts, err := fetchKafkaOffsets(cfg.RpkExec, cfg.KafkaGroup, cfg.KafkaTopic); err == nil {
		kafkaParts = parts
		fmt.Fprint(os.Stderr, formatPartitionOffsets("baseline", kafkaParts))
	} else {
		fmt.Fprintf(os.Stderr, "observation-snapshot: kafka offsets skipped: %v\n", err)
	}
	report := SnapshotReport{
		Timestamp:   time.Now().UTC(),
		Environment: cfg.Environment,
		Commit:      cfg.Commit,
		CohortSize:  len(cohort),
		Tenants:     baselines,
		Metrics:     metrics,
		Kafka:       partitionViews(kafkaParts),
	}
	return writeReport(cfg.OutputPath, report)
}

func runGate(ctx context.Context, cfg Config, client *http.Client) error {
	if err := validateGateConfig(cfg); err != nil {
		return err
	}
	cohort, err := loadCohort(cfg.CohortManifest)
	if err != nil {
		return err
	}
	metrics, err := collectPrometheusMetrics(ctx, client, cfg.PrometheusURL, cfg.SustainedMismatchMinutes)
	if err != nil {
		return err
	}
	if metrics.PrimaryRequestTotal > 0 {
		return fmt.Errorf("gate: primary mode activity detected")
	}

	var baselines []TenantBaseline
	for _, entry := range cohort {
		jwt, err := loginForEntry(cfg, client, entry)
		if err != nil {
			return err
		}
		tenantID, err := entry.resolveTenantID()
		if err != nil {
			return err
		}
		bl, err := fetchTenantBaseline(ctx, client, cfg, jwt, tenantID, entry.Alias, entry.Category)
		if err != nil {
			return err
		}
		if bl.PublicSource != "LEGACY" {
			return fmt.Errorf("gate: alias %s public source=%s", entry.Alias, bl.PublicSource)
		}
		baselines = append(baselines, bl)
	}

	kafkaParts, kafkaErr := fetchKafkaOffsets(cfg.RpkExec, cfg.KafkaGroup, cfg.KafkaTopic)
	if kafkaErr == nil {
		fmt.Fprint(os.Stderr, formatPartitionOffsets("gate", kafkaParts))
	}

	outcome := evaluateGate(gateInputs{
		cfg:                       cfg,
		metrics:                   metrics,
		baselines:                 baselines,
		kafkaParts:                kafkaParts,
		kafkaErr:                  kafkaErr,
		sustainedMismatchIncrease: metrics.SustainedMismatchIncrease,
	})

	report := ObservationReport{
		Timestamp:               time.Now().UTC(),
		Environment:             cfg.Environment,
		Commit:                  cfg.Commit,
		CohortSize:              len(cohort),
		MatchCount:              outcome.MatchCount,
		MismatchCount:           outcome.MismatchCount,
		IncompleteCount:         outcome.IncompleteCount,
		DeadLetterDelta:         outcome.DeadLetterDelta,
		OffsetCommitErrorsDelta: metrics.OffsetCommitErrorsTotal,
		MaxConsumerLag:          outcome.MaxConsumerLag,
		Gateway5xxDelta:         outcome.Gateway5xxDelta,
		PrimaryEnabled:          false,
		LegacyP95Seconds:        metrics.LegacyP95Seconds,
		ReadModelP95Seconds:     metrics.ReadModelP95Seconds,
		RelativeLatencyRatio:    outcome.RelativeLatencyRatio,
		Result:                  outcome.Result,
		Notes:                   outcome.Notes,
	}
	if err := writeReport(cfg.OutputPath, report); err != nil {
		return err
	}
	if outcome.Result != gateResultPass {
		return fmt.Errorf("observation gate failed")
	}
	fmt.Fprintln(os.Stderr, "observation-gate: PASS")
	return nil
}

func loginForEntry(cfg Config, client *http.Client, entry CohortEntry) (string, error) {
	tenantID, err := entry.resolveTenantID()
	if err != nil {
		return "", err
	}
	tenantCfg := cfg
	tenantCfg.TenantID = tenantID
	if email := entry.resolveEmail(); email != "" {
		tenantCfg.AdminEmail = email
	}
	if password := entry.resolvePassword(); password != "" {
		tenantCfg.AdminPassword = password
	}
	return resolveJWT(tenantCfg, client)
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
