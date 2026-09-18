package integrationauth

import "errors"

var (
	ErrInvalidClient       = errors.New("invalid_client")
	ErrInvalidGrant        = errors.New("invalid_grant")
	ErrUnauthorizedClient  = errors.New("unauthorized_client")
	ErrAccessDenied        = errors.New("access_denied")
	ErrAuthSchemeDenied    = errors.New("auth_scheme_denied")
	ErrRateLimited         = errors.New("rate_limited")
	ErrInvalidCredentials  = errors.New("invalid_credentials")
	ErrPrincipalInactive   = errors.New("principal_inactive")
	ErrCredentialExpired   = errors.New("credential_expired")
	ErrCredentialRevoked   = errors.New("credential_revoked")
	ErrCIDRDenied          = errors.New("cidr_denied")
	ErrUnsupportedAlgorithm = errors.New("unsupported_hash_algorithm")
)
