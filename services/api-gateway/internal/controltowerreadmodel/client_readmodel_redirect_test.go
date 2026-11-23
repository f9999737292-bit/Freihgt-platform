package controltowerreadmodel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

const (
	readModelBase    = "http://read-model.test:8080"
	readModelHost    = "read-model.test:8080"
	redirectTenant   = "tenant-r3-secret"
	redirectUser     = "user-r3-secret"
	redirectRequest  = "request-r3-secret"
	metadataRedirect = "http://169.254.169.254/latest/meta-data"
	foreignRedirect  = "http://evil.example/collect"
)

type safetyHop struct {
	method     string
	url        string
	host       string
	hostHeader string
	tenant     string
	user       string
	requestID  string
	body       string
}

type safetyTransport struct {
	mu      sync.Mutex
	hops    []safetyHop
	respond func(*http.Request) (*http.Response, error)
}

func (t *safetyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	t.mu.Lock()
	t.hops = append(t.hops, safetyHop{
		method:     req.Method,
		url:        req.URL.String(),
		host:       req.URL.Host,
		hostHeader: req.Host,
		tenant:     req.Header.Get("X-Tenant-ID"),
		user:       req.Header.Get("X-User-ID"),
		requestID:  req.Header.Get(sharedmiddleware.RequestIDHeader),
		body:       string(body),
	})
	t.mu.Unlock()
	if t.respond == nil {
		return redirectResponse(req, http.StatusBadGateway, "", ""), nil
	}
	return t.respond(req)
}

func (t *safetyTransport) snapshot() []safetyHop {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]safetyHop, len(t.hops))
	copy(out, t.hops)
	return out
}

func newSafetyClient(base string, rt *safetyTransport, check func(*http.Request, []*http.Request) error) *Client {
	return NewClient(&http.Client{Transport: rt, Timeout: time.Second, CheckRedirect: check}, Config{BaseURL: base, Timeout: time.Second}, NewMetrics())
}

func redirectTo(target string, status int) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		return redirectResponse(req, status, target, ""), nil
	}
}

func assertNoForbiddenTarget(t *testing.T, hops []safetyHop, allowedHost string) {
	t.Helper()
	forbidden := 0
	for _, hop := range hops {
		if hop.host != allowedHost {
			forbidden++
			t.Errorf("outbound request to %s (%s)", hop.host, hop.url)
		}
	}
	if forbidden != 0 {
		t.Fatalf("forbidden target request count = %d", forbidden)
	}
}

func TestReadModelMethodsRejectForeignRedirect(t *testing.T) {
	ctx := context.Background()
	calls := []struct {
		name string
		call func(context.Context, *Client) *DependencyError
	}{
		{"FetchStatusSummary", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.FetchStatusSummary(ctx, ModeShadow, redirectTenant, redirectRequest)
			return err
		}},
		{"AcknowledgeCriticalEvent", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.AcknowledgeCriticalEvent(ctx, AcknowledgeCriticalEventInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, EventID: "evt-1", ShipmentID: "shp-1"})
			return err
		}},
		{"LookupAcknowledgements", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.LookupAcknowledgements(ctx, redirectTenant, redirectRequest, []string{"evt-1"})
			return err
		}},
		{"AssignCriticalEvent", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.AssignCriticalEvent(ctx, AssignCriticalEventInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, EventID: "evt-1", AssignedToUser: redirectUser})
			return err
		}},
		{"ResolveCriticalEvent", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ResolveCriticalEvent(ctx, ResolveCriticalEventInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, EventID: "evt-1", ResolutionCode: "done"})
			return err
		}},
		{"ReopenCriticalEvent", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ReopenCriticalEvent(ctx, ReopenCriticalEventInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, EventID: "evt-1"})
			return err
		}},
		{"ListCriticalEventActions", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ListCriticalEventActions(ctx, redirectTenant, redirectUser, redirectRequest, "evt-1")
			return err
		}},
		{"LookupWorkflows", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.LookupWorkflows(ctx, redirectTenant, redirectRequest, []string{"evt-1"})
			return err
		}},
		{"SyncRisks", func(ctx context.Context, c *Client) *DependencyError {
			return c.SyncRisks(ctx, SyncRisksInput{TenantID: redirectTenant, RequestID: redirectRequest, Evaluations: []SyncRiskEvaluation{{RiskKey: "risk-1"}}})
		}},
		{"ListRisks", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ListRisks(ctx, redirectTenant, redirectRequest, ListRisksFilter{})
			return err
		}},
		{"GetRisk", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.GetRisk(ctx, redirectTenant, redirectRequest, "risk-1")
			return err
		}},
		{"GetRiskKPI", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.GetRiskKPI(ctx, redirectTenant, redirectRequest)
			return err
		}},
		{"AcknowledgeRisk", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.AcknowledgeRisk(ctx, AcknowledgeRiskInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, RiskKey: "risk-1"})
			return err
		}},
		{"MitigateRisk", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.MitigateRisk(ctx, MitigateRiskInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, RiskKey: "risk-1", MitigationCode: "done"})
			return err
		}},
		{"EnsureExceptionWorkflows", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.EnsureExceptionWorkflows(ctx, redirectTenant, redirectRequest, []EnsureExceptionSeed{{EventID: "evt-1"}})
			return err
		}},
		{"UpdateException", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.UpdateException(ctx, UpdateExceptionInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, EventID: "evt-1"})
			return err
		}},
		{"ListOpenWorkflowsBySource", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ListOpenWorkflowsBySource(ctx, redirectTenant, redirectRequest, "control-tower")
			return err
		}},
		{"ListWorkItems", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ListWorkItems(ctx, redirectTenant, redirectUser, redirectRequest, WorkItemsFilter{})
			return err
		}},
		{"GetWorkItem", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.GetWorkItem(ctx, redirectTenant, redirectUser, redirectRequest, "exception", "item-1")
			return err
		}},
		{"ClaimWorkItem", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ClaimWorkItem(ctx, WorkItemMutationInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, ItemType: "exception", ItemID: "item-1"})
			return err
		}},
		{"AssignWorkItem", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.AssignWorkItem(ctx, AssignWorkItemInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, ItemType: "exception", ItemID: "item-1"})
			return err
		}},
		{"UnassignWorkItem", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.UnassignWorkItem(ctx, WorkItemMutationInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, ItemType: "exception", ItemID: "item-1"})
			return err
		}},
		{"BulkWorkItemsAction", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.BulkWorkItemsAction(ctx, BulkActionInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, Action: "claim"})
			return err
		}},
		{"GetWorkload", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.GetWorkload(ctx, redirectTenant, redirectUser, redirectRequest)
			return err
		}},
		{"ListViews", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ListViews(ctx, redirectTenant, redirectUser, redirectRequest, "team")
			return err
		}},
		{"CreateView", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.CreateView(ctx, CreateViewInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest, Name: "view"})
			return err
		}},
		{"UpdateView", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.UpdateView(ctx, redirectTenant, redirectUser, redirectRequest, "view-1", map[string]any{"name": "next"})
			return err
		}},
		{"DeleteView", func(ctx context.Context, c *Client) *DependencyError {
			return c.DeleteView(ctx, redirectTenant, redirectUser, redirectRequest, "view-1")
		}},
		{"SetDefaultView", func(ctx context.Context, c *Client) *DependencyError {
			return c.SetDefaultView(ctx, redirectTenant, redirectUser, redirectRequest, "view-1")
		}},
		{"CreateHandoff", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.CreateHandoff(ctx, CreateHandoffInput{TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest})
			return err
		}},
		{"ListHandoffs", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ListHandoffs(ctx, redirectTenant, redirectUser, redirectRequest, url.Values{"limit": []string{"1"}})
			return err
		}},
		{"GetHandoff", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.GetHandoff(ctx, redirectTenant, redirectUser, redirectRequest, "handoff-1")
			return err
		}},
		{"ListCases", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ListCases(ctx, redirectTenant, redirectUser, redirectRequest, CasesFilter{})
			return err
		}},
		{"GetCase", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.GetCase(ctx, redirectTenant, redirectUser, redirectRequest, "case-1")
			return err
		}},
		{"ProxyCaseJSON", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ProxyCaseJSON(ctx, http.MethodPost, redirectTenant, redirectUser, redirectRequest, "/internal/v1/control-tower/cases", []byte(`{"title":"x"}`))
			return err
		}},
		{"ProxyCaseNoContent", func(ctx context.Context, c *Client) *DependencyError {
			return c.ProxyCaseNoContent(ctx, http.MethodDelete, redirectTenant, redirectUser, redirectRequest, "/internal/v1/control-tower/cases/case-1/links/link-1", nil)
		}},
		{"ProxyAutomationJSON", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ProxyAutomationJSON(ctx, http.MethodPost, redirectTenant, redirectUser, redirectRequest, "/internal/v1/control-tower/automation/evaluate", []byte(`{}`))
			return err
		}},
		{"ListAutomationRules", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ListAutomationRules(ctx, redirectTenant, redirectUser, redirectRequest, AutomationListFilter{})
			return err
		}},
		{"ListPlaybooks", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ListPlaybooks(ctx, redirectTenant, redirectUser, redirectRequest, AutomationListFilter{})
			return err
		}},
		{"ListRecommendations", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ListRecommendations(ctx, redirectTenant, redirectUser, redirectRequest, AutomationListFilter{})
			return err
		}},
		{"ListPlaybookExecutions", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.ListPlaybookExecutions(ctx, redirectTenant, redirectUser, redirectRequest, AutomationListFilter{})
			return err
		}},
		{"GetAutomationKPI", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.GetAutomationKPI(ctx, redirectTenant, redirectUser, redirectRequest)
			return err
		}},
		{"EvaluateAutomation", func(ctx context.Context, c *Client) *DependencyError {
			_, err := c.EvaluateAutomation(ctx, redirectTenant, redirectUser, redirectRequest, []byte(`{}`))
			return err
		}},
	}

	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			rt := &safetyTransport{respond: redirectTo(metadataRedirect, http.StatusFound)}
			client := newSafetyClient(readModelBase, rt, nil)
			depErr := tc.call(ctx, client)
			if depErr == nil || !errors.Is(depErr.Err, errWorkspaceRedirectRejected) {
				t.Fatalf("expected redirect reject, got %#v", depErr)
			}
			if strings.Contains(depErr.Error(), redirectTenant) || strings.Contains(depErr.Error(), metadataRedirect) {
				t.Fatalf("error leaked tenant or target: %s", depErr.Error())
			}
			hops := rt.snapshot()
			if len(hops) != 1 {
				t.Fatalf("outbound requests = %d, want 1", len(hops))
			}
			assertNoForbiddenTarget(t, hops, readModelHost)
		})
	}
}

func TestReadModelRedirectSafetyMatrix(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name         string
		status       int
		first        string
		second       string
		wantHops     int
		wantErr      error
		allowedHost  string
		sameOriginOK bool
	}{
		{name: "foreign-origin", status: http.StatusFound, first: foreignRedirect, wantHops: 1, wantErr: errWorkspaceRedirectRejected, allowedHost: readModelHost},
		{name: "metadata-ip", status: http.StatusFound, first: metadataRedirect, wantHops: 1, wantErr: errWorkspaceRedirectRejected, allowedHost: readModelHost},
		{name: "different-port", status: http.StatusFound, first: "http://read-model.test:9090/collect", wantHops: 1, wantErr: errWorkspaceRedirectRejected, allowedHost: readModelHost},
		{name: "different-scheme", status: http.StatusFound, first: "https://read-model.test:8080/collect", wantHops: 1, wantErr: errWorkspaceRedirectRejected, allowedHost: readModelHost},
		{name: "multi-hop-foreign", status: http.StatusFound, first: "/hop-2", second: metadataRedirect, wantHops: 2, wantErr: errWorkspaceRedirectRejected, allowedHost: readModelHost},
		{name: "same-origin", status: http.StatusFound, first: "/internal/v1/control-tower/status-summary-final", wantHops: 2, sameOriginOK: true, allowedHost: readModelHost},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			step := 0
			rt := &safetyTransport{}
			rt.respond = func(req *http.Request) (*http.Response, error) {
				step++
				if step == 1 {
					return redirectResponse(req, tc.status, tc.first, ""), nil
				}
				if tc.second != "" {
					return redirectResponse(req, http.StatusFound, tc.second, ""), nil
				}
				return redirectResponse(req, http.StatusNoContent, "", ""), nil
			}
			client := newSafetyClient(readModelBase, rt, nil)
			_, depErr := client.FetchStatusSummary(ctx, ModeShadow, redirectTenant, redirectRequest)
			hops := rt.snapshot()
			if len(hops) != tc.wantHops {
				t.Fatalf("outbound requests = %d, want %d", len(hops), tc.wantHops)
			}
			assertNoForbiddenTarget(t, hops, tc.allowedHost)
			if tc.sameOriginOK {
				if depErr != nil && (errors.Is(depErr.Err, errWorkspaceRedirectRejected) || errors.Is(depErr.Err, errWorkspaceRedirectLimit)) {
					t.Fatalf("same-origin redirect rejected: %v", depErr)
				}
				return
			}
			if depErr == nil || !errors.Is(depErr.Err, tc.wantErr) {
				t.Fatalf("got %#v, want %v", depErr, tc.wantErr)
			}
			if strings.Contains(depErr.Error(), redirectTenant) || strings.Contains(depErr.Error(), tc.first) || strings.Contains(depErr.Error(), metadataRedirect) {
				t.Fatalf("error leaked: %s", depErr.Error())
			}
		})
	}
}

func TestReadModelRedirectRestoresTrustedHeadersAndHost(t *testing.T) {
	var gotHost, gotTenant, gotUser, gotRequest string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == workloadPath {
			http.Redirect(w, r, "/internal/v1/control-tower/workload-final", http.StatusFound)
			return
		}
		gotHost = r.Host
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotRequest = r.Header.Get(sharedmiddleware.RequestIDHeader)
		_, _ = io.WriteString(w, `{"operators":[],"unassignedPool":0,"kpi":{}}`)
	}))
	defer srv.Close()

	caller := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		req.Host = "evil.example"
		req.Header.Set("X-Tenant-ID", "swapped-tenant")
		req.Header.Set("X-User-ID", "swapped-user")
		req.Header.Set(sharedmiddleware.RequestIDHeader, "swapped-request")
		if len(via) > 0 && via[0] != nil {
			via[0].Header.Set("X-Tenant-ID", "mutated-origin-tenant")
			via[0].Header.Set("X-User-ID", "mutated-origin-user")
			via[0].Header.Set(sharedmiddleware.RequestIDHeader, "mutated-origin-request")
			via[0].Host = "evil.example"
		}
		return nil
	}}
	client := NewClient(caller, Config{BaseURL: srv.URL, Timeout: time.Second}, NewMetrics())
	_, depErr := client.GetWorkload(context.Background(), redirectTenant, redirectUser, redirectRequest)
	if depErr != nil {
		t.Fatalf("allowed redirect failed: %v", depErr)
	}
	if caller.CheckRedirect == nil {
		t.Fatal("caller CheckRedirect was removed")
	}
	originHost := strings.TrimPrefix(srv.URL, "http://")
	if gotHost != originHost {
		t.Fatalf("host = %q, want %q", gotHost, originHost)
	}
	if gotTenant != redirectTenant || gotUser != redirectUser || gotRequest != redirectRequest {
		t.Fatalf("trusted headers tenant=%q user=%q request=%q", gotTenant, gotUser, gotRequest)
	}
}

func TestReadModelPostRedirectDoesNotSendBodyToForeignOrigin(t *testing.T) {
	ctx := context.Background()
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			rt := &safetyTransport{respond: redirectTo(metadataRedirect, status)}
			client := newSafetyClient(readModelBase, rt, nil)
			_, depErr := client.AcknowledgeCriticalEvent(ctx, AcknowledgeCriticalEventInput{
				TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest,
				EventID: "evt-1", ShipmentID: "body-secret-marker",
			})
			if depErr == nil || !errors.Is(depErr.Err, errWorkspaceRedirectRejected) {
				t.Fatalf("expected redirect reject, got %#v", depErr)
			}
			hops := rt.snapshot()
			if len(hops) != 1 {
				t.Fatalf("outbound requests = %d, want 1", len(hops))
			}
			assertNoForbiddenTarget(t, hops, readModelHost)
			if hops[0].method != http.MethodPost || !strings.Contains(hops[0].body, "body-secret-marker") {
				t.Fatalf("first hop did not carry the POST body: %+v", hops[0])
			}
			if strings.Contains(depErr.Error(), "body-secret-marker") || strings.Contains(depErr.Error(), metadataRedirect) {
				t.Fatalf("error leaked body or target: %s", depErr.Error())
			}
		})
	}
}

func TestReadModelSameOrigin307KeepsBody(t *testing.T) {
	step := 0
	rt := &safetyTransport{}
	rt.respond = func(req *http.Request) (*http.Response, error) {
		step++
		if step == 1 {
			return redirectResponse(req, http.StatusTemporaryRedirect, "/ack-final", ""), nil
		}
		return redirectResponse(req, http.StatusNoContent, "", ""), nil
	}
	client := newSafetyClient(readModelBase, rt, nil)
	_, _ = client.AcknowledgeCriticalEvent(context.Background(), AcknowledgeCriticalEventInput{
		TenantID: redirectTenant, UserID: redirectUser, RequestID: redirectRequest,
		EventID: "evt-1", ShipmentID: "body-secret-marker",
	})
	hops := rt.snapshot()
	if len(hops) != 2 {
		t.Fatalf("outbound requests = %d, want 2", len(hops))
	}
	assertNoForbiddenTarget(t, hops, readModelHost)
	if hops[1].method != http.MethodPost || !strings.Contains(hops[1].body, "body-secret-marker") {
		t.Fatalf("same-origin 307 dropped the body: %+v", hops[1])
	}
	if hops[1].tenant != redirectTenant || hops[1].user != redirectUser || hops[1].requestID != redirectRequest {
		t.Fatalf("trusted headers were not kept: %+v", hops[1])
	}
}

func TestReadModelRedirectLoopIsBounded(t *testing.T) {
	rt := &safetyTransport{respond: redirectTo(statusSummaryPath+"?tenant="+redirectTenant, http.StatusFound)}
	client := newSafetyClient(readModelBase, rt, func(*http.Request, []*http.Request) error { return nil })
	_, depErr := client.FetchStatusSummary(context.Background(), ModeShadow, redirectTenant, redirectRequest)
	if depErr == nil || !errors.Is(depErr.Err, errWorkspaceRedirectLimit) {
		t.Fatalf("expected redirect limit, got %#v", depErr)
	}
	hops := rt.snapshot()
	if len(hops) != workspaceRedirectLimit {
		t.Fatalf("outbound requests = %d, want %d", len(hops), workspaceRedirectLimit)
	}
	assertNoForbiddenTarget(t, hops, readModelHost)
	if strings.Contains(depErr.Error(), redirectTenant) || strings.Contains(depErr.Error(), readModelBase) {
		t.Fatalf("error leaked: %s", depErr.Error())
	}
}

func TestDefaultPortNormalization(t *testing.T) {
	positive := []struct {
		base string
		next string
		host string
	}{
		{base: "http://read-model.test", next: "http://read-model.test:80/final", host: "read-model.test:80"},
		{base: "http://read-model.test:80", next: "http://read-model.test/final", host: "read-model.test"},
		{base: "https://read-model.test", next: "https://read-model.test:443/final", host: "read-model.test:443"},
		{base: "https://read-model.test:443", next: "https://read-model.test/final", host: "read-model.test"},
	}
	for _, tc := range positive {
		t.Run("allow "+tc.base+" -> "+tc.next, func(t *testing.T) {
			step := 0
			rt := &safetyTransport{}
			rt.respond = func(req *http.Request) (*http.Response, error) {
				step++
				if step == 1 {
					return redirectResponse(req, http.StatusFound, tc.next, ""), nil
				}
				return redirectResponse(req, http.StatusNoContent, "", ""), nil
			}
			client := newSafetyClient(tc.base, rt, nil)
			_, depErr := client.FetchStatusSummary(context.Background(), ModeShadow, redirectTenant, redirectRequest)
			hops := rt.snapshot()
			if len(hops) != 2 {
				t.Fatalf("outbound requests = %d, want 2; err=%v", len(hops), depErr)
			}
			if hops[1].host != tc.host {
				t.Fatalf("second host = %q, want %q", hops[1].host, tc.host)
			}
			if depErr != nil && (errors.Is(depErr.Err, errWorkspaceRedirectRejected) || errors.Is(depErr.Err, errWorkspaceRedirectLimit)) {
				t.Fatalf("default port rejected: %v", depErr)
			}
		})
	}

	negative := []struct {
		base string
		next string
		host string
	}{
		{base: "http://read-model.test", next: "http://read-model.test:81/final", host: "read-model.test"},
		{base: "http://read-model.test:80", next: "http://read-model.test:8080/final", host: "read-model.test:80"},
		{base: "https://read-model.test", next: "https://read-model.test:444/final", host: "read-model.test"},
		{base: "http://read-model.test", next: "https://read-model.test/final", host: "read-model.test"},
		{base: "http://read-model.test", next: "https://read-model.test:443/final", host: "read-model.test"},
		{base: "http://read-model.test", next: "http://read-model.test:443/final", host: "read-model.test"},
	}
	for _, tc := range negative {
		t.Run("reject "+tc.base+" -> "+tc.next, func(t *testing.T) {
			rt := &safetyTransport{respond: redirectTo(tc.next, http.StatusFound)}
			client := newSafetyClient(tc.base, rt, nil)
			_, depErr := client.FetchStatusSummary(context.Background(), ModeShadow, redirectTenant, redirectRequest)
			if depErr == nil || !errors.Is(depErr.Err, errWorkspaceRedirectRejected) {
				t.Fatalf("expected reject, got %#v", depErr)
			}
			hops := rt.snapshot()
			if len(hops) != 1 || hops[0].host != tc.host {
				t.Fatalf("hops=%+v", hops)
			}
			if strings.Contains(depErr.Error(), tc.next) {
				t.Fatalf("error leaked target: %s", depErr.Error())
			}
		})
	}
}

func TestRedirectPolicyRejectsFragmentUserinfoAndOpaque(t *testing.T) {
	base := readModelBase
	fragment, err := url.Parse("http://read-model.test:8080/next#tenant")
	if err != nil {
		t.Fatal(err)
	}
	userinfo, err := url.Parse("http://user:pass@read-model.test:8080/next")
	if err != nil {
		t.Fatal(err)
	}
	opaque := &url.URL{Scheme: "http", Opaque: "not-a-path", Host: "read-model.test:8080"}
	for _, target := range []*url.URL{fragment, userinfo, opaque} {
		if workspaceRedirectAllowed(base, &http.Request{URL: target, Host: target.Host}) {
			t.Fatalf("allowed %s", target.String())
		}
	}
}
