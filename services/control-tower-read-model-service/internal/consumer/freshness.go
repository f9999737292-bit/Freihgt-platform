package consumer

import (
	"sync/atomic"
	"time"
)

type Freshness struct {
	consumerRunning         atomic.Bool
	lastRecordReceivedAt    atomic.Value // time.Time
	lastProjectionAppliedAt atomic.Value // time.Time
}

func NewFreshness() *Freshness {
	f := &Freshness{}
	f.lastRecordReceivedAt.Store(time.Time{})
	f.lastProjectionAppliedAt.Store(time.Time{})
	return f
}

func (f *Freshness) SetRunning(running bool) {
	f.consumerRunning.Store(running)
}

func (f *Freshness) ConsumerRunning() bool {
	return f.consumerRunning.Load()
}

func (f *Freshness) MarkRecordReceived(at time.Time) {
	f.lastRecordReceivedAt.Store(at.UTC())
}

func (f *Freshness) MarkProjectionApplied(at time.Time) {
	f.lastProjectionAppliedAt.Store(at.UTC())
}

func (f *Freshness) LastRecordReceivedAt() time.Time {
	if v := f.lastRecordReceivedAt.Load(); v != nil {
		if t, ok := v.(time.Time); ok {
			return t
		}
	}
	return time.Time{}
}

func (f *Freshness) LastProjectionAppliedAt() time.Time {
	if v := f.lastProjectionAppliedAt.Load(); v != nil {
		if t, ok := v.(time.Time); ok {
			return t
		}
	}
	return time.Time{}
}

type FreshnessSnapshot struct {
	ConsumerRunning         bool       `json:"consumerRunning"`
	LastRecordReceivedAt    *time.Time `json:"lastRecordReceivedAt,omitempty"`
	LastProjectionAppliedAt *time.Time `json:"lastProjectionAppliedAt,omitempty"`
}

func (f *Freshness) Snapshot() FreshnessSnapshot {
	snap := FreshnessSnapshot{ConsumerRunning: f.ConsumerRunning()}
	if t := f.LastRecordReceivedAt(); !t.IsZero() {
		tt := t
		snap.LastRecordReceivedAt = &tt
	}
	if t := f.LastProjectionAppliedAt(); !t.IsZero() {
		tt := t
		snap.LastProjectionAppliedAt = &tt
	}
	return snap
}
