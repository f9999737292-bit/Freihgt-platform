package outbox

import (
	"testing"
	"time"
)

func TestNextRetryAvailableAt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 5 * time.Second},
		{2, 15 * time.Second},
		{3, 60 * time.Second},
		{4, 5 * time.Minute},
		{9, 5 * time.Minute},
	}
	for _, tc := range cases {
		got := NextRetryAvailableAt(tc.attempt, now)
		if got.Sub(now) != tc.want {
			t.Fatalf("attempt %d: got %v want %v", tc.attempt, got.Sub(now), tc.want)
		}
	}
}
