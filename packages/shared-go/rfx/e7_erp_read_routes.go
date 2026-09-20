package rfx

import "net/http"

// E7ErpReadRoutes returns Wave E6 ERP GET routes (integration-protected).
func E7ErpReadRoutes() []ErpMachineAuthRoute {
	return []ErpMachineAuthRoute{
		{
			Name:               "erp_get_rfx_event_by_id",
			Method:             http.MethodGet,
			ServiceChiPath:     "/rfx-events/{id}",
			ServicePath:        "/v1/integrations/erp/rfx-events/{id}",
			GatewayPath:        "/api/v1/integrations/erp/rfx-events/{id}",
			OpenAPIPath:        "/api/v1/integrations/erp/rfx-events/{id}",
			OpenAPIOperationID: "getErpRfxEventById",
			SuccessStatus:      http.StatusOK,
			Public:             false,
		},
		{
			Name:               "erp_get_rfx_by_external_id",
			Method:             http.MethodGet,
			ServiceChiPath:     "/rfx/by-external-id",
			ServicePath:        "/v1/integrations/erp/rfx/by-external-id",
			GatewayPath:        "/api/v1/integrations/erp/rfx/by-external-id",
			OpenAPIPath:        "/api/v1/integrations/erp/rfx/by-external-id",
			OpenAPIOperationID: "getErpRfxByExternalId",
			SuccessStatus:      http.StatusOK,
			Public:             false,
		},
		{
			Name:               "erp_get_import_analysis_status",
			Method:             http.MethodGet,
			ServiceChiPath:     "/analyses/{analysis_id}",
			ServicePath:        "/v1/integrations/erp/analyses/{analysis_id}",
			GatewayPath:        "/api/v1/integrations/erp/analyses/{analysis_id}",
			OpenAPIPath:        "/api/v1/integrations/erp/analyses/{analysis_id}",
			OpenAPIOperationID: "getErpImportAnalysisStatus",
			SuccessStatus:      http.StatusOK,
			Public:             false,
		},
		{
			Name:               "erp_get_integration_capabilities",
			Method:             http.MethodGet,
			ServiceChiPath:     "/capabilities",
			ServicePath:        "/v1/integrations/erp/capabilities",
			GatewayPath:        "/api/v1/integrations/erp/capabilities",
			OpenAPIPath:        "/api/v1/integrations/erp/capabilities",
			OpenAPIOperationID: "getErpIntegrationCapabilities",
			SuccessStatus:      http.StatusOK,
			Public:             false,
		},
	}
}
