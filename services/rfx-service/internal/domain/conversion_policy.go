package domain

import "strings"

const (
	ConversionImmediateOrder       = "IMMEDIATE_ORDER"
	ConversionAllocationAgreement  = "ALLOCATION_AGREEMENT"
	ConversionNoAutomatic          = "NO_AUTOMATIC_CONVERSION"
)

var immediateOrderTypes = map[string]struct{}{
	"MINI_TENDER": {}, "RFQ": {}, "SPOT_RFQ": {},
}

var allocationAgreementTypes = map[string]struct{}{
	"LANE_TENDER": {}, "CONTRACT_TENDER": {}, "RFP": {}, "RFG": {}, "RFT": {},
	"SEASONAL_TENDER": {}, "PROJECT_TENDER": {},
}

func TenderConversionPolicy(rfxType string) string {
	t := strings.TrimSpace(strings.ToUpper(rfxType))
	if _, ok := immediateOrderTypes[t]; ok {
		return ConversionImmediateOrder
	}
	if _, ok := allocationAgreementTypes[t]; ok {
		return ConversionAllocationAgreement
	}
	return ConversionNoAutomatic
}
