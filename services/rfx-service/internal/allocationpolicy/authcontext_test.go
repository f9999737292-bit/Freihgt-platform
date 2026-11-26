package allocationpolicy

import "testing"

func testAuth(t *testing.T, perms ...string) AuthContext {
	t.Helper()
	a, err := NewServerAuthContext("t1", "actor1", "co1", true, true, perms)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAuth_ABACNegatives(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*EvalAuthRequest)
		code string
	}{
		{"foreign_tenant", func(r *EvalAuthRequest) { r.CandidateTenantID = "other" }, "TENANT_MISMATCH"},
		{"foreign_company", func(r *EvalAuthRequest) { r.CampaignOwnerCompanyID = "other" }, "FOREIGN_COMPANY"},
		{"revoked", func(r *EvalAuthRequest) { r.Auth.MembershipActive = false }, "MEMBERSHIP_DENIED"},
		{"forged", func(r *EvalAuthRequest) { r.Auth.TrustedServerOrigin = false }, "FORGED_AUTHORITY"},
		{"missing", func(r *EvalAuthRequest) { r.Auth.ActorID = "" }, "MISSING_AUTH_CONTEXT"},
		{"tender_read_draft", func(r *EvalAuthRequest) {
			r.Auth.Permissions = map[string]struct{}{PermTenderRead: {}}
			r.ResourceStatus = ResourceStatusDraft
		}, "DRAFT_DENIED"},
		{"stale_version", func(r *EvalAuthRequest) {
			r.PolicyVersion = "1"
			r.ExpectedPolicyVersion = "2"
		}, "STALE_POLICY_VERSION"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := EvalAuthRequest{
				Auth:                   testAuth(t, PermAllocationSimulate),
				CampaignOwnerCompanyID: "co1",
				PolicyOwnerCompanyID:   "co1",
				ResourceStatus:         ResourceStatusDraft,
				CandidateTenantID:      "t1",
			}
			tc.mut(&req)
			err := authorizeEvaluate(req)
			assertDecodeCode(t, err, tc.code)
		})
	}
}
