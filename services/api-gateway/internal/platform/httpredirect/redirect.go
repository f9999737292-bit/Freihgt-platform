package httpredirect

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

const redirectLimit = 10

// ErrRejected means the next hop is not on the configured origin.
// The error text is static so a tenant, credential, or target URL cannot leak through it.
var ErrRejected = errors.New("downstream redirect rejected")

// ErrLimit means the hop count reached the same ceiling as net/http's default policy.
var ErrLimit = errors.New("downstream redirect limit exceeded")

// pinnedHeaders are restored from the server-owned request on an allowed hop
// and removed before a rejected hop returns. They are not read from via.
var pinnedHeaders = []string{
	"X-Tenant-ID",
	"X-User-ID",
	sharedmiddleware.RequestIDHeader,
	"Idempotency-Key",
	"Authorization",
	"Proxy-Authorization",
	"Cookie",
	"Cookie2",
	"X-Internal-Service-Token",
	"X-Company-ID",
	"X-Actor-Kind",
	"X-Driver-ID",
	"X-Vehicle-ID",
}

type trustedSnapshot struct {
	host   string
	values []string
}

// Do sends req with a shallow copy of client. The caller's Client, including a
// client shared by several downstreams, is not modified.
//
// Redirects are followed only when every hop stays on baseURL's origin.
// Trusted headers are copied from req before CheckRedirect runs. A later hop
// restores that snapshot and does not read via, which a caller hook can mutate.
func Do(client *http.Client, baseURL string, req *http.Request) (*http.Response, error) {
	if client == nil || req == nil {
		return nil, ErrRejected
	}
	snap := captureTrusted(req)
	clone := *client
	inherited := clone.CheckRedirect
	clone.CheckRedirect = redirectPolicy(baseURL, snap, inherited)
	resp, err := (&clone).Do(req)
	if err == nil {
		return resp, nil
	}
	if errors.Is(err, ErrLimit) {
		closeBody(resp)
		return nil, ErrLimit
	}
	if errors.Is(err, ErrRejected) || locationParseError(err) {
		closeBody(resp)
		return nil, ErrRejected
	}
	return resp, err
}

func redirectPolicy(baseURL string, snap trustedSnapshot, inherited func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= redirectLimit {
			scrubPinned(req)
			return ErrLimit
		}
		if !redirectAllowed(baseURL, req) {
			scrubPinned(req)
			return ErrRejected
		}
		if inherited != nil {
			if err := inherited(req, via); err != nil {
				scrubPinned(req)
				return ErrRejected
			}
		}
		if !redirectAllowed(baseURL, req) {
			scrubPinned(req)
			return ErrRejected
		}
		applyTrusted(req, snap)
		return nil
	}
}

func captureTrusted(req *http.Request) trustedSnapshot {
	snap := trustedSnapshot{values: make([]string, len(pinnedHeaders))}
	if req == nil {
		return snap
	}
	snap.host = strings.Clone(requestHost(req))
	for i, header := range pinnedHeaders {
		snap.values[i] = strings.Clone(req.Header.Get(header))
	}
	return snap
}

func requestHost(req *http.Request) string {
	if req == nil {
		return ""
	}
	if req.URL != nil && req.URL.Host != "" {
		return req.URL.Host
	}
	return req.Host
}

func applyTrusted(req *http.Request, snap trustedSnapshot) {
	if req == nil {
		return
	}
	for i, header := range pinnedHeaders {
		value := ""
		if i < len(snap.values) {
			value = snap.values[i]
		}
		if value == "" {
			req.Header.Del(header)
			continue
		}
		req.Header.Set(header, value)
	}
	if snap.host != "" {
		req.Host = snap.host
	}
}

func scrubPinned(req *http.Request) {
	if req == nil {
		return
	}
	for _, header := range pinnedHeaders {
		req.Header.Del(header)
	}
}

func redirectAllowed(baseURL string, req *http.Request) bool {
	if req == nil || req.URL == nil {
		return false
	}
	base, ok := configuredOrigin(baseURL)
	if !ok {
		return false
	}
	target := req.URL
	if target.User != nil || target.Fragment != "" || target.Opaque != "" {
		return false
	}
	if target.Hostname() == "" {
		return false
	}
	if !sameOrigin(base, target) {
		return false
	}
	return true
}

func configuredOrigin(raw string) (*url.URL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return nil, false
	}
	if parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" || parsed.Hostname() == "" {
		return nil, false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, false
	}
	return parsed, true
}

func sameOrigin(base, target *url.URL) bool {
	if base == nil || target == nil {
		return false
	}
	if !strings.EqualFold(base.Scheme, target.Scheme) || (strings.ToLower(target.Scheme) != "http" && strings.ToLower(target.Scheme) != "https") {
		return false
	}
	if !strings.EqualFold(base.Hostname(), target.Hostname()) {
		return false
	}
	basePort := canonicalPort(base)
	targetPort := canonicalPort(target)
	return basePort != "" && basePort == targetPort
}

func canonicalPort(u *url.URL) string {
	if u == nil {
		return ""
	}
	if port := u.Port(); port != "" {
		return port
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func locationParseError(err error) bool {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) || urlErr == nil || urlErr.Err == nil {
		return false
	}
	return strings.Contains(urlErr.Err.Error(), "failed to parse Location header")
}

func closeBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))
	_ = resp.Body.Close()
}
