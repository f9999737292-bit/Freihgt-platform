package xlsxexchange

// ClassifyCarrierPreviewErrors decides whether invalid carrier preview issues are structural (400) or domain (422).
func ClassifyCarrierPreviewErrors(errors []BuyerImportIssue) PreviewErrorClass {
	if len(errors) == 0 {
		return PreviewErrorClassNone
	}
	for _, issue := range errors {
		if issue.MachineCode == MachineCodeUnsupportedSchema {
			return PreviewErrorClassDomain
		}
		if _, structural := structuralPreviewMachineCodes[issue.MachineCode]; !structural {
			return PreviewErrorClassDomain
		}
	}
	return PreviewErrorClassStructural
}
