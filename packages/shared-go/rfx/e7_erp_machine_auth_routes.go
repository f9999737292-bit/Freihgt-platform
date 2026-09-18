package rfx

import "net/http"

// ErpMachineAuthRoute is the canonical parity anchor for RFx v3.0E7 Phase 2 ERP machine auth (Wave E2).
type ErpMachineAuthRoute struct {
	Name               string
	Method             string
	ServiceChiPath     string
	ServicePath        string
	GatewayPath        string
	OpenAPIPath        string
	OpenAPIOperationID string
	SuccessStatus      int
	Public             bool
}

// E7ErpMachineAuthRoutes returns Wave E2 OAuth token route only.
func E7ErpMachineAuthRoutes() []ErpMachineAuthRoute {
	return []ErpMachineAuthRoute{
		{
			Name:               "oauth_client_credentials_token",
			Method:             http.MethodPost,
			ServiceChiPath:     "/oauth/token",
			ServicePath:        "/v1/integrations/oauth/token",
			GatewayPath:        "/api/v1/integrations/oauth/token",
			OpenAPIPath:        "/api/v1/integrations/oauth/token",
			OpenAPIOperationID: "post_obtain_oauth_access_token_via_client_credentials",
			SuccessStatus:      http.StatusOK,
			Public:             true,
		},
	}
}
