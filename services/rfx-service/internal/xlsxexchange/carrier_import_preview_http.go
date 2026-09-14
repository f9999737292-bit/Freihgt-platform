package xlsxexchange

// ClassifyCarrierPreviewErrors decides whether invalid carrier preview issues are structural (400) or domain (422).
// Carrier preview maps competitor columns, answer/offer validation, and metadata mismatches to domain (422).
func ClassifyCarrierPreviewErrors(errors []BuyerImportIssue) PreviewErrorClass {
	if len(errors) == 0 {
		return PreviewErrorClassNone
	}
	for _, issue := range errors {
		if _, structural := carrierStructuralPreviewMachineCodes[issue.MachineCode]; !structural {
			return PreviewErrorClassDomain
		}
	}
	return PreviewErrorClassStructural
}

var carrierStructuralPreviewMachineCodes = map[string]struct{}{
	MachineCodeInvalidXLSXSignature: {},
	MachineCodeUnsafePackage:        {},
	MachineCodeMissingSheet:         {},
	MachineCodeUnexpectedSheet:      {},
	MachineCodeFormulaDenied:        {},
	MachineCodeMergedCellDenied:     {},
	MachineCodeHiddenSheetDenied:    {},
	MachineCodeTooManyRows:          {},
	MachineCodeTooManyCells:         {},
	MachineCodeFileTooLarge:         {},
	MachineCodeInvalidMultipart:     {},
}
