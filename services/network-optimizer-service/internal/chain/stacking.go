package chain

func stackingDecision(overlap bool, left, right *bool) (reject bool, reason string) {
	if !overlap {
		return false, ""
	}
	if left == nil || right == nil {
		return true, ReasonStackingUnknown
	}
	if !*left || !*right {
		return true, ReasonStackingRejected
	}
	return false, ""
}
