package sla

import (
	"testing"
	"time"
)

func mustTime(value string) *time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	utc := parsed.UTC()
	return &utc
}

func thresholds() Thresholds {
	return Thresholds{
		AtRiskMinutes:        120,
		CriticalDelayMinutes: 240,
		StaleWarningMinutes:  120,
		StaleCriticalMinutes: 360,
	}
}

func TestCompute(t *testing.T) {
	baseNow := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	plannedPickup := mustTime("2026-07-31T14:00:00Z")
	plannedDelivery := mustTime("2026-08-01T10:00:00Z")

	tests := []struct {
		name       string
		input      Input
		want       Status
		wantReason string
	}{
		{
			name: "missing planned dates unknown",
			input: Input{
				Status:     "IN_TRANSIT",
				Now:        baseNow,
				Thresholds: thresholds(),
			},
			want:       StatusUnknown,
			wantReason: ReasonMissingPlannedDates,
		},
		{
			name: "active in plan on time",
			input: Input{
				Status:            "IN_TRANSIT",
				PlannedPickupAt:   plannedPickup,
				PlannedDeliveryAt: plannedDelivery,
				ActualPickupAt:    mustTime("2026-07-31T13:00:00Z"),
				LastUpdatedAt:     mustTime("2026-07-31T11:30:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       StatusOnTime,
			wantReason: ReasonOnSchedule,
		},
		{
			name: "pickup deadline approaching at risk",
			input: Input{
				Status:          "DRIVER_ASSIGNED",
				PlannedPickupAt: mustTime("2026-07-31T13:00:00Z"),
				LastUpdatedAt:   mustTime("2026-07-31T11:00:00Z"),
				Now:             baseNow,
				Thresholds:      thresholds(),
			},
			want:       StatusAtRisk,
			wantReason: ReasonPickupAtRisk,
		},
		{
			name: "pickup overdue delayed",
			input: Input{
				Status:          "DRIVER_ASSIGNED",
				PlannedPickupAt: mustTime("2026-07-31T10:00:00Z"),
				LastUpdatedAt:   mustTime("2026-07-31T09:00:00Z"),
				Now:             baseNow,
				Thresholds:      thresholds(),
			},
			want:       StatusDelayed,
			wantReason: ReasonPickupOverdue,
		},
		{
			name: "delivery overdue delayed",
			input: Input{
				Status:            "IN_TRANSIT",
				PlannedPickupAt:   mustTime("2026-07-30T08:00:00Z"),
				PlannedDeliveryAt: mustTime("2026-07-31T09:00:00Z"),
				ActualPickupAt:    mustTime("2026-07-30T08:30:00Z"),
				LastUpdatedAt:     mustTime("2026-07-31T08:30:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       StatusDelayed,
			wantReason: ReasonDeliveryOverdue,
		},
		{
			name: "critical delay",
			input: Input{
				Status:            "IN_TRANSIT",
				PlannedPickupAt:   mustTime("2026-07-29T08:00:00Z"),
				PlannedDeliveryAt: mustTime("2026-07-29T12:00:00Z"),
				ActualPickupAt:    mustTime("2026-07-29T08:30:00Z"),
				LastUpdatedAt:     mustTime("2026-07-29T13:00:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       StatusCritical,
			wantReason: ReasonDeliveryOverdue,
		},
		{
			name: "cancelled critical",
			input: Input{
				Status:            "CANCELLED",
				PlannedPickupAt:   plannedPickup,
				PlannedDeliveryAt: plannedDelivery,
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       StatusCritical,
			wantReason: ReasonCancelled,
		},
		{
			name: "stale updates at risk",
			input: Input{
				Status:          "IN_TRANSIT",
				PlannedPickupAt: plannedPickup,
				LastUpdatedAt:   mustTime("2026-07-31T08:00:00Z"),
				Now:             baseNow,
				Thresholds:      thresholds(),
			},
			want:       StatusAtRisk,
			wantReason: ReasonStaleUpdates,
		},
		{
			name: "stale updates critical",
			input: Input{
				Status:          "IN_TRANSIT",
				PlannedPickupAt: plannedPickup,
				LastUpdatedAt:   mustTime("2026-07-31T04:00:00Z"),
				Now:             baseNow,
				Thresholds:      thresholds(),
			},
			want:       StatusCritical,
			wantReason: ReasonStaleUpdates,
		},
		{
			name: "completed on time",
			input: Input{
				Status:            "DELIVERED",
				PlannedPickupAt:   mustTime("2026-07-30T08:00:00Z"),
				PlannedDeliveryAt: mustTime("2026-07-31T10:00:00Z"),
				ActualPickupAt:    mustTime("2026-07-30T08:00:00Z"),
				ActualDeliveryAt:  mustTime("2026-07-31T09:00:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       StatusOnTime,
			wantReason: ReasonCompletedOnTime,
		},
		{
			name: "completed late delayed",
			input: Input{
				Status:            "DELIVERED",
				PlannedPickupAt:   mustTime("2026-07-30T08:00:00Z"),
				PlannedDeliveryAt: mustTime("2026-07-31T10:00:00Z"),
				ActualPickupAt:    mustTime("2026-07-30T08:00:00Z"),
				ActualDeliveryAt:  mustTime("2026-07-31T12:00:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       StatusDelayed,
			wantReason: ReasonCompletedLate,
		},
		{
			name: "technical problem critical",
			input: Input{
				Status:           "IN_TRANSIT",
				PlannedPickupAt:  plannedPickup,
				TechnicalProblem: true,
				Now:              baseNow,
				Thresholds:       thresholds(),
			},
			want:       StatusCritical,
			wantReason: ReasonTechnicalProblem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compute(tt.input)
			if got.Status != tt.want {
				t.Fatalf("status = %s, want %s", got.Status, tt.want)
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("reason = %s, want %s", got.Reason, tt.wantReason)
			}
		})
	}
}

func TestComputePriorityCriticalOverDelayed(t *testing.T) {
	now := time.Date(2026, 7, 31, 18, 0, 0, 0, time.UTC)
	result := Compute(Input{
		Status:            "IN_TRANSIT",
		PlannedPickupAt:   mustTime("2026-07-29T08:00:00Z"),
		PlannedDeliveryAt: mustTime("2026-07-29T12:00:00Z"),
		ActualPickupAt:    mustTime("2026-07-29T08:30:00Z"),
		LastUpdatedAt:     mustTime("2026-07-29T13:00:00Z"),
		Now:               now,
		Thresholds:        thresholds(),
	})
	if result.Status != StatusCritical {
		t.Fatalf("expected CRITICAL priority, got %s", result.Status)
	}
}
