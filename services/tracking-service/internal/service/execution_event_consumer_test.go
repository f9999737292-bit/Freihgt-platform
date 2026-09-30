package service

import (
	"context"
	"errors"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestFailedExecutionRecordIsNotCommittedPastLaterSuccess(t *testing.T) {
	failed := executionRecord(0, 10, "shipment.route_stop.current")
	later := executionRecord(0, 11, "shipment.route_stop.arrived")
	var commits []int64
	attempts := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := consumeBatch(ctx, []*kgo.Record{later, failed}, func(_ context.Context, raw []byte) error {
		if string(raw) == string(failed.Value) {
			attempts++
			if len(commits) != 0 {
				t.Fatalf("committed before the failed record succeeded: %v", commits)
			}
			return errors.New("transient")
		}
		t.Fatal("later record was applied while offset 10 was unresolved")
		return nil
	}, func(_ context.Context, record *kgo.Record) error {
		commits = append(commits, record.Offset)
		return nil
	}, func(context.Context) error {
		cancel()
		return context.Canceled
	})
	if err == nil {
		t.Fatal("expected retry to stop when the context is canceled")
	}
	if attempts != 1 {
		t.Fatalf("attempts %d", attempts)
	}
	if len(commits) != 0 {
		t.Fatalf("committed offsets %v", commits)
	}
}

func TestRetryThenCommitInOffsetOrderOnce(t *testing.T) {
	first := executionRecord(0, 10, "shipment.route_stop.current")
	second := executionRecord(0, 11, "shipment.route_stop.completed")
	attempts := map[int64]int{}
	var commits []int64
	err := consumeBatch(context.Background(), []*kgo.Record{second, first}, func(_ context.Context, raw []byte) error {
		offset := int64(10)
		if string(raw) == string(second.Value) {
			offset = 11
		}
		attempts[offset]++
		if offset == 10 && attempts[10] == 1 {
			return errors.New("transient")
		}
		if offset == 11 && attempts[10] < 2 {
			t.Fatal("offset 11 applied before offset 10 succeeded")
		}
		return nil
	}, func(_ context.Context, record *kgo.Record) error {
		commits = append(commits, record.Offset)
		return nil
	}, func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if attempts[10] != 2 || attempts[11] != 1 {
		t.Fatalf("attempts %+v", attempts)
	}
	if len(commits) != 2 || commits[0] != 10 || commits[1] != 11 {
		t.Fatalf("commits %v", commits)
	}
}

func TestDuplicateExecutionEventRemainsApplicable(t *testing.T) {
	payload := executionRecord(0, 4, "shipment.route_stop.current").Value
	again := &kgo.Record{Topic: "shipment.status.v1", Partition: 0, Offset: 5, Value: append([]byte(nil), payload...)}
	seen := map[string]int{}
	var commits []int64
	err := consumeBatch(context.Background(), []*kgo.Record{executionRecord(0, 4, "shipment.route_stop.current"), again}, func(_ context.Context, raw []byte) error {
		seen[string(raw)]++
		return nil
	}, func(_ context.Context, record *kgo.Record) error {
		commits = append(commits, record.Offset)
		return nil
	}, func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if seen[string(payload)] != 2 {
		t.Fatalf("duplicate apply count %d", seen[string(payload)])
	}
	if len(commits) != 2 || commits[0] != 4 || commits[1] != 5 {
		t.Fatalf("commits %v", commits)
	}
}

func executionRecord(partition int32, offset int64, eventType string) *kgo.Record {
	return &kgo.Record{
		Topic:     "shipment.status.v1",
		Partition: partition,
		Offset:    offset,
		Value:     []byte(`{"event_type":"` + eventType + `","event_id":"evt-` + eventType + `"}`),
	}
}
