package xlsxexchange

import "testing"

func TestClassifyPreviewErrorsStructuralVsDomain(t *testing.T) {
	t.Parallel()
	if ClassifyPreviewErrors([]BuyerImportIssue{{MachineCode: MachineCodeMissingSheet}}) != PreviewErrorClassStructural {
		t.Fatal("expected structural class")
	}
	if ClassifyPreviewErrors([]BuyerImportIssue{{MachineCode: MachineCodeDuplicateStableCode}}) != PreviewErrorClassDomain {
		t.Fatal("expected domain class")
	}
	if ClassifyPreviewErrors([]BuyerImportIssue{
		{MachineCode: MachineCodeMissingSheet},
		{MachineCode: MachineCodeCyclicRule},
	}) != PreviewErrorClassDomain {
		t.Fatal("mixed issues must classify as domain")
	}
}
