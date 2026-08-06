package main

import (
	"errors"
	"testing"
)

func TestGateFailsWhenRPKCommandFails(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{MaxConsumerLag: 0, RequireMatchRatio: 1, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5},
		kafkaErr: errors.New("rpk not found"),
	})
	if out.Result != gateResultFail {
		t.Fatalf("result=%s want FAIL", out.Result)
	}
	if !containsNote(out.Notes, reasonKafkaLagUnavailable) {
		t.Fatalf("notes=%v", out.Notes)
	}
	if out.MaxConsumerLag != 0 {
		t.Fatalf("maxLag=%d want 0 on measurement failure", out.MaxConsumerLag)
	}
}

func TestGateFailsWhenRPKReturnsInvalidJSON(t *testing.T) {
	_, err := parseRPKGroupDescribe("not-json-output", "shipment.status.v1")
	if err == nil {
		t.Fatal("expected parse error")
	}
	out := evaluateGate(gateInputs{
		cfg: Config{MaxConsumerLag: 0, RequireMatchRatio: 1, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5},
		kafkaErr: err,
	})
	if out.Result != gateResultFail {
		t.Fatalf("result=%s want FAIL", out.Result)
	}
}

func TestGateFailsWhenPartitionOffsetsMissing(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{MaxConsumerLag: 0, RequireMatchRatio: 1, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5},
		kafkaParts: nil,
	})
	if out.Result != gateResultFail {
		t.Fatalf("result=%s want FAIL", out.Result)
	}
}

func TestGateFailsWhenLagCannotBeCalculated(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{MaxConsumerLag: 0, RequireMatchRatio: 1, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5},
		kafkaParts: []PartitionOffset{{
			Partition: 0,
			Committed: CommittedOffset{State: "UNCOMMITTED"},
			LogEnd:    10,
			Lag:       -1,
		}},
	})
	if out.Result != gateResultFail {
		t.Fatalf("result=%s want FAIL", out.Result)
	}
	if !containsNote(out.Notes, reasonKafkaLagUnavailable) {
		t.Fatalf("notes=%v", out.Notes)
	}
}

func TestGatePassesWhenAllPartitionsHaveValidLag(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{
			MaxConsumerLag: 10, RequireMatchRatio: 1, MaxRelativeLatencyRatio: 1.2,
			SustainedMismatchMinutes: 5, MaxDeadLetterTotal: 0, MaxGateway5xxTotal: 0,
		},
		metrics: MetricSnapshot{
			MatchTotal: 1, LegacyP95Seconds: 0.2, ReadModelP95Seconds: 0.21,
		},
		baselines: []TenantBaseline{{Alias: "A", ComparisonCategory: "MATCH"}},
		kafkaParts: []PartitionOffset{{
			Partition: 0, Committed: CommittedOffset{State: "10"}, LogEnd: 10, Lag: 0,
		}},
	})
	if out.Result != gateResultPass {
		t.Fatalf("result=%s notes=%v", out.Result, out.Notes)
	}
}

func TestGateFailsWhenDeadLetterThresholdExceeded(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{MaxDeadLetterTotal: 0, MaxGateway5xxTotal: 0, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5, RequireMatchRatio: 1},
		metrics: MetricSnapshot{DeadLetterTotal: 1, MatchTotal: 1, LegacyP95Seconds: 0.2, ReadModelP95Seconds: 0.21},
		baselines: []TenantBaseline{{Alias: "A", ComparisonCategory: "MATCH"}},
		kafkaParts: []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "1"}, LogEnd: 1, Lag: 0}},
	})
	if out.Result != gateResultFail || !containsNote(out.Notes, reasonSLODeadLetterExceeded) {
		t.Fatalf("result=%s notes=%v", out.Result, out.Notes)
	}
}

func TestGateFailsWhenGateway5xxThresholdExceeded(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{MaxDeadLetterTotal: 0, MaxGateway5xxTotal: 0, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5, RequireMatchRatio: 1},
		metrics: MetricSnapshot{Gateway5xxTotal: 2, MatchTotal: 1, LegacyP95Seconds: 0.2, ReadModelP95Seconds: 0.21},
		baselines: []TenantBaseline{{Alias: "A", ComparisonCategory: "MATCH"}},
		kafkaParts: []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "1"}, LogEnd: 1, Lag: 0}},
	})
	if out.Result != gateResultFail || !containsNote(out.Notes, reasonSLOGateway5xxExceeded) {
		t.Fatalf("result=%s notes=%v", out.Result, out.Notes)
	}
}

func TestGateFailsWhenLatencyThresholdExceeded(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{MaxDeadLetterTotal: 0, MaxGateway5xxTotal: 0, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5, RequireMatchRatio: 1},
		metrics: MetricSnapshot{MatchTotal: 1, LegacyP95Seconds: 0.2, ReadModelP95Seconds: 0.5},
		baselines: []TenantBaseline{{Alias: "A", ComparisonCategory: "MATCH"}},
		kafkaParts: []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "1"}, LogEnd: 1, Lag: 0}},
	})
	if out.Result != gateResultFail || !containsNote(out.Notes, reasonSLOLatencyExceeded) {
		t.Fatalf("result=%s notes=%v", out.Result, out.Notes)
	}
}

func TestGateFailsWhenMatchEvidenceMissing(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{MaxDeadLetterTotal: 0, MaxGateway5xxTotal: 0, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5, RequireMatchRatio: 1},
		metrics: MetricSnapshot{MatchTotal: 0, LegacyP95Seconds: 0.2, ReadModelP95Seconds: 0.21},
		baselines: []TenantBaseline{{Alias: "A", ComparisonCategory: "PENDING_PROMETHEUS_CLASSIFICATION"}},
		kafkaParts: []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "1"}, LogEnd: 1, Lag: 0}},
	})
	if out.Result != gateResultFail || !containsNote(out.Notes, reasonShadowMatchEvidenceMissing) {
		t.Fatalf("result=%s notes=%v", out.Result, out.Notes)
	}
}

func TestGateFailsForPendingClassification(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{MaxDeadLetterTotal: 0, MaxGateway5xxTotal: 0, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5, RequireMatchRatio: 1},
		metrics: MetricSnapshot{MatchTotal: 0, MismatchTotal: 0, LegacyP95Seconds: 0.2, ReadModelP95Seconds: 0.21},
		baselines: []TenantBaseline{{Alias: "A", ComparisonCategory: "PENDING_PROMETHEUS_CLASSIFICATION"}},
		kafkaParts: []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "1"}, LogEnd: 1, Lag: 0}},
	})
	if out.Result != gateResultFail {
		t.Fatalf("result=%s notes=%v", out.Result, out.Notes)
	}
}

func TestGateWarnsForTransientMismatch(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{MaxDeadLetterTotal: 0, MaxGateway5xxTotal: 0, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5, RequireMatchRatio: 1},
		metrics: MetricSnapshot{MatchTotal: 1, MismatchTotal: 1, LegacyP95Seconds: 0.2, ReadModelP95Seconds: 0.21},
		baselines: []TenantBaseline{{Alias: "A", ComparisonCategory: "MATCH"}},
		kafkaParts: []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "1"}, LogEnd: 1, Lag: 0}},
		sustainedMismatchIncrease: 0,
	})
	if out.Result != gateResultWarn {
		t.Fatalf("result=%s notes=%v", out.Result, out.Notes)
	}
}

func TestGateFailsForSustainedMismatch(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{MaxDeadLetterTotal: 0, MaxGateway5xxTotal: 0, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5, RequireMatchRatio: 1},
		metrics: MetricSnapshot{MatchTotal: 1, MismatchTotal: 2, LegacyP95Seconds: 0.2, ReadModelP95Seconds: 0.21},
		baselines: []TenantBaseline{{Alias: "A", ComparisonCategory: "MATCH"}},
		kafkaParts: []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "1"}, LogEnd: 1, Lag: 0}},
		sustainedMismatchIncrease: 2,
	})
	if out.Result != gateResultFail || !containsNote(out.Notes, reasonSustainedMismatch) {
		t.Fatalf("result=%s notes=%v", out.Result, out.Notes)
	}
}

func TestGateRejectsInvalidSustainedMismatchDuration(t *testing.T) {
	err := validateGateConfig(Config{SustainedMismatchMinutes: 0, MaxRelativeLatencyRatio: 1.2})
	if err == nil {
		t.Fatal("expected config validation error")
	}
}

func TestGateDoesNotTreatMissingTimestampAsMatch(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{MaxDeadLetterTotal: 0, MaxGateway5xxTotal: 0, MaxRelativeLatencyRatio: 1.2, SustainedMismatchMinutes: 5, RequireMatchRatio: 1},
		metrics: MetricSnapshot{MatchTotal: 0, LegacyP95Seconds: 0.2, ReadModelP95Seconds: 0.21},
		baselines: []TenantBaseline{{Alias: "A", ComparisonCategory: "UNKNOWN"}},
		kafkaParts: []PartitionOffset{{Partition: 0, Committed: CommittedOffset{State: "1"}, LogEnd: 1, Lag: 0}},
	})
	if out.MatchCount > 0 || out.Result == gateResultPass {
		t.Fatalf("result=%s matchCount=%d notes=%v", out.Result, out.MatchCount, out.Notes)
	}
}

func containsNote(notes []string, prefix string) bool {
	for _, note := range notes {
		if len(note) >= len(prefix) && note[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func TestValidateGateConfigRequiresLatencyRatio(t *testing.T) {
	err := validateGateConfig(Config{SustainedMismatchMinutes: 5, MaxRelativeLatencyRatio: 0})
	if err == nil || !containsNote([]string{err.Error()}, reasonSLOConfigurationMissing) {
		t.Fatalf("err=%v", err)
	}
}

func TestPositiveMatchPass(t *testing.T) {
	out := evaluateGate(gateInputs{
		cfg: Config{
			MaxConsumerLag: 5, RequireMatchRatio: 1, MaxRelativeLatencyRatio: 1.2,
			SustainedMismatchMinutes: 5, MaxDeadLetterTotal: 0, MaxGateway5xxTotal: 0,
		},
		metrics: MetricSnapshot{
			MatchTotal: 3, LegacyP95Seconds: 0.25, ReadModelP95Seconds: 0.28,
		},
		baselines: []TenantBaseline{
			{Alias: "A", ComparisonCategory: "MATCH"},
			{Alias: "B", ComparisonCategory: "MATCH"},
		},
		kafkaParts: []PartitionOffset{
			{Partition: 0, Committed: CommittedOffset{State: "10"}, LogEnd: 10, Lag: 0},
			{Partition: 1, Committed: CommittedOffset{State: "20"}, LogEnd: 20, Lag: 0},
		},
	})
	if out.Result != gateResultPass {
		t.Fatalf("result=%s notes=%v", out.Result, out.Notes)
	}
}
