package controltowerreadmodel

import (
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
)

type redirectHop struct {
	url    string
	host   string
	auth   string
	tenant string
	user   string
}

type redirectTransport struct {
	mu       sync.Mutex
	hops     []redirectHop
	respond  func(*http.Request) (*http.Response, error)
	injected bool
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.hops = append(t.hops, redirectHop{
		url: req.URL.String(), host: req.URL.Host,
		auth: req.Header.Get("Authorization"), tenant: req.Header.Get("X-Tenant-ID"), user: req.Header.Get("X-User-ID"),
	})
	t.mu.Unlock()
	return t.respond(req)
}

func (t *redirectTransport) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.hops)
}

func (t *redirectTransport) last() redirectHop {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.hops[len(t.hops)-1]
}

func redirectResponse(req *http.Request, status int, location, body string) *http.Response {
	header := make(http.Header)
	if location != "" {
		header.Set("Location", location)
	}
	header.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
}

func workloadOK(req *http.Request) *http.Response {
	return redirectResponse(req, http.StatusOK, "", `{"operators":[],"unassignedPool":0,"kpi":{}}`)
}

func TestWorkspaceRedirectAllowsSameOriginHop(t *testing.T) {
	const finalPath = "/internal/v1/control-tower/workload-final"
	transport := &redirectTransport{}
	transport.respond = func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == workloadPath {
			return redirectResponse(req, http.StatusFound, finalPath, ""), nil
		}
		return workloadOK(req), nil
	}
	base := "http://read-model.test:8080"
	client := NewClient(&http.Client{Transport: transport, Timeout: time.Second}, Config{
		BaseURL: base, Timeout: time.Second, MaxResponseBytes: 1024,
	}, NewMetrics())
	payload, depErr := client.GetWorkload(context.Background(), originTenantSecret, originUserSecret, originRequestID)
	if depErr != nil {
		t.Fatalf("same-origin redirect: %v", depErr)
	}
	if payload == nil || transport.count() != 2 {
		t.Fatalf("hops=%d payload=%#v", transport.count(), payload)
	}
	got := transport.last()
	if got.host != "read-model.test:8080" || got.tenant != originTenantSecret || got.user != originUserSecret || got.auth != "" {
		t.Fatalf("final hop %+v", got)
	}
}

func TestWorkspaceRedirectRejectsForeignHostPortAndInternalTargets(t *testing.T) {
	base := "http://read-model.test:8080"
	targets := []string{
		"http://evil.example/collect?tenant=" + originTenantSecret,
		"http://read-model.test:9090/collect",
		"http://127.0.0.1/latest/meta-data",
		"http://127.0.0.1:8080/collect",
		"http://[::1]/collect",
		"http://169.254.169.254/latest/meta-data",
		"https://read-model.test:8080/collect",
		"http://user:pass@read-model.test:8080/collect?tenant=" + originTenantSecret,
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			transport := &redirectTransport{}
			transport.respond = func(req *http.Request) (*http.Response, error) {
				if !transport.injected {
					req.Header.Set("Authorization", "Bearer "+originUserSecret)
					transport.injected = true
				}
				return redirectResponse(req, http.StatusFound, target, ""), nil
			}
			client := NewClient(&http.Client{Transport: transport, Timeout: time.Second}, Config{
				BaseURL: base, Timeout: time.Second, MaxResponseBytes: 1024,
			}, NewMetrics())
			_, depErr := client.GetWorkload(context.Background(), originTenantSecret, originUserSecret, originRequestID)
			if depErr == nil || !errors.Is(depErr.Err, errWorkspaceRedirectRejected) {
				t.Fatalf("error = %v", depErr)
			}
			if strings.Contains(depErr.Error(), originTenantSecret) || strings.Contains(depErr.Error(), originUserSecret) || strings.Contains(depErr.Error(), target) {
				t.Fatalf("error leaked redirect data: %s", depErr.Error())
			}
			if transport.count() != 1 {
				t.Fatalf("downstream requests = %d, want 1", transport.count())
			}
			if transport.last().host != "read-model.test:8080" {
				t.Fatalf("only hop host = %s", transport.last().host)
			}
		})
	}
}

func TestWorkspaceRedirectStripsAuthorizationOnAllowedHop(t *testing.T) {
	const finalPath = "/internal/v1/control-tower/workload-final"
	caller := &http.Client{
		Timeout: time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			req.Header.Set("Authorization", "Bearer "+originUserSecret)
			req.Header.Set("Cookie", "session="+originTenantSecret)
			return nil
		},
	}
	var gotAuth, gotCookie, gotTenant string
	var hops int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops++
		if r.URL.Path == workloadPath {
			http.Redirect(w, r, finalPath, http.StatusFound)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		gotTenant = r.Header.Get("X-Tenant-ID")
		_, _ = io.WriteString(w, `{"operators":[],"unassignedPool":0,"kpi":{}}`)
	}))
	defer server.Close()
	client := NewClient(caller, Config{BaseURL: server.URL, Timeout: time.Second, MaxResponseBytes: 1024}, NewMetrics())
	if _, depErr := client.GetWorkload(context.Background(), originTenantSecret, originUserSecret, originRequestID); depErr != nil {
		t.Fatalf("allowed redirect: %v", depErr)
	}
	if hops != 2 || gotAuth != "" || gotCookie != "" || gotTenant != originTenantSecret {
		t.Fatalf("hops=%d auth=%q cookie=%q tenant=%q", hops, gotAuth, gotCookie, gotTenant)
	}
	if caller.CheckRedirect == nil {
		t.Fatal("caller redirect policy was replaced")
	}
}

func TestWorkspaceCallerCannotRetargetRedirect(t *testing.T) {
	var stolen atomicHits
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stolen.add()
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Tenant-ID") != "" {
			t.Errorf("forbidden hop received auth=%q tenant=%q", r.Header.Get("Authorization"), r.Header.Get("X-Tenant-ID"))
		}
	}))
	defer foreign.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer origin.Close()

	caller := &http.Client{
		Timeout: time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			stolenURL, _ := url.Parse(foreign.URL + "/steal?tenant=" + originTenantSecret)
			req.URL = stolenURL
			req.Host = stolenURL.Host
			req.Header.Set("Authorization", "Bearer "+originUserSecret)
			req.Header.Set("X-Tenant-ID", originTenantSecret)
			return nil
		},
	}
	client := NewClient(caller, Config{BaseURL: origin.URL, Timeout: time.Second, MaxResponseBytes: 1024}, NewMetrics())
	_, depErr := client.GetWorkload(context.Background(), originTenantSecret, originUserSecret, originRequestID)
	if depErr == nil || !errors.Is(depErr.Err, errWorkspaceRedirectRejected) {
		t.Fatalf("error = %v", depErr)
	}
	if strings.Contains(depErr.Error(), originTenantSecret) || strings.Contains(depErr.Error(), foreign.URL) {
		t.Fatalf("error leaked redirect data: %s", depErr.Error())
	}
	if stolen.n() != 0 {
		t.Fatalf("retargeted redirect was sent %d times", stolen.n())
	}
}

func TestWorkspaceMultiHopStopsAtForeignTarget(t *testing.T) {
	base := "http://read-model.test:8080"
	transport := &redirectTransport{}
	transport.respond = func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case workloadPath:
			return redirectResponse(req, http.StatusFound, "/hop-2", ""), nil
		case "/hop-2":
			return redirectResponse(req, http.StatusFound, "http://169.254.169.254/latest", ""), nil
		default:
			t.Errorf("unexpected hop %s", req.URL.String())
			return workloadOK(req), nil
		}
	}
	client := NewClient(&http.Client{Transport: transport, Timeout: time.Second}, Config{
		BaseURL: base, Timeout: time.Second, MaxResponseBytes: 1024,
	}, NewMetrics())
	_, depErr := client.GetWorkload(context.Background(), originTenantSecret, originUserSecret, originRequestID)
	if depErr == nil || !errors.Is(depErr.Err, errWorkspaceRedirectRejected) {
		t.Fatalf("error = %v", depErr)
	}
	if transport.count() != 2 {
		t.Fatalf("hops=%d", transport.count())
	}
	if transport.last().host != "read-model.test:8080" || transport.last().tenant != originTenantSecret {
		t.Fatalf("last allowed hop %+v", transport.last())
	}
}

func TestWorkspaceRedirectLoopStopsAtLimit(t *testing.T) {
	base := "http://read-model.test:8080"
	transport := &redirectTransport{}
	transport.respond = func(req *http.Request) (*http.Response, error) {
		return redirectResponse(req, http.StatusFound, workloadPath+"?tenant="+originTenantSecret, ""), nil
	}
	caller := &http.Client{
		Transport: transport,
		Timeout:   time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return nil
		},
	}
	client := NewClient(caller, Config{BaseURL: base, Timeout: time.Second, MaxResponseBytes: 1024}, NewMetrics())
	_, depErr := client.GetWorkload(context.Background(), originTenantSecret, originUserSecret, originRequestID)
	if depErr == nil || !errors.Is(depErr.Err, errWorkspaceRedirectLimit) {
		t.Fatalf("error = %v", depErr)
	}
	if strings.Contains(depErr.Error(), originTenantSecret) || strings.Contains(depErr.Error(), base) {
		t.Fatalf("limit error leaked request data: %s", depErr.Error())
	}
	if transport.count() != workspaceRedirectLimit {
		t.Fatalf("hops=%d want %d", transport.count(), workspaceRedirectLimit)
	}
}

func TestWorkspaceRedirectPolicyDoesNotResolveRejectedTarget(t *testing.T) {
	base := "http://read-model.test:8080"
	next, err := http.NewRequest(http.MethodGet, base+"/rebound", nil)
	if err != nil {
		t.Fatal(err)
	}
	next.Header.Set("Authorization", "Bearer "+originUserSecret)
	next.Header.Set("X-Tenant-ID", originTenantSecret)
	policy := workspaceRedirectPolicy(base, func(req *http.Request, _ []*http.Request) error {
		req.URL, _ = url.Parse("http://10.0.0.8/rebound")
		return nil
	})
	if err := policy(next, nil); !errors.Is(err, errWorkspaceRedirectRejected) {
		t.Fatalf("policy error = %v", err)
	}
	if next.Header.Get("Authorization") != "" || next.Header.Get("X-Tenant-ID") != "" {
		t.Fatal("rejected redirect kept sensitive headers")
	}
	if next.URL.Hostname() != "10.0.0.8" {
		t.Fatalf("rejected target host = %s", next.URL.Hostname())
	}

	blocked, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/rebound", nil)
	if err != nil {
		t.Fatal(err)
	}
	blocked.Header.Set("X-Tenant-ID", originTenantSecret)
	called := false
	rejectFirst := workspaceRedirectPolicy(base, func(*http.Request, []*http.Request) error {
		called = true
		return nil
	})
	if err := rejectFirst(blocked, nil); !errors.Is(err, errWorkspaceRedirectRejected) {
		t.Fatalf("first-hop error = %v", err)
	}
	if called {
		t.Fatal("caller policy ran after the target was already rejected")
	}
	if blocked.Header.Get("X-Tenant-ID") != "" || blocked.URL.Hostname() != "127.0.0.1" {
		t.Fatalf("blocked request header=%q host=%s", blocked.Header.Get("X-Tenant-ID"), blocked.URL.Hostname())
	}
}

type atomicHits struct {
	mu sync.Mutex
	v  int
}

func (a *atomicHits) add() {
	a.mu.Lock()
	a.v++
	a.mu.Unlock()
}

func (a *atomicHits) n() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.v
}
