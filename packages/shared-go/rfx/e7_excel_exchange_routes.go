package rfx

import "net/http"

// ExcelExchangeRoute is the canonical parity anchor for RFx v3.0E7 Phase 2 buyer XLSX export.
type ExcelExchangeRoute struct {
	Name                 string
	Method               string
	ServiceChiPath       string
	ServicePath          string
	GatewayPath          string
	OpenAPIPath          string
	OpenAPIOperationID   string
	RBACPolicy           string
	SuccessStatus        int
	IdempotencyRequired  bool
	FeatureFlagProtected bool
}

// E7ExcelExchangeRoutes returns buyer draft XLSX export and import preview operations.
func E7ExcelExchangeRoutes() []ExcelExchangeRoute {
	return []ExcelExchangeRoute{
		{
			Name:                 "export_buyer_draft_xlsx",
			Method:               http.MethodGet,
			ServiceChiPath:       "/{id}/xlsx-export",
			ServicePath:          "/v1/rfx-events/{id}/xlsx-export",
			GatewayPath:          "/api/v1/rfx-events/{id}/xlsx-export",
			OpenAPIPath:          "/api/v1/rfx-events/{id}/xlsx-export",
			OpenAPIOperationID:   "get_export_buyer_draft_rfx_event_as_xlsx_workbook",
			RBACPolicy:           "PolicyBuyerManage",
			SuccessStatus:        http.StatusOK,
			IdempotencyRequired:  false,
			FeatureFlagProtected: true,
		},
		{
			Name:                 "preview_buyer_draft_xlsx_import",
			Method:               http.MethodPost,
			ServiceChiPath:       "/{id}/xlsx-import/preview",
			ServicePath:          "/v1/rfx-events/{id}/xlsx-import/preview",
			GatewayPath:          "/api/v1/rfx-events/{id}/xlsx-import/preview",
			OpenAPIPath:          "/api/v1/rfx-events/{id}/xlsx-import/preview",
			OpenAPIOperationID:   "post_preview_buyer_draft_rfx_event_xlsx_import",
			RBACPolicy:           "PolicyBuyerManage",
			SuccessStatus:        http.StatusOK,
			IdempotencyRequired:  false,
			FeatureFlagProtected: true,
		},
	}
}
