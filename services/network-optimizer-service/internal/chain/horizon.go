package chain

import "time"

// Window is an absolute UTC instant pair. A missing or zoned value fails closed.
type Window struct {
	Start time.Time
	End   time.Time
}

func absoluteUTC(t time.Time) bool {
	return !t.IsZero() && t.Location() == time.UTC
}

func horizonBounds(seed Seed, clock time.Time) (time.Time, time.Time, bool) {
	if !absoluteUTC(clock) {
		return time.Time{}, time.Time{}, false
	}
	start := clock.UTC()
	if seed.Source == SeedManualAvailable && seed.AvailableFrom != nil {
		if !absoluteUTC(*seed.AvailableFrom) {
			return time.Time{}, time.Time{}, false
		}
		start = seed.AvailableFrom.UTC()
	}
	end := start.Add(Horizon)
	if seed.Source == SeedManualAvailable && seed.AvailableUntil != nil {
		if !absoluteUTC(*seed.AvailableUntil) {
			return time.Time{}, time.Time{}, false
		}
		if seed.AvailableUntil.Before(end) {
			end = seed.AvailableUntil.UTC()
		}
	}
	if !end.After(start) {
		return start, start, true
	}
	return start, end, true
}

func insideHorizon(load VisibleLoad, start, end time.Time) (bool, string) {
	if !absoluteUTC(load.PickupStart) || !absoluteUTC(load.PickupEnd) || !absoluteUTC(load.DeliveryStart) || !absoluteUTC(load.DeliveryEnd) {
		return false, ReasonWindowNotAbsolute
	}
	if load.PickupEnd.Before(start) {
		return false, ReasonOutsideHorizon
	}
	if !load.PickupStart.Before(end) {
		return false, ReasonOutsideHorizon
	}
	if !load.DeliveryStart.Before(end) {
		return false, ReasonOutsideHorizon
	}
	return true, ""
}
