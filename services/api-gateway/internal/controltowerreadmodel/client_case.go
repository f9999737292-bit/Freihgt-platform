package controltowerreadmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	listCasesPath      = "/internal/v1/control-tower/cases"
	casePath           = "/internal/v1/control-tower/cases/%s"
	caseClaimPath      = "/internal/v1/control-tower/cases/%s/claim"
	caseAssignPath     = "/internal/v1/control-tower/cases/%s/assign"
	caseUnassignPath   = "/internal/v1/control-tower/cases/%s/unassign"
	caseLinksPath      = "/internal/v1/control-tower/cases/%s/links"
	caseLinkPath       = "/internal/v1/control-tower/cases/%s/links/%s"
	caseNotesPath      = "/internal/v1/control-tower/cases/%s/notes"
	caseNotePath       = "/internal/v1/control-tower/cases/%s/notes/%s"
	caseActionsPath    = "/internal/v1/control-tower/cases/%s/actions"
	caseActionPath     = "/internal/v1/control-tower/cases/%s/actions/%s"
	caseActionComplete = "/internal/v1/control-tower/cases/%s/actions/%s/complete"
	caseDecisionsPath  = "/internal/v1/control-tower/cases/%s/decisions"
	caseResolvePath    = "/internal/v1/control-tower/cases/%s/resolve"
	caseClosePath      = "/internal/v1/control-tower/cases/%s/close"
	caseReopenPath     = "/internal/v1/control-tower/cases/%s/reopen"
	CaseTimelinePath   = "/internal/v1/control-tower/cases/%s/timeline"
	CaseKPIPath        = "/internal/v1/control-tower/cases/kpi"
	CaseDuplicatesPath = "/internal/v1/control-tower/cases/duplicates"
)

type CasesFilter struct {
	Status         string
	Severity       string
	OwnerUserID    string
	ShipmentID     string
	Search         string
	Preset         string
	SlaState       string
	MyCases        bool
	Unassigned     bool
	IncludeClosed  bool
	HasSlaBreach   bool
	HasSlaWarning  bool
	OverdueActions bool
	HasOpenActions bool
	Page           int
	Limit          int
}

func (c *Client) ListCases(ctx context.Context, tenantID, userID, requestID string, filter CasesFilter) (json.RawMessage, *DependencyError) {
	q := url.Values{}
	setIf(q, "status", filter.Status)
	setIf(q, "severity", filter.Severity)
	setIf(q, "ownerUserId", filter.OwnerUserID)
	setIf(q, "shipmentId", filter.ShipmentID)
	setIf(q, "search", filter.Search)
	setIf(q, "preset", filter.Preset)
	if filter.MyCases {
		q.Set("myCases", "true")
	}
	if filter.Unassigned {
		q.Set("unassigned", "true")
	}
	if filter.IncludeClosed {
		q.Set("includeClosed", "true")
	}
	if filter.HasSlaBreach {
		q.Set("hasSlaBreach", "true")
	}
	if filter.HasSlaWarning {
		q.Set("hasSlaWarning", "true")
	}
	if filter.OverdueActions {
		q.Set("overdueActions", "true")
	}
	if filter.HasOpenActions {
		q.Set("hasOpenActions", "true")
	}
	setIf(q, "slaState", filter.SlaState)
	if filter.Page > 0 {
		q.Set("page", fmt.Sprintf("%d", filter.Page))
	}
	if filter.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	var payload json.RawMessage
	if err := c.doWorkspaceJSON(ctx, http.MethodGet, listCasesPath+"?"+q.Encode(), tenantID, userID, requestID, nil, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) GetCase(ctx context.Context, tenantID, userID, requestID, caseID string) (json.RawMessage, *DependencyError) {
	var payload json.RawMessage
	path := fmt.Sprintf(casePath, url.PathEscape(caseID))
	if err := c.doWorkspaceJSON(ctx, http.MethodGet, path, tenantID, userID, requestID, nil, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) ProxyCaseJSON(ctx context.Context, method, tenantID, userID, requestID, path string, body []byte) (json.RawMessage, *DependencyError) {
	var payload json.RawMessage
	if err := c.doWorkspaceJSON(ctx, method, path, tenantID, userID, requestID, body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) ProxyCaseNoContent(ctx context.Context, method, tenantID, userID, requestID, path string, body []byte) *DependencyError {
	return c.doWorkspaceJSON(ctx, method, path, tenantID, userID, requestID, body, nil)
}

func CasePath(caseID string, suffix string) string {
	base := fmt.Sprintf(casePath, url.PathEscape(caseID))
	if suffix == "" {
		return base
	}
	return base + "/" + strings.TrimPrefix(suffix, "/")
}
