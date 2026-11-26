package allocationpolicy

import (
	"fmt"
	"strings"
)

// Permission names (W1_ABAC_AUTHORIZATION_MATRIX §3).
const (
	PermPolicyManage        = "allocation.policy.manage"
	PermPolicyReadOwn       = "allocation.policy.read_own"
	PermPolicyReadPublished = "allocation.policy.read_published"
	PermTenderRead          = "tender.read"
	PermAllocationSimulate  = "allocation.simulate"
	PermEvaluationRead      = "allocation.evaluation.read"
)

// ResourceStatusDraft / Published gate draft vs published reads.
const (
	ResourceStatusDraft     = "draft"
	ResourceStatusPublished = "published"
)

// AuthContext is a verified server-side authority context.
// It MUST NOT be constructed from client X-Tenant-ID / X-User-ID / X-Company-ID headers.
type AuthContext struct {
	TenantID            string
	ActorID             string
	CompanyID           string
	MembershipVerified  bool
	MembershipActive    bool
	Permissions         map[string]struct{}
	TrustedServerOrigin bool // must be true; forged/header-built contexts fail closed
}

// NewServerAuthContext builds an AuthContext from already-verified server identity.
// Callers must pass only gateway/JWT-resolved values — never raw client headers.
func NewServerAuthContext(tenantID, actorID, companyID string, membershipVerified, membershipActive bool, perms []string) (AuthContext, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(actorID) == "" {
		return AuthContext{}, authErr("MISSING_AUTH_CONTEXT", "tenant_id and actor_id are required")
	}
	if strings.TrimSpace(companyID) == "" {
		return AuthContext{}, authErr("MISSING_COMPANY", "company_id is required")
	}
	m := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		m[p] = struct{}{}
	}
	return AuthContext{
		TenantID:            tenantID,
		ActorID:             actorID,
		CompanyID:           companyID,
		MembershipVerified:  membershipVerified,
		MembershipActive:    membershipActive,
		Permissions:         m,
		TrustedServerOrigin: true,
	}, nil
}

func authErr(code, msg string) error {
	return decodeErr(code, msg)
}

// EvalAuthRequest carries authorization inputs for Evaluate (in-process only).
type EvalAuthRequest struct {
	Auth                   AuthContext
	CampaignOwnerCompanyID string
	PolicyOwnerCompanyID   string
	ResourceStatus         string
	PolicyVersion          string
	ExpectedPolicyVersion  string
	CandidateTenantID      string
}

func authorizeEvaluate(req EvalAuthRequest) error {
	a := req.Auth
	if !a.TrustedServerOrigin {
		return authErr("FORGED_AUTHORITY", "AuthContext must come from verified server-side origin")
	}
	if strings.TrimSpace(a.TenantID) == "" || strings.TrimSpace(a.ActorID) == "" {
		return authErr("MISSING_AUTH_CONTEXT", "AuthContext tenant/actor missing")
	}
	if strings.TrimSpace(a.CompanyID) == "" {
		return authErr("MISSING_COMPANY", "AuthContext company missing")
	}
	if !a.MembershipVerified || !a.MembershipActive {
		return authErr("MEMBERSHIP_DENIED", "membership not verified or revoked")
	}
	if req.CandidateTenantID != "" && req.CandidateTenantID != a.TenantID {
		return authErr("TENANT_MISMATCH", "candidate tenant_id does not match AuthContext")
	}
	if req.CampaignOwnerCompanyID != "" && req.CampaignOwnerCompanyID != a.CompanyID {
		return authErr("FOREIGN_COMPANY", "campaign owner company mismatch")
	}
	if req.PolicyOwnerCompanyID != "" && req.PolicyOwnerCompanyID != a.CompanyID {
		return authErr("FOREIGN_COMPANY", "policy owner company mismatch")
	}
	if req.ExpectedPolicyVersion != "" && req.PolicyVersion != req.ExpectedPolicyVersion {
		return authErr("STALE_POLICY_VERSION", "policy version precondition failed")
	}

	status := req.ResourceStatus
	if status == "" {
		status = ResourceStatusDraft
	}
	switch status {
	case ResourceStatusDraft:
		if !hasPerm(a, PermPolicyManage) && !hasPerm(a, PermPolicyReadOwn) && !hasPerm(a, PermAllocationSimulate) {
			if hasPerm(a, PermTenderRead) {
				return authErr("DRAFT_DENIED", "tender.read alone is not sufficient for draft policy evaluation")
			}
			return authErr("PERMISSION_DENIED", "missing draft evaluation permission")
		}
	case ResourceStatusPublished:
		if !hasPerm(a, PermPolicyReadPublished) && !hasPerm(a, PermAllocationSimulate) &&
			!hasPerm(a, PermPolicyManage) && !hasPerm(a, PermEvaluationRead) {
			return authErr("PERMISSION_DENIED", "missing published evaluation permission")
		}
	default:
		return authErr("UNKNOWN_RESOURCE_STATUS", fmt.Sprintf("unknown resource_status %q", status))
	}
	return nil
}

func hasPerm(a AuthContext, p string) bool {
	_, ok := a.Permissions[p]
	return ok
}
