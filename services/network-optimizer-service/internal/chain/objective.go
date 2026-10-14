package chain

// Score is one finished feasible order. Nil components are omitted, not zero.
type Score struct {
	EmptyRoadMeters int
	DurationSeconds int
	Utilization     *float64
	TargetMeters    *int
	Revenue         *int64
	MinLoadID       string
	MaxLoadID       string
	Order           string
}

// Better reports whether a ranks ahead of b.
// Missing revenue, utilization, or target distance does not compare as zero.
func Better(a, b Score) bool {
	if a.EmptyRoadMeters != b.EmptyRoadMeters {
		return a.EmptyRoadMeters < b.EmptyRoadMeters
	}
	if a.DurationSeconds != b.DurationSeconds {
		return a.DurationSeconds < b.DurationSeconds
	}
	if a.Utilization != nil && b.Utilization != nil && *a.Utilization != *b.Utilization {
		return *a.Utilization > *b.Utilization
	}
	if a.TargetMeters != nil && b.TargetMeters != nil && *a.TargetMeters != *b.TargetMeters {
		return *a.TargetMeters < *b.TargetMeters
	}
	if a.Revenue != nil && b.Revenue != nil && *a.Revenue != *b.Revenue {
		return *a.Revenue > *b.Revenue
	}
	if a.MinLoadID != b.MinLoadID {
		return a.MinLoadID < b.MinLoadID
	}
	if a.MaxLoadID != b.MaxLoadID {
		return a.MaxLoadID < b.MaxLoadID
	}
	return orderRank(a.Order) < orderRank(b.Order)
}

func orderRank(order string) int {
	if order == OrderLowerFirst {
		return 0
	}
	return 1
}

// OmitShared drops a component for the whole comparison when any score lacks it.
func OmitShared(scores []Score) []Score {
	out := append([]Score(nil), scores...)
	if sharedMissing(out, func(s Score) bool { return s.Utilization == nil }) {
		for i := range out {
			out[i].Utilization = nil
		}
	}
	if sharedMissing(out, func(s Score) bool { return s.TargetMeters == nil }) {
		for i := range out {
			out[i].TargetMeters = nil
		}
	}
	if sharedMissing(out, func(s Score) bool { return s.Revenue == nil }) {
		for i := range out {
			out[i].Revenue = nil
		}
	}
	return out
}

func sharedMissing(scores []Score, missing func(Score) bool) bool {
	for _, score := range scores {
		if missing(score) {
			return true
		}
	}
	return false
}
