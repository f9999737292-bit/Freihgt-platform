package main

import (
	"fmt"
	"strings"
)

const (
	gateResultPass = "PASS"
	gateResultWarn = "WARN"
	gateResultFail = "FAIL"

	reasonKafkaLagUnavailable       = "KAFKA_LAG_UNAVAILABLE"
	reasonShadowMatchEvidenceMissing = "SHADOW_MATCH_EVIDENCE_MISSING"
	reasonSLODeadLetterExceeded     = "SLO_DEAD_LETTER_EXCEEDED"
	reasonSLOGateway5xxExceeded     = "SLO_GATEWAY_5XX_EXCEEDED"
	reasonSLOLatencyExceeded        = "SLO_LATENCY_EXCEEDED"
	reasonSLOConfigurationMissing   = "SLO_CONFIGURATION_MISSING"
	reasonSustainedMismatch         = "SUSTAINED_MISMATCH"
)

type gateInputs struct {
	cfg                       Config
	metrics                   MetricSnapshot
	baselines                 []TenantBaseline
	kafkaParts                []PartitionOffset
	kafkaErr                  error
	sustainedMismatchIncrease float64
}

type gateOutcome struct {
	Result              string
	Notes               []string
	MatchCount          int
	MismatchCount       int
	IncompleteCount     int
	MaxConsumerLag      int64
	DeadLetterDelta     float64
	Gateway5xxDelta     float64
	RelativeLatencyRatio float64
}

func validateGateConfig(cfg Config) error {
	if cfg.SustainedMismatchMinutes <= 0 {
		return fmt.Errorf("%s: OBSERVATION_SUSTAINED_MISMATCH_MIN must be > 0", reasonSLOConfigurationMissing)
	}
	if cfg.MaxRelativeLatencyRatio <= 0 {
		return fmt.Errorf("%s: OBSERVATION_MAX_RELATIVE_LATENCY_RATIO must be > 0", reasonSLOConfigurationMissing)
	}
	return nil
}

func evaluateGate(in gateInputs) gateOutcome {
	out := gateOutcome{
		Result:           gateResultPass,
		DeadLetterDelta:  in.metrics.DeadLetterTotal,
		Gateway5xxDelta:  in.metrics.Gateway5xxTotal,
		RelativeLatencyRatio: relativeLatency(in.metrics.LegacyP95Seconds, in.metrics.ReadModelP95Seconds),
	}

	if in.kafkaErr != nil {
		out.Result = gateResultFail
		out.Notes = append(out.Notes, reasonKafkaLagUnavailable+": "+in.kafkaErr.Error())
		return out
	}
	if len(in.kafkaParts) == 0 {
		out.Result = gateResultFail
		out.Notes = append(out.Notes, reasonKafkaLagUnavailable+": no partition offsets returned")
		return out
	}

	maxLag, lagReason := evaluateConsumerLag(in.kafkaParts)
	out.MaxConsumerLag = maxLag
	if lagReason != "" {
		out.Result = gateResultFail
		out.Notes = append(out.Notes, lagReason)
	}
	if maxLag > in.cfg.MaxConsumerLag {
		out.Result = gateResultFail
		out.Notes = append(out.Notes, fmt.Sprintf("max consumer lag %d exceeds %d", maxLag, in.cfg.MaxConsumerLag))
	}

	if in.metrics.OffsetCommitErrorsTotal > 0 {
		out.Result = gateResultFail
		out.Notes = append(out.Notes, "offset commit errors > 0")
	}

	if in.metrics.DeadLetterTotal > in.cfg.MaxDeadLetterTotal {
		out.Result = gateResultFail
		out.Notes = append(out.Notes, fmt.Sprintf("%s: dead-letter total %.0f exceeds allowed %.0f",
			reasonSLODeadLetterExceeded, in.metrics.DeadLetterTotal, in.cfg.MaxDeadLetterTotal))
	}
	if in.metrics.Gateway5xxTotal > in.cfg.MaxGateway5xxTotal {
		out.Result = gateResultFail
		out.Notes = append(out.Notes, fmt.Sprintf("%s: gateway 5xx total %.0f exceeds allowed %.0f",
			reasonSLOGateway5xxExceeded, in.metrics.Gateway5xxTotal, in.cfg.MaxGateway5xxTotal))
	}

	if in.metrics.LegacyP95Seconds <= 0 || in.metrics.ReadModelP95Seconds <= 0 {
		out.Result = gateResultFail
		out.Notes = append(out.Notes, reasonSLOConfigurationMissing+": latency p95 metrics unavailable")
	} else if out.RelativeLatencyRatio > in.cfg.MaxRelativeLatencyRatio {
		out.Result = gateResultFail
		out.Notes = append(out.Notes, fmt.Sprintf("%s: relative latency ratio %.2f exceeds allowed %.2f",
			reasonSLOLatencyExceeded, out.RelativeLatencyRatio, in.cfg.MaxRelativeLatencyRatio))
	}

	matchCount, mismatchCount, incompleteCount, matchReasons := evaluateMatchEvidence(in.cfg, in.metrics, in.baselines)
	out.MatchCount = matchCount
	out.MismatchCount = mismatchCount
	out.IncompleteCount = incompleteCount
	if len(matchReasons) > 0 {
		out.Result = gateResultFail
		out.Notes = append(out.Notes, matchReasons...)
	}

	if in.sustainedMismatchIncrease > 0 {
		out.Result = gateResultFail
		out.Notes = append(out.Notes, fmt.Sprintf("%s: mismatch persisted for >= %d minutes",
			reasonSustainedMismatch, in.cfg.SustainedMismatchMinutes))
	} else if in.metrics.MismatchTotal > 0 {
		out.Result = gateResultWarn
		out.Notes = append(out.Notes, "transient mismatch below sustained threshold")
	}

	if out.Result == gateResultPass || out.Result == gateResultWarn {
		ratio := float64(matchCount) / float64(max(1, len(in.baselines)))
		if ratio < in.cfg.RequireMatchRatio && out.Result != gateResultWarn {
			out.Result = gateResultFail
			out.Notes = append(out.Notes, fmt.Sprintf("match ratio %.2f below required %.2f", ratio, in.cfg.RequireMatchRatio))
		}
	}

	if out.Result == gateResultWarn {
		out.Notes = append(out.Notes, "observation gate WARN blocks staging PASS")
	}

	return out
}

func evaluateConsumerLag(parts []PartitionOffset) (int64, string) {
	if len(parts) == 0 {
		return 0, reasonKafkaLagUnavailable + ": partition offsets missing"
	}
	var maxLag int64
	for _, part := range parts {
		if part.Committed.State == "UNCOMMITTED" || part.Lag < 0 {
			return 0, fmt.Sprintf("%s: partition %d lag unavailable", reasonKafkaLagUnavailable, part.Partition)
		}
		if part.Lag > maxLag {
			maxLag = part.Lag
		}
	}
	return maxLag, ""
}

func evaluateMatchEvidence(cfg Config, metrics MetricSnapshot, baselines []TenantBaseline) (matchCount, mismatchCount, incompleteCount int, failReasons []string) {
	blockedCategories := map[string]struct{}{
		"PENDING_PROMETHEUS_CLASSIFICATION": {},
		"UNKNOWN":                           {},
		"MISSING":                           {},
		"NO_DATA":                           {},
	}
	if metrics.MatchTotal > 0 {
		allClassified := true
		for _, baseline := range baselines {
			category := strings.ToUpper(strings.TrimSpace(baseline.ComparisonCategory))
			if _, blocked := blockedCategories[category]; blocked {
				failReasons = append(failReasons, fmt.Sprintf("%s: alias %s category=%s",
					reasonShadowMatchEvidenceMissing, baseline.Alias, baseline.ComparisonCategory))
				allClassified = false
				continue
			}
			if category == "FALLBACK_USED" || category == "PUBLIC_SOURCE_NOT_LEGACY" {
				mismatchCount++
				allClassified = false
			}
		}
		if allClassified {
			matchCount = len(baselines)
		}
	} else {
		failReasons = append(failReasons, reasonShadowMatchEvidenceMissing+": MATCH counter is zero")
	}

	if metrics.MismatchTotal > 0 {
		mismatchCount = int(metrics.MismatchTotal)
	}
	if metrics.IncompletePartialTotal > 0 {
		incompleteCount = int(metrics.IncompletePartialTotal)
	}

	return matchCount, mismatchCount, incompleteCount, failReasons
}
