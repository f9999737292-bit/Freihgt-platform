package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type ObservationReport struct {
	Timestamp                time.Time `json:"timestamp"`
	Environment              string    `json:"environment"`
	Commit                   string    `json:"commit"`
	CohortSize               int       `json:"cohortSize"`
	MatchCount               int       `json:"matchCount"`
	MismatchCount            int       `json:"mismatchCount"`
	IncompleteCount          int       `json:"incompleteCount"`
	DeadLetterDelta          float64   `json:"deadLetterDelta"`
	OffsetCommitErrorsDelta  float64   `json:"offsetCommitErrorsDelta"`
	MaxConsumerLag           int64     `json:"maxConsumerLag"`
	Gateway5xxDelta          float64   `json:"gateway5xxDelta"`
	PrimaryEnabled           bool      `json:"primaryEnabled"`
	LegacyP95Seconds         float64   `json:"legacyP95Seconds"`
	ReadModelP95Seconds      float64   `json:"readModelP95Seconds"`
	RelativeLatencyRatio     float64   `json:"relativeLatencyRatio"`
	Result                   string    `json:"result"`
	Notes                    []string  `json:"notes,omitempty"`
}

type SnapshotReport struct {
	Timestamp   time.Time        `json:"timestamp"`
	Environment string           `json:"environment"`
	Commit      string           `json:"commit"`
	CohortSize  int              `json:"cohortSize"`
	Tenants     []TenantBaseline `json:"tenants"`
	Metrics     MetricSnapshot   `json:"metrics"`
	Kafka       []PartitionView  `json:"kafkaPartitions,omitempty"`
}

type PartitionView struct {
	Partition int    `json:"partition"`
	Committed string `json:"committed"`
	LogEnd    int64  `json:"logEnd"`
	Lag       string `json:"lag"`
}

func partitionViews(parts []PartitionOffset) []PartitionView {
	out := make([]PartitionView, 0, len(parts))
	for _, p := range parts {
		lag := "UNCOMMITTED"
		if p.Lag >= 0 {
			lag = fmt.Sprintf("%d", p.Lag)
		}
		out = append(out, PartitionView{
			Partition: p.Partition,
			Committed: p.Committed.State,
			LogEnd:    p.LogEnd,
			Lag:       lag,
		})
	}
	return out
}

func writeReport(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if path == "" || path == "-" {
		fmt.Println(string(raw))
		return nil
	}
	return os.WriteFile(path, raw, 0o600)
}

func relativeLatency(legacy, readModel float64) float64 {
	if legacy <= 0 {
		return 0
	}
	return readModel / legacy
}
