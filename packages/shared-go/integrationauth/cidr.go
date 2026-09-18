package integrationauth

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// MatchAllowedCIDRs returns true when allowlist is empty/null (no restriction) or IP matches a CIDR.
// Malformed allowlist entries fail closed.
func MatchAllowedCIDRs(allowedJSON []byte, clientIP string) (bool, error) {
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return false, fmt.Errorf("invalid client ip")
	}
	if len(allowedJSON) == 0 || strings.TrimSpace(string(allowedJSON)) == "" || strings.TrimSpace(string(allowedJSON)) == "null" {
		return true, nil
	}
	var entries []string
	if err := json.Unmarshal(allowedJSON, &entries); err != nil {
		return false, fmt.Errorf("invalid allowed_cidrs")
	}
	if len(entries) == 0 {
		return true, nil
	}
	for _, raw := range entries {
		cidr := strings.TrimSpace(raw)
		if cidr == "" {
			continue
		}
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			plain := net.ParseIP(cidr)
			if plain == nil {
				return false, fmt.Errorf("invalid cidr entry")
			}
			if plain.Equal(ip) {
				return true, nil
			}
			continue
		}
		if network.Contains(ip) {
			return true, nil
		}
	}
	return false, nil
}
