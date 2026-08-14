package domain

import "time"

const (
	maxReceiptDelayDegraded = 5 * time.Minute
	maxReceiptDelayPoor     = 30 * time.Minute
	maxImpliedSpeedKph      = 300.0
	poorAccuracyMeters      = 500.0
	degradedAccuracyMeters  = 100.0
)

func EvaluateQuality(
	freshnessStatus string,
	accuracyMeters *float64,
	receiptDelay time.Duration,
	prevLat, prevLon *float64,
	prevRecordedAt *time.Time,
	latitude, longitude float64,
	recordedAt time.Time,
) (status string, reason *string) {
	if freshnessStatus == FreshnessUnknown {
		return QualityUnknown, nil
	}

	if prevLat != nil && prevLon != nil && prevRecordedAt != nil {
		elapsed := recordedAt.Sub(*prevRecordedAt)
		if elapsed > 0 {
			distanceKm := HaversineDistanceKm(*prevLat, *prevLon, latitude, longitude)
			hours := elapsed.Hours()
			if hours > 0 {
				impliedSpeed := distanceKm / hours
				if impliedSpeed > maxImpliedSpeedKph {
					r := "impossible_movement"
					return QualityPoor, &r
				}
			}
		}
	}

	if receiptDelay >= maxReceiptDelayPoor {
		r := "high_receipt_delay"
		return QualityPoor, &r
	}
	if freshnessStatus == FreshnessLost {
		r := "telemetry_lost"
		return QualityPoor, &r
	}
	if accuracyMeters != nil {
		if *accuracyMeters >= poorAccuracyMeters {
			r := "poor_accuracy"
			return QualityPoor, &r
		}
		if *accuracyMeters >= degradedAccuracyMeters {
			r := "degraded_accuracy"
			return QualityDegraded, &r
		}
	}
	if receiptDelay >= maxReceiptDelayDegraded {
		r := "receipt_delay"
		return QualityDegraded, &r
	}
	if freshnessStatus == FreshnessStale {
		r := "telemetry_stale"
		return QualityDegraded, &r
	}
	return QualityGood, nil
}
