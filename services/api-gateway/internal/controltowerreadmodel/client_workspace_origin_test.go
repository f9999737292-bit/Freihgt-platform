package controltowerreadmodel

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	originTenantSecret = "secret-tenant"
	originUserSecret   = "secret-user"
	originRequestID    = "secret-request"
)

func TestWorkspaceSameOriginRequests(t *testing.T) {
	var gotMethod, gotPath, gotQuery, gotTenant, gotUser, gotRequest string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotRequest = r.Header.Get("X-Request-ID")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == listViewsPath:
			_, _ = io.WriteString(w, `{"id":"view-1","name":"ops","scope":"USER","filterSchemaVersion":1,"filters":{},"sort":{},"isDefault":false,"workspaceScope":"OPS","createdAt":"t","updatedAt":"t"}`)
		default:
			_, _ = io.WriteString(w, `{"items":[],"page":1,"limit":1,"total":0,"hasNext":false}`)
		}
	}))
	defer server.Close()

	caller := &http.Client{Timeout: time.Second}
	client := NewClient(caller, Config{BaseURL: server.URL, Timeout: time.Second, MaxResponseBytes: 1 << 20}, NewMetrics())
	base, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	page, depErr := client.ListWorkItems(context.Background(), originTenantSecret, originUserSecret, originRequestID, WorkItemsFilter{Preset: "ops"})
	if depErr != nil {
		t.Fatalf("relative query through list: %v", depErr)
	}
	if page == nil || page.Total != 0 {
		t.Fatalf("list payload = %#v", page)
	}
	if gotMethod != http.MethodGet || gotPath != listWorkItemsPath || !strings.Contains(gotQuery, "preset=ops") {
		t.Fatalf("list request method=%s path=%s query=%s", gotMethod, gotPath, gotQuery)
	}
	assertOriginHeaders(t, gotTenant, gotUser, gotRequest)

	absolute := base.Scheme + "://" + base.Host + workloadPath
	var payload RemoteWorkloadResponse
	if depErr = client.doWorkspaceJSON(context.Background(), http.MethodGet, absolute, originTenantSecret, originUserSecret, originRequestID, nil, &payload); depErr != nil {
		t.Fatalf("same-origin absolute url: %v", depErr)
	}
	if gotMethod != http.MethodGet || gotPath != workloadPath || base.Host != hostOf(t, server.URL) {
		t.Fatalf("absolute request method=%s path=%s host=%s", gotMethod, gotPath, base.Host)
	}
	assertOriginHeaders(t, gotTenant, gotUser, gotRequest)

	if depErr = client.doWorkspaceJSON(context.Background(), http.MethodGet, "/internal/v1/control-tower/handoffs?mine=1", originTenantSecret, originUserSecret, originRequestID, nil, &payload); depErr != nil {
		t.Fatalf("relative endpoint: %v", depErr)
	}
	if gotPath != listHandoffsPath || gotQuery != "mine=1" {
		t.Fatalf("relative path=%s query=%s", gotPath, gotQuery)
	}

	if depErr = client.doWorkspaceJSON(context.Background(), http.MethodGet, "", originTenantSecret, originUserSecret, originRequestID, nil, &payload); depErr != nil {
		t.Fatalf("empty endpoint: %v", depErr)
	}
	if gotPath != "/" {
		t.Fatalf("empty endpoint path=%s", gotPath)
	}

	view, depErr := client.CreateView(context.Background(), CreateViewInput{
		TenantID: originTenantSecret, UserID: originUserSecret, RequestID: originRequestID, Name: "ops", Scope: "USER",
	})
	if depErr != nil {
		t.Fatalf("workspace post: %v", depErr)
	}
	if view == nil || view.ID != "view-1" || gotMethod != http.MethodPost || gotPath != listViewsPath {
		t.Fatalf("post method=%s path=%s view=%#v", gotMethod, gotPath, view)
	}
	assertOriginHeaders(t, gotTenant, gotUser, gotRequest)
	if caller.CheckRedirect != nil || http.DefaultClient.CheckRedirect != nil {
		t.Fatal("workspace request changed a shared redirect policy")
	}
}

func TestWorkspaceDeniedEndpointsDoNotDial(t *testing.T) {
	var trustedHits atomic.Int32
	trusted := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		trustedHits.Add(1)
	}))
	defer trusted.Close()
	var foreignHits atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		foreignHits.Add(1)
	}))
	defer foreign.Close()

	base, err := url.Parse(trusted.URL)
	if err != nil {
		t.Fatalf("parse trusted url: %v", err)
	}
	foreignURL, err := url.Parse(foreign.URL)
	if err != nil {
		t.Fatalf("parse foreign url: %v", err)
	}
	httpsSameSocket := "https://" + base.Host + workloadPath
	userinfo := "http://user:pass@" + base.Host + workloadPath + "?user=" + originUserSecret + "&tenant=" + originTenantSecret
	similar := "http://" + base.Hostname() + ".evil:" + base.Port() + workloadPath + "?user=" + originUserSecret

	probe := &originProbeTransport{}
	client := NewClient(&http.Client{Transport: probe, Timeout: time.Second}, Config{
		BaseURL: trusted.URL, Timeout: time.Second, MaxResponseBytes: 1024,
	}, NewMetrics())

	cases := []string{
		"http://evil.example" + workloadPath + "?user=" + originUserSecret + "&tenant=" + originTenantSecret,
		foreign.URL + workloadPath,
		httpsSameSocket,
		userinfo,
		"http://[",
		similar,
		"//evil.example" + workloadPath,
		"///evil.example" + workloadPath,
		"ftp://" + base.Host + workloadPath,
	}
	for _, endpoint := range cases {
		before := probe.calls.Load()
		var payload RemoteWorkloadResponse
		depErr := client.doWorkspaceJSON(context.Background(), http.MethodGet, endpoint, originTenantSecret, originUserSecret, originRequestID, nil, &payload)
		if depErr == nil {
			t.Fatalf("endpoint %q was accepted", endpoint)
		}
		if !errors.Is(depErr.Err, errWorkspaceEndpointRejected) {
			t.Fatalf("endpoint %q error = %v", endpoint, depErr)
		}
		if strings.Contains(depErr.Error(), originTenantSecret) || strings.Contains(depErr.Error(), originUserSecret) || strings.Contains(depErr.Error(), originRequestID) {
			t.Fatalf("endpoint %q leaked sensitive values in %q", endpoint, depErr.Error())
		}
		if probe.calls.Load() != before {
			t.Fatalf("endpoint %q dialed", endpoint)
		}
		if foreignURL.Hostname() == "" {
			t.Fatal("foreign server has no host")
		}
	}
	if trustedHits.Load() != 0 || foreignHits.Load() != 0 {
		t.Fatalf("denied endpoints reached trusted=%d foreign=%d", trustedHits.Load(), foreignHits.Load())
	}
}

func TestWorkspaceCrossOriginRedirectIsNotFollowed(t *testing.T) {
	var foreignHits atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		foreignHits.Add(1)
	}))
	defer foreign.Close()

	var trustedHits atomic.Int32
	trusted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trustedHits.Add(1)
		if r.Header.Get("X-Tenant-ID") != originTenantSecret {
			t.Errorf("trusted hop missing tenant header")
		}
		http.Redirect(w, r, foreign.URL+"/collect?user="+originUserSecret+"&tenant="+originTenantSecret, http.StatusFound)
	}))
	defer trusted.Close()

	caller := &http.Client{Timeout: time.Second}
	client := NewClient(caller, Config{BaseURL: trusted.URL, Timeout: time.Second, MaxResponseBytes: 1024}, NewMetrics())
	_, depErr := client.GetWorkload(context.Background(), originTenantSecret, originUserSecret, originRequestID)
	if depErr == nil {
		t.Fatal("cross-origin redirect was accepted")
	}
	if !errors.Is(depErr.Err, errWorkspaceRedirectRejected) {
		t.Fatalf("redirect error = %v", depErr)
	}
	if strings.Contains(depErr.Error(), originTenantSecret) || strings.Contains(depErr.Error(), originUserSecret) || strings.Contains(depErr.Error(), foreign.URL) {
		t.Fatalf("redirect error leaked request data: %s", depErr.Error())
	}
	if trustedHits.Load() != 1 || foreignHits.Load() != 0 {
		t.Fatalf("redirect hops trusted=%d foreign=%d", trustedHits.Load(), foreignHits.Load())
	}
	if caller.CheckRedirect != nil {
		t.Fatal("redirect guard replaced the caller client policy")
	}
}

func TestWorkspaceSameOriginRedirectKeepsHeaders(t *testing.T) {
	const finalPath = "/internal/v1/control-tower/workload-final"
	var hops atomic.Int32
	var gotTenant, gotUser, gotRequest string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops.Add(1)
		if r.URL.Path == workloadPath {
			http.Redirect(w, r, finalPath, http.StatusFound)
			return
		}
		gotTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		gotRequest = r.Header.Get("X-Request-ID")
		_, _ = io.WriteString(w, `{"operators":[],"unassignedPool":0,"kpi":{}}`)
	}))
	defer server.Close()

	client := NewClient(&http.Client{Timeout: time.Second}, Config{BaseURL: server.URL, Timeout: time.Second, MaxResponseBytes: 1024}, NewMetrics())
	payload, depErr := client.GetWorkload(context.Background(), originTenantSecret, originUserSecret, originRequestID)
	if depErr != nil {
		t.Fatalf("same-origin redirect: %v", depErr)
	}
	if payload == nil || hops.Load() != 2 {
		t.Fatalf("hops=%d payload=%#v", hops.Load(), payload)
	}
	assertOriginHeaders(t, gotTenant, gotUser, gotRequest)
}

func TestWorkspaceCallerRedirectPolicyStaysInPlace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"operators":[],"unassignedPool":0,"kpi":{}}`)
	}))
	defer server.Close()

	var seen atomic.Bool
	caller := &http.Client{
		Timeout: time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			seen.Store(true)
			return errors.New("caller policy")
		},
	}
	client := NewClient(caller, Config{BaseURL: server.URL, Timeout: time.Second, MaxResponseBytes: 1024}, NewMetrics())
	if _, depErr := client.GetWorkload(context.Background(), originTenantSecret, originUserSecret, originRequestID); depErr != nil {
		t.Fatalf("workload: %v", depErr)
	}
	if seen.Load() {
		t.Fatal("caller redirect policy ran for a request without a redirect")
	}
	if err := caller.CheckRedirect(nil, nil); err == nil || !seen.Load() {
		t.Fatal("caller redirect policy was replaced")
	}
}

type originProbeTransport struct {
	calls atomic.Int32
}

func (p *originProbeTransport) RoundTrip(*http.Request) (*http.Response, error) {
	p.calls.Add(1)
	return nil, errors.New("unexpected outbound request")
}

func assertOriginHeaders(t *testing.T, tenant, user, requestID string) {
	t.Helper()
	if tenant != originTenantSecret || user != originUserSecret || requestID != originRequestID {
		t.Fatalf("headers tenant=%q user=%q request=%q", tenant, user, requestID)
	}
}

func hostOf(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse host: %v", err)
	}
	return parsed.Host
}
