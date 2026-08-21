package domain

func CalculateAccrual(planned *Money, approvedAccessorials []Money) (*Money, error) {
	if planned == nil {
		return nil, nil
	}
	if len(approvedAccessorials) == 0 {
		copy := *planned
		return &copy, nil
	}
	return SumMoney(*planned, approvedAccessorials)
}

func CalculateForecastExposure(planned *Money, proposedAccessorials []Money) (*Money, error) {
	if planned == nil {
		return nil, nil
	}
	if len(proposedAccessorials) == 0 {
		copy := *planned
		return &copy, nil
	}
	return SumMoney(*planned, proposedAccessorials)
}
