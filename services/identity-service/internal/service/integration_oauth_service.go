package service

import (
	"context"
	"errors"
	"time"

	"github.com/freight-platform/shared-go/integrationauth"
)

type IntegrationOAuthService struct {
	verifier      *integrationauth.Verifier
	jwtService    *integrationauth.JWTService
	auditor       *integrationauth.AuditRecorder
	limiter       *integrationauth.PrincipalRateLimiter
	failedLimiter *integrationauth.PrincipalRateLimiter
}

func NewIntegrationOAuthService(
	verifier *integrationauth.Verifier,
	jwtService *integrationauth.JWTService,
	auditor *integrationauth.AuditRecorder,
	limiter *integrationauth.PrincipalRateLimiter,
	failedLimiter *integrationauth.PrincipalRateLimiter,
) *IntegrationOAuthService {
	return &IntegrationOAuthService{
		verifier:      verifier,
		jwtService:    jwtService,
		auditor:       auditor,
		limiter:       limiter,
		failedLimiter: failedLimiter,
	}
}

type OAuthTokenResult struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int64
	Scope       string
}

func (s *IntegrationOAuthService) IssueClientCredentialsToken(ctx context.Context, clientID, clientSecret, clientIP, requestID string) (OAuthTokenResult, error) {
	failKey := integrationauth.OAuthFailedAttemptKey(clientIP, clientID)
	if s.failedLimiter != nil {
		if limited, _ := s.failedLimiter.IsLimited(failKey); limited {
			_ = s.auditor.RecordAuthFailure(ctx, nil, nil, integrationauth.ErrRateLimited.Error(), integrationauth.AuthSchemeOAuth, requestID, clientIP)
			return OAuthTokenResult{}, integrationauth.ErrRateLimited
		}
	}
	authCtx, err := s.verifier.AuthenticateOAuthClientCredentials(ctx, clientID, clientSecret, clientIP)
	if err != nil {
		if s.failedLimiter != nil {
			s.failedLimiter.RecordAttempt(failKey)
		}
		_ = s.auditor.RecordAuthFailure(ctx, nil, nil, mapAuthError(err), integrationauth.AuthSchemeOAuth, requestID, clientIP)
		return OAuthTokenResult{}, err
	}
	key := s.limiter.Key(authCtx.TenantID.String(), authCtx.PrincipalID.String(), "oauth_token")
	if allowed, _ := s.limiter.Allow(key); !allowed {
		_ = s.auditor.RecordAuthFailure(ctx, &authCtx.TenantID, &authCtx.PrincipalID, integrationauth.ErrRateLimited.Error(), integrationauth.AuthSchemeOAuth, requestID, clientIP)
		return OAuthTokenResult{}, integrationauth.ErrRateLimited
	}
	token, expiresIn, err := s.jwtService.CreateIntegrationToken(authCtx)
	if err != nil {
		return OAuthTokenResult{}, err
	}
	_ = s.auditor.RecordAuthSuccess(ctx, authCtx, requestID)
	return OAuthTokenResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		Scope:       integrationauth.JoinScopes(authCtx.Scopes),
	}, nil
}

func mapAuthError(err error) string {
	switch {
	case errors.Is(err, integrationauth.ErrInvalidClient):
		return "invalid_client"
	case errors.Is(err, integrationauth.ErrCIDRDenied):
		return "cidr_denied"
	case errors.Is(err, integrationauth.ErrPrincipalInactive):
		return "principal_inactive"
	case errors.Is(err, integrationauth.ErrCredentialExpired):
		return "credential_expired"
	case errors.Is(err, integrationauth.ErrCredentialRevoked):
		return "credential_revoked"
	default:
		return "access_denied"
	}
}

func DefaultIntegrationTokenTTL() time.Duration {
	return integrationauth.TokenTTL
}
