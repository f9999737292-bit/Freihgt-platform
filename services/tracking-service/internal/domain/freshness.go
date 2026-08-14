package domain

import "time"

// Centralized tracking freshness policy defaults (v0.7.0).
// Tenant/provider-specific overrides may be added in future releases.
const (
	FreshThresholdMinutes = 10
	StaleThresholdMinutes = 30
)

type FreshnessPolicy struct {
	FreshThreshold time.Duration
	StaleThreshold time.Duration
}

func DefaultFreshnessPolicy() FreshnessPolicy {
	return FreshnessPolicy{
		FreshThreshold: time.Duration(FreshThresholdMinutes) * time.Minute,
		StaleThreshold: time.Duration(StaleThresholdMinutes) * time.Minute,
	}
}

func EvaluateFreshness(lastRecordedAt *time.Time, now time.Time, policy FreshnessPolicy) (status string, ageSeconds int64) {
	if lastRecordedAt == nil {
		return FreshnessUnknown, 0
	}
	age := now.Sub(lastRecordedAt.UTC())
	if age < 0 {
		age = 0
	}
	ageSeconds = int64(age.Seconds())
	switch {
	case age <= policy.FreshThreshold:
		return FreshnessFresh, ageSeconds
	case age <= policy.StaleThreshold:
		return FreshnessStale, ageSeconds
	default:
		return FreshnessLost, ageSeconds
	}
}

func DeriveTrackingStatus(hasActiveBinding bool, lastRecordedAt *time.Time, bindingEnded bool, now time.Time, policy FreshnessPolicy) string {
	if bindingEnded {
		return TrackingStatusEnded
	}
	if !hasActiveBinding {
		return TrackingStatusNotConfigured
	}
	if lastRecordedAt == nil {
		return TrackingStatusAwaitingData
	}
	freshness, _ := EvaluateFreshness(lastRecordedAt, now, policy)
	switch freshness {
	case FreshnessFresh:
		return TrackingStatusActive
	case FreshnessStale:
		return TrackingStatusStale
	case FreshnessLost:
		return TrackingStatusLost
	default:
		return TrackingStatusAwaitingData
	}
}
