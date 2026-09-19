package rfx

import "net/http"

// E7ErpCreateCommitRoutes returns Wave E4 ERP CREATE Commit routes (integration-protected).
func E7ErpCreateCommitRoutes() []ErpMachineAuthRoute {
	return []ErpMachineAuthRoute{
		{
			Name:               "erp_create_draft_commit",
			Method:             http.MethodPost,
			ServiceChiPath:     "/rfx/drafts/commit",
			ServicePath:        "/v1/integrations/erp/rfx/drafts/commit",
			GatewayPath:        "/api/v1/integrations/erp/rfx/drafts/commit",
			OpenAPIPath:        "/api/v1/integrations/erp/rfx/drafts/commit",
			OpenAPIOperationID: "postErpRfxDraftCreateCommit",
			SuccessStatus:      http.StatusCreated,
			Public:             false,
		},
	}
}
