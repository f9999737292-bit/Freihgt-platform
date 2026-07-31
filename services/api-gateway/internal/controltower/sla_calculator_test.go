package controltower

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

func thresholds() SLAThresholds {
	return SLAThresholds{
		AtRiskMinutes:        120,
		CriticalDelayMinutes: 240,
		StaleWarningMinutes:  120,
		StaleCriticalMinutes: 360,
	}
}

func TestComputeSLA(t *testing.T) {
	baseNow := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	plannedPickup := mustTime("2026-07-31T14:00:00Z")
	plannedDelivery := mustTime("2026-08-01T10:00:00Z")

	tests := []struct {
		name       string
		input      SLAInput
		want       SLAStatus
		wantReason string
	}{
		{
			name: "missing planned dates unknown",
			input: SLAInput{
				Status:     "IN_TRANSIT",
				Now:        baseNow,
				Thresholds: thresholds(),
			},
			want:       SLAStatusUnknown,
			wantReason: SLAReasonMissingPlannedDates,
		},
		{
			name: "active in plan on time",
			input: SLAInput{
				Status:            "IN_TRANSIT",
				PlannedPickupAt:   plannedPickup,
				PlannedDeliveryAt: plannedDelivery,
				ActualPickupAt:    mustTime("2026-07-31T13:00:00Z"),
				LastUpdatedAt:     mustTime("2026-07-31T11:30:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       SLAStatusOnTime,
			wantReason: SLAReasonOnSchedule,
		},
		{
			name: "pickup deadline approaching at risk",
			input: SLAInput{
				Status:          "DRIVER_ASSIGNED",
				PlannedPickupAt: mustTime("2026-07-31T13:00:00Z"),
				LastUpdatedAt:   mustTime("2026-07-31T11:00:00Z"),
				Now:             baseNow,
				Thresholds:      thresholds(),
			},
			want:       SLAStatusAtRisk,
			wantReason: SLAReasonPickupAtRisk,
		},
		{
			name: "pickup overdue delayed",
			input: SLAInput{
				Status:          "DRIVER_ASSIGNED",
				PlannedPickupAt: mustTime("2026-07-31T10:00:00Z"),
				LastUpdatedAt:   mustTime("2026-07-31T09:00:00Z"),
				Now:             baseNow,
				Thresholds:      thresholds(),
			},
			want:       SLAStatusDelayed,
			wantReason: SLAReasonPickupOverdue,
		},
		{
			name: "delivery overdue delayed",
			input: SLAInput{
				Status:            "IN_TRANSIT",
				PlannedPickupAt:   mustTime("2026-07-30T08:00:00Z"),
				PlannedDeliveryAt: mustTime("2026-07-31T09:00:00Z"),
				ActualPickupAt:    mustTime("2026-07-30T08:30:00Z"),
				LastUpdatedAt:     mustTime("2026-07-31T08:30:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       SLAStatusDelayed,
			wantReason: SLAReasonDeliveryOverdue,
		},
		{
			name: "critical delay",
			input: SLAInput{
				Status:            "IN_TRANSIT",
				PlannedPickupAt:   mustTime("2026-07-30T08:00:00Z"),
				PlannedDeliveryAt: mustTime("2026-07-30T12:00:00Z"),
				ActualPickupAt:    mustTime("2026-07-30T08:30:00Z"),
				LastUpdatedAt:     mustTime("2026-07-30T13:00:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       SLAStatusCritical,
			wantReason: SLAReasonDeliveryOverdue,
		},
		{
			name: "cancelled critical",
			input: SLAInput{
				Status:            "CANCELLED",
				PlannedPickupAt:   plannedPickup,
				PlannedDeliveryAt: plannedDelivery,
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       SLAStatusCritical,
			wantReason: SLAReasonCancelled,
		},
		{
			name: "stale warning at risk",
			input: SLAInput{
				Status:            "IN_TRANSIT",
				PlannedPickupAt:   mustTime("2026-07-30T08:00:00Z"),
				PlannedDeliveryAt: plannedDelivery,
				ActualPickupAt:    mustTime("2026-07-30T08:30:00Z"),
				LastUpdatedAt:     mustTime("2026-07-31T08:00:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       SLAStatusAtRisk,
			wantReason: SLAReasonStaleUpdates,
		},
		{
			name: "stale critical",
			input: SLAInput{
				Status:            "IN_TRANSIT",
				PlannedPickupAt:   mustTime("2026-07-29T08:00:00Z"),
				PlannedDeliveryAt: plannedDelivery,
				ActualPickupAt:    mustTime("2026-07-29T08:30:00Z"),
				LastUpdatedAt:     mustTime("2026-07-31T03:00:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       SLAStatusCritical,
			wantReason: SLAReasonStaleUpdates,
		},
		{
			name: "completed on time",
			input: SLAInput{
				Status:            "DELIVERED",
				PlannedPickupAt:   mustTime("2026-07-30T08:00:00Z"),
				PlannedDeliveryAt: mustTime("2026-07-31T10:00:00Z"),
				ActualPickupAt:    mustTime("2026-07-30T08:30:00Z"),
				ActualDeliveryAt:  mustTime("2026-07-31T09:00:00Z"),
				LastUpdatedAt:     mustTime("2026-07-31T09:00:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       SLAStatusOnTime,
			wantReason: SLAReasonCompletedOnTime,
		},
		{
			name: "completed late delayed",
			input: SLAInput{
				Status:            "DELIVERED",
				PlannedPickupAt:   mustTime("2026-07-30T08:00:00Z"),
				PlannedDeliveryAt: mustTime("2026-07-31T10:00:00Z"),
				ActualPickupAt:    mustTime("2026-07-30T08:30:00Z"),
				ActualDeliveryAt:  mustTime("2026-07-31T11:30:00Z"),
				LastUpdatedAt:     mustTime("2026-07-31T11:30:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
			},
			want:       SLAStatusDelayed,
			wantReason: SLAReasonCompletedLate,
		},
		{
			name: "critical priority over delayed",
			input: SLAInput{
				Status:            "IN_TRANSIT",
				PlannedPickupAt:   mustTime("2026-07-29T08:00:00Z"),
				PlannedDeliveryAt: mustTime("2026-07-29T12:00:00Z"),
				ActualPickupAt:    mustTime("2026-07-29T08:30:00Z"),
				LastUpdatedAt:     mustTime("2026-07-29T13:00:00Z"),
				Now:               baseNow,
				Thresholds:        thresholds(),
				TechnicalProblem:  true,
			},
			want:       SLAStatusCritical,
			wantReason: SLAReasonTechnicalProblem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeSLA(tt.input)
			if got.Status != tt.want {
				t.Fatalf("status = %s, want %s", got.Status, tt.want)
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("reason = %s, want %s", got.Reason, tt.wantReason)
			}
		})
	}
}

func TestComputeSLAPriorityCriticalOverDelayed(t *testing.T) {
	now := time.Date(2026, 7, 31, 18, 0, 0, 0, time.UTC)
	result := ComputeSLA(SLAInput{
		Status:            "IN_TRANSIT",
		PlannedPickupAt:   mustTime("2026-07-29T08:00:00Z"),
		PlannedDeliveryAt: mustTime("2026-07-29T12:00:00Z"),
		ActualPickupAt:    mustTime("2026-07-29T08:30:00Z"),
		LastUpdatedAt:     mustTime("2026-07-29T13:00:00Z"),
		Now:               now,
		Thresholds:        thresholds(),
	})
	if result.Status != SLAStatusCritical {
		t.Fatalf("expected CRITICAL priority, got %s", result.Status)
	}
}
