package xlsxexchange

// PreviewErrorClass classifies parser issues for HTTP mapping.
type PreviewErrorClass int

const (
	PreviewErrorClassNone PreviewErrorClass = iota
	PreviewErrorClassStructural
	PreviewErrorClassDomain
)

var structuralPreviewMachineCodes = map[string]struct{}{
	MachineCodeInvalidXLSXSignature:   {},
	MachineCodeUnsafePackage:          {},
	MachineCodeUnsupportedSchema:      {},
	MachineCodeMissingSheet:           {},
	MachineCodeUnexpectedSheet:        {},
	MachineCodeInvalidHeader:          {},
	MachineCodeFormulaDenied:          {},
	MachineCodeMergedCellDenied:       {},
	MachineCodeHiddenSheetDenied:      {},
	MachineCodeCompetitorColumnDenied: {},
	MachineCodeTooManyRows:            {},
	MachineCodeTooManyCells:           {},
	MachineCodeInvalidType:            {},
	MachineCodeMissingRequiredValue:   {},
	MachineCodeFileTooLarge:           {},
	MachineCodeInvalidMultipart:       {},
}

// ClassifyPreviewErrors decides whether invalid preview issues are structural (400) or domain (422).
// Mixed lists containing any domain-class issue return PreviewErrorClassDomain.
func ClassifyPreviewErrors(errors []BuyerImportIssue) PreviewErrorClass {
	if len(errors) == 0 {
		return PreviewErrorClassNone
	}
	for _, issue := range errors {
		if _, structural := structuralPreviewMachineCodes[issue.MachineCode]; !structural {
			return PreviewErrorClassDomain
		}
	}
	return PreviewErrorClassStructural
}
