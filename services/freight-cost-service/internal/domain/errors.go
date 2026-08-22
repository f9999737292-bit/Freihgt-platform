package domain

import "errors"

var (
	ErrReconciliationMutatedProjection      = errors.New("reconciliation must not mutate projection")
	ErrReclassificationChangedFinancialAmounts = errors.New("reclassification must not change financial amounts")
)
