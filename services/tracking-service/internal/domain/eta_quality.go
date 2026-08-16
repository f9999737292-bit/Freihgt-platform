package domain

import "time"

func EvaluateETAQuality(freshnessStatus, sourceType string, deliveryLag time.Duration, providerConfidence *float64) (status string, reasons []string) {
	reasons = make([]string, 0, 4)
	if freshnessStatus == ETAFreshnessUnknown {
		return ETAQualityUnknown, reasons
	}
	if freshnessStatus == ETAFreshnessExpired {
		reasons = append(reasons, "stale_source")
		return ETAQualityPoor, reasons
	}
	if deliveryLag >= 30*time.Minute {
		reasons = append(reasons, "high_delivery_lag")
		return ETAQualityPoor, reasons
	}
	if sourceType == ETASourceManualOperator {
		reasons = append(reasons, "manual_source")
		return ETAQualityDegraded, reasons
	}
	if freshnessStatus == ETAFreshnessStale {
		reasons = append(reasons, "stale_source")
		return ETAQualityDegraded, reasons
	}
	if deliveryLag >= 5*time.Minute {
		reasons = append(reasons, "receipt_delay")
		return ETAQualityDegraded, reasons
	}
	if providerConfidence != nil && *providerConfidence < 0.5 {
		reasons = append(reasons, "provider_quality_degraded")
		return ETAQualityDegraded, reasons
	}
	reasons = append(reasons, "fresh_source")
	return ETAQualityGood, reasons
}
