package rfx

import "net/http"

// E7ErpUpdateCommitRoutes returns Wave E5 ERP UPDATE Commit routes (integration-protected).
func E7ErpUpdateCommitRoutes() []ErpMachineAuthRoute {
	return []ErpMachineAuthRoute{
		{
			Name:               "erp_update_draft_commit",
			Method:             http.MethodPost,
			ServiceChiPath:     "/{id}/erp-import/commit",
			ServicePath:        "/v1/rfx-events/{id}/erp-import/commit",
			GatewayPath:        "/api/v1/rfx-events/{id}/erp-import/commit",
			OpenAPIPath:        "/api/v1/rfx-events/{id}/erp-import/commit",
			OpenAPIOperationID: "postErpRfxDraftUpdateCommit",
			SuccessStatus:      http.StatusOK,
			Public:             false,
		},
	}
}
