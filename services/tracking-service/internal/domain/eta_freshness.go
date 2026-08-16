package domain

import "time"

const (
	ETAFreshThresholdMinutes   = 15
	ETAStaleThresholdMinutes   = 60
)

type ETAFreshnessPolicy struct {
	FreshThreshold   time.Duration
	StaleThreshold   time.Duration
}

func DefaultETAFreshnessPolicy() ETAFreshnessPolicy {
	return ETAFreshnessPolicy{
		FreshThreshold: time.Duration(ETAFreshThresholdMinutes) * time.Minute,
		StaleThreshold: time.Duration(ETAStaleThresholdMinutes) * time.Minute,
	}
}

func EvaluateETAFreshness(sourceObservedAt *time.Time, now time.Time, policy ETAFreshnessPolicy) (status string, ageSeconds int64) {
	if sourceObservedAt == nil {
		return ETAFreshnessUnknown, 0
	}
	age := now.Sub(sourceObservedAt.UTC())
	if age < 0 {
		age = 0
	}
	ageSeconds = int64(age.Seconds())
	switch {
	case age <= policy.FreshThreshold:
		return ETAFreshnessFresh, ageSeconds
	case age <= policy.StaleThreshold:
		return ETAFreshnessStale, ageSeconds
	default:
		return ETAFreshnessExpired, ageSeconds
	}
}

func DeriveETAStatus(hasObservation bool, freshnessStatus string, completed bool) string {
	if completed {
		return ETAStatusCompleted
	}
	if !hasObservation {
		return ETAStatusUnavailable
	}
	switch freshnessStatus {
	case ETAFreshnessFresh:
		return ETAStatusAvailable
	case ETAFreshnessStale:
		return ETAStatusStale
	case ETAFreshnessExpired:
		return ETAStatusExpired
	default:
		return ETAStatusUnavailable
	}
}

func ETAUsableForRisk(status string) bool {
	return status == ETAStatusAvailable || status == ETAStatusStale
}
