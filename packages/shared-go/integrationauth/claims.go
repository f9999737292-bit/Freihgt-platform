package integrationauth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenIssuer   = "bintrans/integrations"
	TokenAudience = "bintrans/api"
	TokenTTL      = 15 * time.Minute
)

type AuthenticatedContext struct {
	PrincipalID  uuid.UUID
	TenantID     uuid.UUID
	CompanyID    uuid.UUID
	CredentialID uuid.UUID
	AuthScheme   string
	Scopes       []string
	ClientIP     string
}

type IntegrationTokenClaims struct {
	TenantID   string `json:"tenant_id"`
	CompanyID  string `json:"company_id"`
	Scope      string `json:"scope"`
	AuthScheme string `json:"auth_scheme"`
	Act        struct {
		Kind string `json:"kind"`
	} `json:"act"`
	jwt.RegisteredClaims
}

func (c *IntegrationTokenClaims) IsIntegrationToken() bool {
	return c.Act.Kind == ActorKindIntegration
}
