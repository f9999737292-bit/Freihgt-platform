package httpredirect

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

const (
	allowedBase = "http://shipment.internal:8080"
	tenantValue = "tenant-secret"
	bodyMarker  = "post-body-secret"
)

type hopRecord struct {
	host       string
	hostHeader string
	method     string
	tenant     string
	body       string
}

type recordingTransport struct {
	mu    sync.Mutex
	hops  []hopRecord
	reply func(call int, req *http.Request) *http.Response
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(strings.NewReader(string(body)))
	}
	t.mu.Lock()
	t.hops = append(t.hops, hopRecord{
		host: req.URL.Host, hostHeader: req.Host, method: req.Method,
		tenant: req.Header.Get("X-Tenant-ID"), body: string(body),
	})
	n := len(t.hops)
	t.mu.Unlock()
	return t.reply(n, req), nil
}

func (t *recordingTransport) snapshot() []hopRecord {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]hopRecord, len(t.hops))
	copy(out, t.hops)
	return out
}

func response(req *http.Request, status int, location string) *http.Response {
	header := make(http.Header)
	if location != "" {
		header.Set("Location", location)
	}
	return &http.Response{
		StatusCode: status, Status: http.StatusText(status), Header: header,
		Body: io.NopCloser(strings.NewReader("")), Request: req,
		Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
	}
}

func newRequest(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, allowedBase+"/start", strings.NewReader(bodyMarker))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tenant-ID", tenantValue)
	req.Header.Set("X-User-ID", "user-secret")
	req.Header.Set("X-Request-ID", "request-secret")
	req.Header.Set("Idempotency-Key", "idem-secret")
	req.Header.Set("Authorization", "Bearer service-secret")
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(bodyMarker)), nil
	}
	return req
}

func assertNoForeign(t *testing.T, hops []hopRecord) {
	t.Helper()
	forbidden := 0
	for _, hop := range hops {
		if hop.host != "shipment.internal:8080" {
			forbidden++
			t.Errorf("outbound request to %s", hop.host)
		}
	}
	if forbidden != 0 {
		t.Fatalf("forbidden target request count = %d", forbidden)
	}
}

func TestRedirectRejectsForbiddenTargets(t *testing.T) {
	targets := []string{
		"http://evil.example/collect",
		"http://169.254.169.254/latest/meta-data",
		"http://shipment.internal:9090/collect",
		"https://shipment.internal:8080/collect",
		"http://user:pass@shipment.internal:8080/collect",
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			transport := &recordingTransport{reply: func(_ int, req *http.Request) *http.Response {
				return response(req, http.StatusFound, target)
			}}
			_, err := Do(&http.Client{Transport: transport}, allowedBase, newRequest(t))
			if err != ErrRejected {
				t.Fatalf("err = %v", err)
			}
			hops := transport.snapshot()
			if len(hops) != 1 {
				t.Fatalf("outbound requests = %d", len(hops))
			}
			assertNoForeign(t, hops)
			if strings.Contains(err.Error(), tenantValue) || strings.Contains(err.Error(), target) {
				t.Fatalf("error leaked: %s", err)
			}
		})
	}
}

func TestPostRedirectDoesNotSendBodyOffOrigin(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			transport := &recordingTransport{reply: func(_ int, req *http.Request) *http.Response {
				return response(req, status, "http://169.254.169.254/collect")
			}}
			_, err := Do(&http.Client{Transport: transport}, allowedBase, newRequest(t))
			if err != ErrRejected {
				t.Fatalf("err = %v", err)
			}
			hops := transport.snapshot()
			if len(hops) != 1 || !strings.Contains(hops[0].body, bodyMarker) {
				t.Fatalf("hops = %+v", hops)
			}
			assertNoForeign(t, hops)
		})
	}
}

func TestMultiHopAndLoop(t *testing.T) {
	t.Run("multi-hop", func(t *testing.T) {
		transport := &recordingTransport{reply: func(n int, req *http.Request) *http.Response {
			if n == 1 {
				return response(req, http.StatusFound, "/hop-2")
			}
			return response(req, http.StatusFound, "http://169.254.169.254/latest/meta-data")
		}}
		_, err := Do(&http.Client{Transport: transport}, allowedBase, newRequest(t))
		if err != ErrRejected {
			t.Fatalf("err = %v", err)
		}
		hops := transport.snapshot()
		if len(hops) != 2 {
			t.Fatalf("outbound requests = %d", len(hops))
		}
		assertNoForeign(t, hops)
	})
	t.Run("loop", func(t *testing.T) {
		transport := &recordingTransport{reply: func(_ int, req *http.Request) *http.Response {
			return response(req, http.StatusFound, "/start")
		}}
		_, err := Do(&http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return nil }}, allowedBase, newRequest(t))
		if err != ErrLimit {
			t.Fatalf("err = %v", err)
		}
		hops := transport.snapshot()
		if len(hops) != redirectLimit {
			t.Fatalf("outbound requests = %d, want %d", len(hops), redirectLimit)
		}
		assertNoForeign(t, hops)
	})
}

func TestSnapshotSurvivesMutatedVia(t *testing.T) {
	transport := &recordingTransport{reply: func(n int, req *http.Request) *http.Response {
		if n < 3 {
			return response(req, http.StatusFound, "/hop")
		}
		return response(req, http.StatusNoContent, "")
	}}
	caller := &http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && via[0] != nil {
			via[0].Header.Set("X-Tenant-ID", "mutated-tenant")
			via[0].Header.Set("Idempotency-Key", "mutated-key")
			via[0].Header.Set("Authorization", "Bearer mutated")
			via[0].Host = "evil.example"
		}
		req.Header.Set("X-Tenant-ID", "hop-tenant")
		req.Header.Set("Authorization", "Bearer hop")
		req.Host = "evil.example"
		return nil
	}}
	original := caller.CheckRedirect
	_, err := Do(caller, allowedBase, newRequest(t))
	if err != nil {
		t.Fatalf("same-origin hops: %v", err)
	}
	if caller.CheckRedirect == nil {
		t.Fatal("caller policy removed")
	}
	_ = original
	hops := transport.snapshot()
	if len(hops) != 3 {
		t.Fatalf("outbound requests = %d", len(hops))
	}
	for _, hop := range hops[1:] {
		if hop.tenant != tenantValue || hop.hostHeader != "shipment.internal:8080" {
			t.Fatalf("snapshot lost: %+v", hop)
		}
	}
}

func TestSameOrigin307KeepsBody(t *testing.T) {
	transport := &recordingTransport{reply: func(n int, req *http.Request) *http.Response {
		if n == 1 {
			return response(req, http.StatusTemporaryRedirect, "/final")
		}
		return response(req, http.StatusNoContent, "")
	}}
	_, err := Do(&http.Client{Transport: transport}, allowedBase, newRequest(t))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	hops := transport.snapshot()
	if len(hops) != 2 || hops[1].method != http.MethodPost || !strings.Contains(hops[1].body, bodyMarker) {
		t.Fatalf("hops = %+v", hops)
	}
	if hops[1].tenant != tenantValue {
		t.Fatalf("tenant = %q", hops[1].tenant)
	}
}

func TestDefaultPorts(t *testing.T) {
	transport := &recordingTransport{reply: func(n int, req *http.Request) *http.Response {
		if n == 1 {
			return response(req, http.StatusFound, "http://shipment.internal/final")
		}
		return response(req, http.StatusNoContent, "")
	}}
	req, err := http.NewRequest(http.MethodGet, "http://shipment.internal:80/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Do(&http.Client{Transport: transport}, "http://shipment.internal:80", req); err != nil {
		t.Fatal(err)
	}
	if len(transport.snapshot()) != 2 {
		t.Fatalf("default port was rejected: %+v", transport.snapshot())
	}

	blocked := &recordingTransport{reply: func(_ int, req *http.Request) *http.Response {
		return response(req, http.StatusFound, "http://shipment.internal:81/final")
	}}
	req, _ = http.NewRequest(http.MethodGet, "http://shipment.internal/start", nil)
	if _, err := Do(&http.Client{Transport: blocked}, "http://shipment.internal", req); err != ErrRejected {
		t.Fatalf("different port err = %v", err)
	}
	if len(blocked.snapshot()) != 1 {
		t.Fatalf("hops = %+v", blocked.snapshot())
	}
}
