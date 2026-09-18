package integrationauth

import (
	"fmt"
	"sort"
	"strings"
)

var allowedScopes = map[string]struct{}{
	"rfx:draft:create":  {},
	"rfx:draft:read":    {},
	"rfx:draft:preview": {},
	"rfx:draft:commit":  {},
	"rfx:status:read":   {},
}

func NormalizeScopes(scopes []string) ([]string, error) {
	seen := make(map[string]struct{}, len(scopes))
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := allowedScopes[scope]; !ok {
			return nil, fmt.Errorf("unsupported scope")
		}
		if _, dup := seen[scope]; dup {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	sort.Strings(out)
	return out, nil
}

func HasRequiredScopes(granted, required []string) bool {
	if len(required) == 0 {
		return true
	}
	grantSet := make(map[string]struct{}, len(granted))
	for _, scope := range granted {
		grantSet[strings.TrimSpace(scope)] = struct{}{}
	}
	for _, scope := range required {
		if _, ok := grantSet[strings.TrimSpace(scope)]; !ok {
			return false
		}
	}
	return true
}

func JoinScopes(scopes []string) string {
	normalized, err := NormalizeScopes(scopes)
	if err != nil || len(normalized) == 0 {
		return ""
	}
	return strings.Join(normalized, " ")
}
