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

var createStructuralPreviewMachineCodes = map[string]struct{}{
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
	MachineCodeFileTooLarge:           {},
	MachineCodeInvalidMultipart:       {},
}

// ClassifyCreatePreviewErrors maps CREATE_NEW_DRAFT preview issues.
// Structural/unsafe/schema failures are 400; domain-invalid shells/graphs are 422.
func ClassifyCreatePreviewErrors(errors []BuyerImportIssue) PreviewErrorClass {
	if len(errors) == 0 {
		return PreviewErrorClassNone
	}
	for _, issue := range errors {
		if !isCreateStructuralIssue(issue) {
			return PreviewErrorClassDomain
		}
	}
	return PreviewErrorClassStructural
}

func isCreateStructuralIssue(issue BuyerImportIssue) bool {
	if _, ok := createStructuralPreviewMachineCodes[issue.MachineCode]; ok {
		return true
	}
	if issue.Sheet == sheetMetadata && (issue.MachineCode == MachineCodeMissingRequiredValue || issue.MachineCode == MachineCodeInvalidType) {
		return true
	}
	return false
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
