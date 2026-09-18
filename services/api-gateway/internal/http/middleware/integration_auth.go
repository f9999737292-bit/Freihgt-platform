package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	"github.com/freight-platform/shared-go/integrationauth"
)

type IntegrationAuthConfig struct {
	Enabled              bool
	JWTSecret            string
	IdentityInternalURL  string
	InternalServiceToken string
	RateLimiter          *integrationauth.PrincipalRateLimiter
}

type IntegrationAuthContext struct {
	Authenticated integrationauth.AuthenticatedContext
}

type integrationAuthContextKey struct{}

func WithIntegrationAuthContext(ctx context.Context, ac IntegrationAuthContext) context.Context {
	return context.WithValue(ctx, integrationAuthContextKey{}, ac)
}

func IntegrationAuthFromContext(ctx context.Context) (IntegrationAuthContext, bool) {
	value, ok := ctx.Value(integrationAuthContextKey{}).(IntegrationAuthContext)
	return value, ok
}

func IntegrationAuth(cfg IntegrationAuthConfig) func(http.Handler) http.Handler {
	integrationJWT := integrationauth.NewJWTService(cfg.JWTSecret, integrationauth.TokenTTL)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled || !requiresIntegrationAuth(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			integrationauth.StripUntrustedIntegrationHeaders(r.Header)
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" {
				respond.Error(w, apperrors.Unauthorized("authorization header is required"))
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				respond.Error(w, apperrors.Unauthorized("invalid authorization header"))
				return
			}
			token := strings.TrimSpace(parts[1])
			var authCtx integrationauth.AuthenticatedContext
			var err error
			switch {
			case integrationauth.IsAPIKeyBearer(token):
				authCtx, err = verifyAPIKeyViaIdentity(r.Context(), cfg, token, clientIPFromRequest(r))
			default:
				authCtx, err = verifyIntegrationJWT(integrationJWT, token)
			}
			if err != nil {
				respond.Error(w, mapIntegrationAuthError(err))
				return
			}
			if cfg.RateLimiter != nil {
				key := cfg.RateLimiter.Key(authCtx.TenantID.String(), authCtx.PrincipalID.String(), "integration_request")
				if allowed, retryAfter := cfg.RateLimiter.Allow(key); !allowed {
					if retryAfter > 0 {
						w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
					}
					respond.Error(w, apperrors.RateLimitExceeded("too many integration requests"))
					return
				}
			}
			integrationauth.InjectTrustedIntegrationHeaders(r.Header, authCtx)
			r = r.WithContext(WithIntegrationAuthContext(r.Context(), IntegrationAuthContext{Authenticated: authCtx}))
			next.ServeHTTP(w, r)
		})
	}
}

func requiresIntegrationAuth(path string) bool {
	return strings.HasPrefix(path, "/api/v1/integrations/")
}

func verifyIntegrationJWT(jwtService *integrationauth.JWTService, token string) (integrationauth.AuthenticatedContext, error) {
	claims, err := jwtService.ParseIntegrationToken(token)
	if err != nil {
		return integrationauth.AuthenticatedContext{}, integrationauth.ErrInvalidCredentials
	}
	return integrationauth.ClaimsToContext(claims)
}

type verifyBearerResponse struct {
	PrincipalID string   `json:"principal_id"`
	TenantID    string   `json:"tenant_id"`
	CompanyID   string   `json:"company_id"`
	AuthScheme  string   `json:"auth_scheme"`
	Scopes      []string `json:"scopes"`
}

func verifyAPIKeyViaIdentity(ctx context.Context, cfg IntegrationAuthConfig, token, clientIP string) (integrationauth.AuthenticatedContext, error) {
	if strings.TrimSpace(cfg.IdentityInternalURL) == "" {
		return integrationauth.AuthenticatedContext{}, integrationauth.ErrInvalidCredentials
	}
	body, _ := json.Marshal(map[string]string{"bearer": token})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.IdentityInternalURL, "/")+"/internal/v1/integrations/auth/verify-bearer", bytes.NewReader(body))
	if err != nil {
		return integrationauth.AuthenticatedContext{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Service-Token", cfg.InternalServiceToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return integrationauth.AuthenticatedContext{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		raw, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(raw), "auth_scheme_denied") {
			return integrationauth.AuthenticatedContext{}, integrationauth.ErrAuthSchemeDenied
		}
		return integrationauth.AuthenticatedContext{}, integrationauth.ErrInvalidCredentials
	}
	if resp.StatusCode == http.StatusForbidden {
		return integrationauth.AuthenticatedContext{}, integrationauth.ErrCIDRDenied
	}
	if resp.StatusCode != http.StatusOK {
		return integrationauth.AuthenticatedContext{}, integrationauth.ErrInvalidCredentials
	}
	var out verifyBearerResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return integrationauth.AuthenticatedContext{}, err
	}
	principalID, err := uuid.Parse(out.PrincipalID)
	if err != nil {
		return integrationauth.AuthenticatedContext{}, integrationauth.ErrInvalidCredentials
	}
	tenantID, err := uuid.Parse(out.TenantID)
	if err != nil {
		return integrationauth.AuthenticatedContext{}, integrationauth.ErrInvalidCredentials
	}
	companyID, err := uuid.Parse(out.CompanyID)
	if err != nil {
		return integrationauth.AuthenticatedContext{}, integrationauth.ErrInvalidCredentials
	}
	scopes, err := integrationauth.NormalizeScopes(out.Scopes)
	if err != nil {
		return integrationauth.AuthenticatedContext{}, err
	}
	return integrationauth.AuthenticatedContext{
		PrincipalID: principalID,
		TenantID:    tenantID,
		CompanyID:   companyID,
		AuthScheme:  out.AuthScheme,
		Scopes:      scopes,
		ClientIP:    clientIP,
	}, nil
}

func mapIntegrationAuthError(err error) *apperrors.AppError {
	switch {
	case errors.Is(err, integrationauth.ErrAuthSchemeDenied):
		return apperrors.Unauthorized("auth_scheme_denied")
	case errors.Is(err, integrationauth.ErrCIDRDenied):
		return apperrors.Forbidden("access_denied")
	case errors.Is(err, integrationauth.ErrRateLimited):
		return apperrors.RateLimitExceeded("too many requests")
	default:
		return apperrors.Unauthorized("invalid or expired token")
	}
}

func AuthWithIntegrationSupport(enabled bool, jwtSecret string, integrationJWT *integrationauth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled || isPublicRoute(r.Method, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			requestedCompanyID := strings.TrimSpace(r.Header.Get("X-Company-ID"))
			StripUntrustedIdentityHeaders(r.Header)
			integrationauth.StripUntrustedIntegrationHeaders(r.Header)

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respond.Error(w, apperrors.Unauthorized("authorization header is required"))
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				respond.Error(w, apperrors.Unauthorized("invalid authorization header"))
				return
			}
			token := strings.TrimSpace(parts[1])

			if integrationJWT != nil {
				if claims, err := integrationJWT.ParseIntegrationToken(token); err == nil && claims.IsIntegrationToken() {
					authCtx, err := integrationauth.ClaimsToContext(claims)
					if err != nil {
						respond.Error(w, apperrors.Unauthorized("invalid integration token"))
						return
					}
					if claims.AuthScheme != "" && claims.AuthScheme != integrationauth.AuthSchemeOAuth {
						respond.Error(w, apperrors.Unauthorized("auth_scheme_denied"))
						return
					}
					integrationauth.InjectTrustedIntegrationHeaders(r.Header, authCtx)
					next.ServeHTTP(w, r)
					return
				}
			}

			claims, err := parseToken(token, jwtSecret)
			if err != nil {
				respond.Error(w, apperrors.Unauthorized("invalid or expired token"))
				return
			}
			ac := AuthContext{AuthToken: authHeader, RequestedCompanyID: requestedCompanyID}
			if claims.Subject != "" {
				ac.UserID = claims.Subject
				r.Header.Set("X-User-ID", claims.Subject)
			}
			if claims.TenantID != "" {
				ac.TenantID = claims.TenantID
				r.Header.Set("X-Tenant-ID", claims.TenantID)
			}
			if claims.Email != "" {
				ac.Email = claims.Email
				r.Header.Set("X-User-Email", claims.Email)
			}
			r = r.WithContext(WithAuthContext(r.Context(), ac))
			next.ServeHTTP(w, r)
		})
	}
}

func clientIPFromRequest(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	return strings.TrimSpace(r.RemoteAddr)
}

// ParseIntegrationTokenForTest exposes integration token parsing for unit tests.
func ParseIntegrationTokenForTest(tokenString, secret string) (*integrationauth.IntegrationTokenClaims, error) {
	svc := integrationauth.NewJWTService(secret, integrationauth.TokenTTL)
	return svc.ParseIntegrationToken(tokenString)
}

// IsIntegrationTokenClaims detects integration JWT without importing jwt in tests.
func IsIntegrationTokenClaims(token *jwt.Token) bool {
	claims, ok := token.Claims.(*integrationauth.IntegrationTokenClaims)
	return ok && claims.IsIntegrationToken()
}
