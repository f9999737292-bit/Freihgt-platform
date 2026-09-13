package xlsxexchange

import "context"

// ValidateCommitProposal re-runs domain validation for a stored import proposal at commit time.
func ValidateCommitProposal(ctx context.Context, target TargetDraftBaseline, proposal BuyerImportProposal) []BuyerImportIssue {
	p := newBuyerImportParser(ctx, productionParseConfig())
	p.validateProposalGraph(target, proposal)
	return p.issues.errors
}
