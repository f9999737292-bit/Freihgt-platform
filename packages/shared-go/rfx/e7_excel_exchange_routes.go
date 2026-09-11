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

// E7ExcelExchangeRoutes returns exactly one buyer draft XLSX export operation.
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
	}
}
