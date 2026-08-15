package tender

import "fmt"

const (
	AwardProposalDraft           = "DRAFT_PROPOSAL"
	AwardProposalPendingApproval = "PENDING_APPROVAL"
	AwardProposalApproved        = "APPROVED"
	AwardProposalRejected        = "REJECTED"
	AwardProposalAwarded         = "AWARDED"
)

type AwardProposalInput struct {
	Lines []AllocationLine `json:"lines"`
}

func ValidateAwardProposalTransition(current, next string) error {
	allowed := map[string]map[string]struct{}{
		AwardProposalDraft: {
			AwardProposalPendingApproval: {},
		},
		AwardProposalPendingApproval: {
			AwardProposalApproved: {},
			AwardProposalRejected: {},
		},
		AwardProposalApproved: {
			AwardProposalAwarded: {},
		},
	}
	nextSet, ok := allowed[current]
	if !ok {
		return fmt.Errorf("award proposal cannot transition from %s", current)
	}
	if _, ok := nextSet[next]; !ok {
		return fmt.Errorf("invalid award proposal transition %s -> %s", current, next)
	}
	return nil
}

func ValidateFinalizeAward(proposalStatus string) error {
	if proposalStatus != AwardProposalApproved {
		return fmt.Errorf("award can only be finalized from APPROVED proposal, got %s", proposalStatus)
	}
	return nil
}
