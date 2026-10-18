package identityguard

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

const platformAdminRole = "PLATFORM_ADMIN"

// PlatformAdminVerifier reports server-verified platform-admin state.
// Implementations must use the caller's bearer token. A generic internal
// token or caller-name header is not authority.
type PlatformAdminVerifier interface {
	IsPlatformAdmin(ctx context.Context, authorization, userID, tenantID string) (bool, error)
}

type Guard struct {
	secret   string
	verifier PlatformAdminVerifier
}

func New(secret string, verifier PlatformAdminVerifier) *Guard {
	return &Guard{secret: secret, verifier: verifier}
}

func (g *Guard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind, targetUser, managed := classify(r.Method, r.URL.Path)
		if !managed {
			next.ServeHTTP(w, r)
			return
		}

		stripUntrusted(r.Header)
		claims, authHeader, err := g.claims(r)
		if err != nil {
			respond.Error(w, err)
			return
		}
		if claims.Subject == "" || claims.TenantID == "" {
			if strings.TrimSpace(claims.Subject) == "" {
				respond.Error(w, apperrors.AuthenticationRequired())
				return
			}
			respond.Error(w, apperrors.TenantContextRequired())
			return
		}
		if _, err := uuid.Parse(claims.Subject); err != nil {
			respond.Error(w, apperrors.AuthenticationRequired())
			return
		}
		if _, err := uuid.Parse(claims.TenantID); err != nil {
			respond.Error(w, apperrors.TenantContextRequired())
			return
		}

		r.Header.Set("X-User-ID", claims.Subject)
		r.Header.Set("X-Tenant-ID", claims.TenantID)
		if claims.Email != "" {
			r.Header.Set("X-User-Email", claims.Email)
		}

		bodyTenant, err := peekBodyTenant(r)
		if err != nil {
			respond.Error(w, err)
			return
		}
		supplied, conflict := combinedTenant(r.URL.Query().Get("tenant_id"), bodyTenant)
		if conflict {
			respond.Error(w, apperrors.TenantContextConflict())
			return
		}

		adminKnown := false
		admin := false
		var adminErr error
		ensureAdmin := func() (bool, error) {
			if adminKnown {
				return admin, adminErr
			}
			adminKnown = true
			if g.verifier == nil {
				adminErr = apperrors.AuthorizationDenied()
				return false, adminErr
			}
			admin, adminErr = g.verifier.IsPlatformAdmin(r.Context(), authHeader, claims.Subject, claims.TenantID)
			return admin, adminErr
		}

		if supplied != "" && !sameTenant(supplied, claims.TenantID) {
			isAdmin, err := ensureAdmin()
			if err != nil || !isAdmin {
				if err != nil {
					respond.Error(w, apperrors.AuthorizationDenied())
					return
				}
				respond.Error(w, apperrors.TenantContextConflict())
				return
			}
		}

		switch kind {
		case kindSelfOrAdmin:
			if targetUser != claims.Subject {
				isAdmin, err := ensureAdmin()
				if err != nil || !isAdmin {
					respond.Error(w, apperrors.AuthorizationDenied())
					return
				}
			}
		case kindAdmin:
			isAdmin, err := ensureAdmin()
			if err != nil || !isAdmin {
				respond.Error(w, apperrors.AuthorizationDenied())
				return
			}
		default:
			respond.Error(w, apperrors.AuthorizationDenied())
			return
		}

		next.ServeHTTP(w, r)
	})
}

type tokenClaims struct {
	TenantID        string `json:"tenant_id"`
	Email           string `json:"email"`
	PreferredLocale string `json:"preferred_locale"`
	jwt.RegisteredClaims
}

func (g *Guard) claims(r *http.Request) (*tokenClaims, string, error) {
	header := r.Header.Get("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return nil, "", apperrors.AuthenticationRequired()
	}
	token, err := jwt.ParseWithClaims(strings.TrimSpace(parts[1]), &tokenClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(g.secret), nil
	})
	if err != nil || token == nil || !token.Valid {
		return nil, "", apperrors.AuthenticationRequired()
	}
	claims, ok := token.Claims.(*tokenClaims)
	if !ok || claims == nil {
		return nil, "", apperrors.AuthenticationRequired()
	}
	return claims, header, nil
}

func stripUntrusted(header http.Header) {
	for _, key := range []string{
		"X-Tenant-ID",
		"X-User-ID",
		"X-User-Email",
		"X-User-Roles",
		"X-Platform-Admin",
		"X-Role",
		"X-Internal-Service-Name",
		"X-Internal-Service-Token",
	} {
		header.Del(key)
	}
}

func peekBodyTenant(r *http.Request) (string, error) {
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodDelete {
		return "", nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", apperrors.Validation("invalid JSON body", map[string]any{"field": "body"})
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return "", nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil
	}
	raw, ok := payload["tenant_id"]
	if !ok || raw == nil {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", apperrors.TenantContextConflict()
	}
	return strings.TrimSpace(value), nil
}

func sameTenant(supplied, verified string) bool {
	left, err := uuid.Parse(strings.TrimSpace(supplied))
	right, rightErr := uuid.Parse(strings.TrimSpace(verified))
	if err != nil || rightErr != nil || left == uuid.Nil || right == uuid.Nil {
		return false
	}
	return left == right
}

func combinedTenant(queryTenant, bodyTenant string) (string, bool) {
	queryTenant = strings.TrimSpace(queryTenant)
	bodyTenant = strings.TrimSpace(bodyTenant)
	if queryTenant != "" && bodyTenant != "" && queryTenant != bodyTenant {
		return "", true
	}
	if queryTenant != "" {
		return queryTenant, false
	}
	return bodyTenant, false
}

type routeKind int

const (
	kindDeny routeKind = iota
	kindAdmin
	kindSelfOrAdmin
)

func classify(method, path string) (routeKind, string, bool) {
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "v1" {
		return kindDeny, "", false
	}
	switch parts[2] {
	case "users":
		return classifyUsers(method, parts)
	case "roles":
		return classifyRoles(method, parts)
	default:
		return kindDeny, "", false
	}
}

func classifyUsers(method string, parts []string) (routeKind, string, bool) {
	switch len(parts) {
	case 3:
		if method == http.MethodGet || method == http.MethodPost {
			return kindAdmin, "", true
		}
	case 4:
		if !validID(parts[3]) {
			return kindDeny, "", true
		}
		switch method {
		case http.MethodGet:
			return kindSelfOrAdmin, parts[3], true
		case http.MethodPatch, http.MethodDelete:
			return kindAdmin, parts[3], true
		}
	case 5:
		if !validID(parts[3]) {
			return kindDeny, "", true
		}
		switch parts[4] {
		case "roles":
			if method == http.MethodGet {
				return kindSelfOrAdmin, parts[3], true
			}
			if method == http.MethodPost {
				return kindAdmin, parts[3], true
			}
		case "companies":
			if method == http.MethodGet {
				return kindSelfOrAdmin, parts[3], true
			}
		}
	case 7:
		if parts[4] == "companies" && parts[6] == "roles" && validID(parts[3]) && validID(parts[5]) && method == http.MethodPost {
			return kindAdmin, parts[3], true
		}
	case 8:
		if parts[4] == "companies" && parts[6] == "roles" && validID(parts[3]) && validID(parts[5]) && validID(parts[7]) && method == http.MethodDelete {
			return kindAdmin, parts[3], true
		}
	}
	return kindDeny, "", true
}

func classifyRoles(method string, parts []string) (routeKind, string, bool) {
	switch len(parts) {
	case 3:
		if method == http.MethodGet {
			return kindAdmin, "", true
		}
	case 5:
		if parts[4] == "permissions" && validID(parts[3]) && method == http.MethodGet {
			return kindAdmin, "", true
		}
	}
	return kindDeny, "", true
}

func validID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
