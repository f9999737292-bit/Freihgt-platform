package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type CommittedOffset struct {
	State string // UNCOMMITTED or decimal string
}

type PartitionOffset struct {
	Partition int
	Committed CommittedOffset
	LogEnd    int64
	Lag       int64 // -1 when UNCOMMITTED lag unknown
}

func normalizeCommitted(raw string) CommittedOffset {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "-" || raw == "-1" {
		return CommittedOffset{State: "UNCOMMITTED"}
	}
	if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
		return CommittedOffset{State: "UNCOMMITTED"}
	}
	return CommittedOffset{State: raw}
}

func parseRPKGroupDescribe(output, topic string) ([]PartitionOffset, error) {
	var rows []PartitionOffset
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "GROUP") || strings.HasPrefix(line, "TOPIC") || strings.HasPrefix(line, "TOTAL") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 || fields[0] != topic {
			continue
		}
		part, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		logEnd, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse logEnd partition %d: %w", part, err)
		}
		committed := normalizeCommitted(fields[2])
		var lag int64 = -1
		if committed.State != "UNCOMMITTED" {
			lagVal, err := strconv.ParseInt(fields[5], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse lag partition %d: %w", part, err)
			}
			lag = lagVal
		}
		rows = append(rows, PartitionOffset{
			Partition: part,
			Committed: committed,
			LogEnd:    logEnd,
			Lag:       lag,
		})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no partitions found for topic %s", topic)
	}
	return rows, nil
}

func fetchKafkaOffsets(rpkExec, group, topic string) ([]PartitionOffset, error) {
	cmd := exec.Command(rpkExec, "group", "describe", group)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("rpk group describe: %w (%s)", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	return parseRPKGroupDescribe(string(out), topic)
}

func computeMaxLag(partitions []PartitionOffset) int64 {
	var max int64
	for _, p := range partitions {
		if p.Lag >= 0 && p.Lag > max {
			max = p.Lag
		}
	}
	return max
}

func formatPartitionOffsets(label string, parts []PartitionOffset) string {
	var b strings.Builder
	for _, p := range parts {
		lag := "UNCOMMITTED"
		if p.Lag >= 0 {
			lag = strconv.FormatInt(p.Lag, 10)
		}
		fmt.Fprintf(&b, "%s partition %d: committed=%s, logEnd=%d, lag=%s\n",
			label, p.Partition, p.Committed.State, p.LogEnd, lag)
	}
	return b.String()
}

func assertCommittedUnchanged(before, after []PartitionOffset) error {
	afterMap := map[int]CommittedOffset{}
	for _, p := range after {
		afterMap[p.Partition] = p.Committed
	}
	for _, p := range before {
		got, ok := afterMap[p.Partition]
		if !ok {
			return fmt.Errorf("partition %d missing in after snapshot", p.Partition)
		}
		if p.Committed.State != got.State {
			return fmt.Errorf("partition %d committed changed (%s -> %s)", p.Partition, p.Committed.State, got.State)
		}
	}
	return nil
}

func assertCatchUp(before, during, after []PartitionOffset) error {
	duringMap := map[int]PartitionOffset{}
	afterMap := map[int]PartitionOffset{}
	for _, p := range during {
		duringMap[p.Partition] = p
	}
	for _, p := range after {
		afterMap[p.Partition] = p
	}
	for _, b := range before {
		d, ok := duringMap[b.Partition]
		if !ok {
			return fmt.Errorf("partition %d missing in during snapshot", b.Partition)
		}
		a, ok := afterMap[b.Partition]
		if !ok {
			return fmt.Errorf("partition %d missing in after snapshot", b.Partition)
		}
		if a.Lag != 0 {
			return fmt.Errorf("partition %d lag=%d after catch-up (expected 0)", a.Partition, a.Lag)
		}
		if b.Committed.State != "UNCOMMITTED" && d.LogEnd > parseCommittedInt(b.Committed) {
			if a.Committed.State == "UNCOMMITTED" || parseCommittedInt(a.Committed) <= parseCommittedInt(b.Committed) {
				return fmt.Errorf("partition %d committed did not advance (%s -> %s) while logEnd grew",
					b.Partition, b.Committed.State, a.Committed.State)
			}
		}
		if b.Committed.State != "UNCOMMITTED" && a.Committed.State != "UNCOMMITTED" {
			if parseCommittedInt(a.Committed) < parseCommittedInt(b.Committed) {
				return fmt.Errorf("partition %d committed regressed", b.Partition)
			}
		}
	}
	return nil
}

func parseCommittedInt(c CommittedOffset) int64 {
	if c.State == "UNCOMMITTED" {
		return -1
	}
	v, _ := strconv.ParseInt(c.State, 10, 64)
	return v
}
