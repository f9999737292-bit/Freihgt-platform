package rfx

import "net/http"

// E7ErpPreviewRoutes returns Wave E3 ERP CREATE/UPDATE Preview routes (integration-protected).
func E7ErpPreviewRoutes() []ErpMachineAuthRoute {
	return []ErpMachineAuthRoute{
		{
			Name:               "erp_create_draft_preview",
			Method:             http.MethodPost,
			ServiceChiPath:     "/rfx/drafts/preview",
			ServicePath:        "/v1/integrations/erp/rfx/drafts/preview",
			GatewayPath:        "/api/v1/integrations/erp/rfx/drafts/preview",
			OpenAPIPath:        "/api/v1/integrations/erp/rfx/drafts/preview",
			OpenAPIOperationID: "postErpRfxDraftCreatePreview",
			SuccessStatus:      http.StatusOK,
			Public:             false,
		},
		{
			Name:               "erp_update_draft_preview",
			Method:             http.MethodPost,
			ServiceChiPath:     "/{id}/erp-import/preview",
			ServicePath:        "/v1/rfx-events/{id}/erp-import/preview",
			GatewayPath:        "/api/v1/rfx-events/{id}/erp-import/preview",
			OpenAPIPath:        "/api/v1/rfx-events/{id}/erp-import/preview",
			OpenAPIOperationID: "postErpRfxDraftUpdatePreview",
			SuccessStatus:      http.StatusOK,
			Public:             false,
		},
	}
}
