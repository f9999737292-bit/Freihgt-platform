package clientip

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// Resolver resolves the client IP from an HTTP request using trusted-proxy rules.
//
// Precedence when the immediate peer is trusted:
//  1. X-Forwarded-For (right-to-left trusted-hop walk)
//  2. X-Real-IP
//  3. RemoteAddr
//
// RFC Forwarded is intentionally not parsed.
type Resolver struct {
	trusted []*net.IPNet
}

func NewResolver(trusted []*net.IPNet) *Resolver {
	return &Resolver{trusted: trusted}
}

// ParseTrustedProxyCIDRs parses comma-separated CIDR entries.
// Empty input means no trusted proxies.
func ParseTrustedProxyCIDRs(raw string) ([]*net.IPNet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	out := make([]*net.IPNet, 0)
	for _, part := range strings.Split(raw, ",") {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, fmt.Errorf("invalid trusted proxy entry %q", entry)
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			entry = fmt.Sprintf("%s/%d", ip.String(), bits)
		}
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", entry, err)
		}
		out = append(out, network)
	}
	return out, nil
}

// ClientIP returns the resolved client IP or an error for malformed/ambiguous chains.
func (r *Resolver) ClientIP(req *http.Request) (string, error) {
	peerIP, err := hostFromAddr(req.RemoteAddr)
	if err != nil {
		return "", err
	}
	peer := net.ParseIP(peerIP)
	if peer == nil {
		return "", fmt.Errorf("invalid remote addr")
	}
	if len(r.trusted) == 0 || !r.contains(peer) {
		return normalizeIP(peer), nil
	}
	if forwarded := strings.TrimSpace(req.Header.Get("X-Forwarded-For")); forwarded != "" {
		ip, err := clientFromForwardedFor(forwarded, r.trusted)
		if err != nil {
			return "", err
		}
		return ip, nil
	}
	if realIP := strings.TrimSpace(req.Header.Get("X-Real-IP")); realIP != "" {
		ip := net.ParseIP(realIP)
		if ip == nil {
			return "", fmt.Errorf("invalid x-real-ip")
		}
		return normalizeIP(ip), nil
	}
	return normalizeIP(peer), nil
}

func clientFromForwardedFor(raw string, trusted []*net.IPNet) (string, error) {
	parts := make([]net.IP, 0)
	for _, segment := range strings.Split(raw, ",") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		ip := net.ParseIP(segment)
		if ip == nil {
			return "", fmt.Errorf("invalid x-forwarded-for entry")
		}
		parts = append(parts, ip)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("empty x-forwarded-for chain")
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if !containsIP(trusted, parts[i]) {
			return normalizeIP(parts[i]), nil
		}
	}
	return "", fmt.Errorf("ambiguous x-forwarded-for chain")
}

func (r *Resolver) contains(ip net.IP) bool {
	return containsIP(r.trusted, ip)
}

func containsIP(networks []*net.IPNet, ip net.IP) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func hostFromAddr(remoteAddr string) (string, error) {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err == nil {
		return host, nil
	}
	if strings.Contains(remoteAddr, ":") {
		return "", fmt.Errorf("invalid remote addr")
	}
	return strings.TrimSpace(remoteAddr), nil
}

func normalizeIP(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.String()
}
