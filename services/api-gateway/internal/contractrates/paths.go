package contractrates

import (
	"strings"
)

func mapPublicToInternalPath(publicPath string) (string, bool) {
	if !strings.HasPrefix(publicPath, "/api/v1/") {
		return "", false
	}
	suffix := strings.TrimPrefix(publicPath, "/api/v1/")
	if suffix == "" {
		return "", false
	}
	return "/internal/v1/" + suffix, true
}

func isAllowlistedPublicPath(method, path string) bool {
	path = strings.TrimSuffix(path, "/")
	if !strings.HasPrefix(path, "/api/v1/") {
		return false
	}
	rest := strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(rest, "/")

	switch method {
	case httpMethodGet:
		return allowGet(parts, rest)
	case httpMethodPost:
		return allowPost(parts, rest)
	case httpMethodPatch:
		return allowPatch(parts, rest)
	case httpMethodDelete:
		return allowDelete(parts, rest)
	default:
		return false
	}
}

const (
	httpMethodGet    = "GET"
	httpMethodPost   = "POST"
	httpMethodPatch  = "PATCH"
	httpMethodDelete = "DELETE"
)

func allowGet(parts []string, rest string) bool {
	switch {
	case rest == "transport-contracts":
		return true
	case len(parts) == 2 && parts[0] == "transport-contracts":
		return true
	case len(parts) == 3 && parts[0] == "transport-contracts" && parts[2] == "rate-cards":
		return true
	case len(parts) == 2 && parts[0] == "rate-cards":
		return true
	case len(parts) == 3 && parts[0] == "rate-cards" && parts[2] == "versions":
		return true
	case len(parts) == 2 && parts[0] == "rate-card-versions":
		return true
	case len(parts) == 3 && parts[0] == "rate-card-versions" && parts[2] == "rate-lines":
		return true
	case len(parts) == 2 && parts[0] == "rate-lines":
		return true
	case len(parts) == 3 && parts[0] == "rate-lines" && parts[2] == "components":
		return true
	default:
		return false
	}
}

func allowPost(parts []string, rest string) bool {
	switch {
	case rest == "transport-contracts":
		return true
	case rest == "rates/resolve":
		return true
	case len(parts) == 3 && parts[0] == "transport-contracts" && parts[2] == "rate-cards":
		return true
	case len(parts) == 3 && parts[0] == "rate-cards" && parts[2] == "versions":
		return true
	case len(parts) == 3 && parts[0] == "rate-card-versions" && parts[2] == "rate-lines":
		return true
	case len(parts) == 3 && parts[0] == "rate-lines" && parts[2] == "components":
		return true
	case len(parts) == 3 && parts[0] == "transport-contracts" && isLifecycleAction(parts[2]):
		return true
	case len(parts) == 3 && parts[0] == "rate-card-versions" && parts[2] == "activate":
		return true
	default:
		return false
	}
}

func allowPatch(parts []string, rest string) bool {
	switch {
	case len(parts) == 2 && parts[0] == "transport-contracts":
		return true
	case len(parts) == 2 && parts[0] == "rate-card-versions":
		return true
	case len(parts) == 2 && parts[0] == "rate-lines":
		return true
	case len(parts) == 2 && parts[0] == "rate-components":
		return true
	default:
		return false
	}
}

func allowDelete(parts []string, rest string) bool {
	switch {
	case len(parts) == 2 && parts[0] == "rate-card-versions":
		return true
	case len(parts) == 2 && parts[0] == "rate-lines":
		return true
	case len(parts) == 2 && parts[0] == "rate-components":
		return true
	default:
		return false
	}
}

func isLifecycleAction(action string) bool {
	switch action {
	case "activate", "suspend", "reactivate", "terminate", "cancel":
		return true
	default:
		return false
	}
}
