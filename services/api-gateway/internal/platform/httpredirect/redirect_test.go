package httpredirect

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOriginRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		base   string
		target string
		allow  bool
	}{
		{name: "same host", base: "http://shipment.internal", target: "http://shipment.internal/v1", allow: true},
		{name: "http default port", base: "http://shipment.internal", target: "http://shipment.internal:80/v1", allow: true},
		{name: "http explicit default port", base: "http://shipment.internal:80", target: "http://shipment.internal/v1", allow: true},
		{name: "https default port", base: "https://shipment.internal", target: "https://shipment.internal:443/v1", allow: true},
		{name: "https explicit default port", base: "https://shipment.internal:443", target: "https://SHIPMENT.internal/v1", allow: true},
		{name: "host case", base: "http://Shipment.Internal", target: "http://shipment.internal/v1", allow: true},
		{name: "different port", base: "http://shipment.internal", target: "http://shipment.internal:81/v1", allow: false},
		{name: "scheme change", base: "http://shipment.internal", target: "https://shipment.internal/v1", allow: false},
		{name: "foreign host", base: "http://shipment.internal", target: "http://evil.example/v1", allow: false},
		{name: "subdomain", base: "http://shipment.internal", target: "http://api.shipment.internal/v1", allow: false},
		{name: "userinfo", base: "http://shipment.internal", target: "http://user:pass@shipment.internal/v1", allow: false},
		{name: "fragment", base: "http://shipment.internal", target: "http://shipment.internal/v1#frag", allow: false},
		{name: "base userinfo", base: "http://user:pass@shipment.internal", target: "http://shipment.internal/v1", allow: false},
		{name: "base fragment", base: "http://shipment.internal/v1#frag", target: "http://shipment.internal/v1", allow: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target, err := url.Parse(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			req := &http.Request{URL: target}
			if got := redirectAllowed(tc.base, req); got != tc.allow {
				t.Fatalf("redirectAllowed=%v want %v", got, tc.allow)
			}
		})
	}

	opaque := &http.Request{URL: &url.URL{Scheme: "http", Host: "shipment.internal", Opaque: "//shipment.internal/v1"}}
	if redirectAllowed("http://shipment.internal", opaque) {
		t.Fatal("opaque target was allowed")
	}
}

func TestProbeForeignRedirectIsNotSent(t *testing.T) {
	t.Parallel()
	var forbidden atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		http.Error(w, "forbidden", http.StatusOK)
	}))
	t.Cleanup(foreign.Close)

	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			forbidden.Store(0)
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, foreign.URL+"/steal", status)
			}))
			defer origin.Close()

			req, err := http.NewRequest(http.MethodPost, origin.URL+"/start", bytes.NewReader([]byte(`{"secret":true}`)))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("X-Tenant-ID", "tenant-a")
			req.Header.Set("X-User-ID", "user-a")
			req.Header.Set("X-Request-ID", "req-a")
			req.Header.Set("Idempotency-Key", "idem-a")
			req.Header.Set("Authorization", "Bearer secret-token")
			req.Header.Set("X-Internal-Service-Token", "internal-secret")

			caller := &http.Client{}
			_, err = Do(caller, origin.URL, req)
			if !errorsIsRejected(err) {
				t.Fatalf("error=%v want rejected", err)
			}
			if forbidden.Load() != 0 {
				t.Fatalf("forbidden requests=%d", forbidden.Load())
			}
			if caller.CheckRedirect != nil {
				t.Fatal("caller CheckRedirect was set")
			}
			assertNoSecret(t, err)
		})
	}
}

func TestProbeSameOriginRedirectKeepsTrustedHeaders(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var gotMethod, gotBody, gotTenant, gotUser, gotRequest, gotIdem, gotAuth, gotToken, gotHost string
			var hops atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/final" {
					hops.Add(1)
					gotMethod = r.Method
					raw, _ := io.ReadAll(r.Body)
					gotBody = string(raw)
					gotTenant = r.Header.Get("X-Tenant-ID")
					gotUser = r.Header.Get("X-User-ID")
					gotRequest = r.Header.Get("X-Request-ID")
					gotIdem = r.Header.Get("Idempotency-Key")
					gotAuth = r.Header.Get("Authorization")
					gotToken = r.Header.Get("X-Internal-Service-Token")
					gotHost = r.Host
					_, _ = w.Write([]byte("ok"))
					return
				}
				hops.Add(1)
				http.Redirect(w, r, originURLPath(r, "/final"), status)
			}))
			defer origin.Close()

			var hookCalls atomic.Int32
			caller := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
				hookCalls.Add(1)
				if len(via) > 0 && via[0] != nil {
					via[0].Header.Set("X-Tenant-ID", "attacker-tenant")
					via[0].Header.Set("X-User-ID", "attacker-user")
					via[0].Header.Set("X-Request-ID", "attacker-req")
					via[0].Header.Set("Idempotency-Key", "attacker-key")
					via[0].Header.Set("Authorization", "Bearer attacker")
					via[0].Header.Set("X-Internal-Service-Token", "attacker-token")
					via[0].Host = "attacker.internal"
				}
				req.Header.Set("X-Tenant-ID", "attacker-tenant")
				req.Header.Set("Authorization", "Bearer attacker")
				req.Header.Set("X-Internal-Service-Token", "attacker-token")
				req.Host = "attacker.internal"
				return nil
			}}

			req, err := http.NewRequest(http.MethodPost, origin.URL+"/start", bytes.NewReader([]byte(`{"keep":true}`)))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Tenant-ID", "tenant-a")
			req.Header.Set("X-User-ID", "user-a")
			req.Header.Set("X-Request-ID", "req-a")
			req.Header.Set("Idempotency-Key", "idem-a")
			req.Header.Set("Authorization", "Bearer secret-token")
			req.Header.Set("X-Internal-Service-Token", "internal-secret")

			resp, err := Do(caller, origin.URL, req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status=%d", resp.StatusCode)
			}
			if hookCalls.Load() == 0 {
				t.Fatal("caller CheckRedirect was not invoked")
			}
			if gotTenant != "tenant-a" || gotUser != "user-a" || gotRequest != "req-a" || gotIdem != "idem-a" {
				t.Fatalf("headers tenant=%q user=%q request=%q idem=%q", gotTenant, gotUser, gotRequest, gotIdem)
			}
			if gotAuth != "Bearer secret-token" || gotToken != "internal-secret" {
				t.Fatalf("credentials auth=%q token=%q", gotAuth, gotToken)
			}
			if gotHost == "attacker.internal" || gotHost == "" {
				t.Fatalf("host=%q", gotHost)
			}
			switch status {
			case http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
				if gotMethod != http.MethodPost || gotBody != `{"keep":true}` {
					t.Fatalf("method=%s body=%q want POST body preserved", gotMethod, gotBody)
				}
			default:
				if gotMethod != http.MethodGet {
					t.Fatalf("method=%s want GET", gotMethod)
				}
				if gotBody != "" {
					t.Fatalf("body=%q want empty after %d", gotBody, status)
				}
			}
			foreignReq, _ := http.NewRequest(http.MethodGet, "http://evil.example/x", nil)
			if err := caller.CheckRedirect(foreignReq, nil); err != nil {
				t.Fatalf("caller CheckRedirect was replaced: %v", err)
			}
		})
	}
}

func TestProbeSecondHopKeepsOriginalSnapshot(t *testing.T) {
	t.Parallel()
	var gotTenant, gotIdem, gotHost string
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/final":
			gotTenant = r.Header.Get("X-Tenant-ID")
			gotIdem = r.Header.Get("Idempotency-Key")
			gotHost = r.Host
			_, _ = w.Write([]byte("ok"))
		case "/mid":
			http.Redirect(w, r, originURLPath(r, "/final"), http.StatusFound)
		default:
			http.Redirect(w, r, originURLPath(r, "/mid"), http.StatusFound)
		}
	}))
	defer origin.Close()

	caller := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && via[0] != nil {
			via[0].Header.Set("X-Tenant-ID", "hop-mutated")
			via[0].Header.Set("Idempotency-Key", "hop-mutated")
			via[0].Host = "attacker.internal"
		}
		req.Header.Set("X-Tenant-ID", "hop-mutated")
		req.Header.Set("Idempotency-Key", "hop-mutated")
		req.Host = "attacker.internal"
		return nil
	}}
	req, err := http.NewRequest(http.MethodGet, origin.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tenant-ID", "tenant-a")
	req.Header.Set("Idempotency-Key", "idem-a")
	resp, err := Do(caller, origin.URL, req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if gotTenant != "tenant-a" || gotIdem != "idem-a" || gotHost == "attacker.internal" {
		t.Fatalf("tenant=%q idem=%q host=%q", gotTenant, gotIdem, gotHost)
	}
}

func TestProbeCallerRetargetIsRejected(t *testing.T) {
	t.Parallel()
	var forbidden atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forbidden.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer foreign.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, originURLPath(r, "/final"), http.StatusFound)
	}))
	defer origin.Close()

	foreignURL, err := url.Parse(foreign.URL + "/steal")
	if err != nil {
		t.Fatal(err)
	}
	caller := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		req.URL = foreignURL
		req.Host = foreignURL.Host
		return nil
	}}
	req, err := http.NewRequest(http.MethodGet, origin.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tenant-ID", "tenant-a")
	_, err = Do(caller, origin.URL, req)
	if !errorsIsRejected(err) {
		t.Fatalf("error=%v", err)
	}
	if forbidden.Load() != 0 {
		t.Fatalf("forbidden requests=%d", forbidden.Load())
	}
}

func TestProbeMalformedLocationDoesNotLeak(t *testing.T) {
	t.Parallel()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://tenant-secret-value%zz")
		w.WriteHeader(http.StatusFound)
	}))
	defer origin.Close()
	req, err := http.NewRequest(http.MethodGet, origin.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tenant-ID", "tenant-secret-value")
	_, err = Do(&http.Client{}, origin.URL, req)
	if !errorsIsRejected(err) {
		t.Fatalf("error=%v", err)
	}
	assertNoSecret(t, err)
}

func TestRedirectLimitMatchesDefaultClient(t *testing.T) {
	t.Parallel()
	var trips atomic.Int32
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		n := trips.Add(1)
		next := "http://shipment.internal/hop/" + strings.TrimPrefix(req.URL.Path, "/") + "-next"
		if n > redirectLimit+2 {
			t.Errorf("trip %d exceeded the limit window", n)
		}
		header := make(http.Header)
		header.Set("Location", next)
		return &http.Response{
			StatusCode: http.StatusFound,
			Status:     "302 Found",
			Header:     header,
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
		}, nil
	})
	req, err := http.NewRequest(http.MethodGet, "http://shipment.internal/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Do(&http.Client{Transport: transport}, "http://shipment.internal", req)
	if err != ErrLimit {
		t.Fatalf("error=%v want limit", err)
	}
	if trips.Load() != redirectLimit {
		t.Fatalf("trips=%d want %d", trips.Load(), redirectLimit)
	}
}

func TestDefaultPortRedirectUsesTransportWithoutDial(t *testing.T) {
	t.Parallel()
	var trips []string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		trips = append(trips, req.URL.String())
		header := make(http.Header)
		status := http.StatusOK
		body := "ok"
		switch req.URL.String() {
		case "http://shipment.internal/start":
			status = http.StatusFound
			header.Set("Location", "http://shipment.internal:80/final")
			body = ""
		case "http://shipment.internal:80/final":
		default:
			t.Errorf("unexpected request %s", req.URL)
		}
		return &http.Response{
			StatusCode:    status,
			Header:        header,
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
			Request:       req,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
		}, nil
	})
	req, err := http.NewRequest(http.MethodGet, "http://shipment.internal/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tenant-ID", "tenant-a")
	resp, err := Do(&http.Client{Transport: transport}, "http://shipment.internal", req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if len(trips) != 2 {
		t.Fatalf("trips=%v", trips)
	}

	trips = nil
	blocked := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		trips = append(trips, req.URL.String())
		if req.URL.Host == "shipment.internal:81" {
			t.Errorf("forbidden port was requested")
		}
		header := make(http.Header)
		header.Set("Location", "http://shipment.internal:81/final")
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     header,
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
		}, nil
	})
	req, err = http.NewRequest(http.MethodGet, "http://shipment.internal/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Do(&http.Client{Transport: blocked}, "http://shipment.internal", req)
	if err != ErrRejected {
		t.Fatalf("error=%v", err)
	}
	if len(trips) != 1 {
		t.Fatalf("trips=%v", trips)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func originURLPath(r *http.Request, path string) string {
	return "http://" + r.Host + path
}

func errorsIsRejected(err error) bool {
	return err == ErrRejected
}

func assertNoSecret(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	text := err.Error()
	for _, secret := range []string{"tenant-a", "tenant-secret-value", "secret-token", "internal-secret", "evil.example", "idem-a"} {
		if strings.Contains(text, secret) {
			t.Fatalf("error leaked %q: %s", secret, text)
		}
	}
}
