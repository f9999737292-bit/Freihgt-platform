package rfx

import "net/http"

// LateSubmissionRoute is the canonical parity anchor for RFx v3.0E7 Phase 1 HTTP routes.
type LateSubmissionRoute struct {
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

// E7LateSubmissionRoutes returns exactly five late-submission operations.
func E7LateSubmissionRoutes() []LateSubmissionRoute {
	return []LateSubmissionRoute{
		{
			Name:                 "create_late_request",
			Method:               http.MethodPost,
			ServiceChiPath:       "/{id}/late-submission-requests",
			ServicePath:          "/v1/rfx-events/{id}/late-submission-requests",
			GatewayPath:          "/api/v1/rfx-events/{id}/late-submission-requests",
			OpenAPIPath:          "/api/v1/rfx-events/{id}/late-submission-requests",
			OpenAPIOperationID:   "post_create_carrier_late_submission_request_after_response_deadline",
			RBACPolicy:           "PolicyCarrierRespond",
			SuccessStatus:        http.StatusCreated,
			IdempotencyRequired:  true,
			FeatureFlagProtected: true,
		},
		{
			Name:                 "list_own_requests",
			Method:               http.MethodGet,
			ServiceChiPath:       "/{id}/late-submission-requests/mine",
			ServicePath:          "/v1/rfx-events/{id}/late-submission-requests/mine",
			GatewayPath:          "/api/v1/rfx-events/{id}/late-submission-requests/mine",
			OpenAPIPath:          "/api/v1/rfx-events/{id}/late-submission-requests/mine",
			OpenAPIOperationID:   "get_list_own_late_submission_requests_for_carrier",
			RBACPolicy:           "PolicyCarrierRead",
			SuccessStatus:        http.StatusOK,
			IdempotencyRequired:  false,
			FeatureFlagProtected: true,
		},
		{
			Name:                 "buyer_queue",
			Method:               http.MethodGet,
			ServiceChiPath:       "/{id}/late-submission-requests",
			ServicePath:          "/v1/rfx-events/{id}/late-submission-requests",
			GatewayPath:          "/api/v1/rfx-events/{id}/late-submission-requests",
			OpenAPIPath:          "/api/v1/rfx-events/{id}/late-submission-requests",
			OpenAPIOperationID:   "get_list_late_submission_requests_for_buyer_review_queue",
			RBACPolicy:           "PolicyBuyerRead",
			SuccessStatus:        http.StatusOK,
			IdempotencyRequired:  false,
			FeatureFlagProtected: true,
		},
		{
			Name:                 "approve_request",
			Method:               http.MethodPost,
			ServiceChiPath:       "/{id}/late-submission-requests/{request_id}/approve",
			ServicePath:          "/v1/rfx-events/{id}/late-submission-requests/{request_id}/approve",
			GatewayPath:          "/api/v1/rfx-events/{id}/late-submission-requests/{request_id}/approve",
			OpenAPIPath:          "/api/v1/rfx-events/{id}/late-submission-requests/{request_id}/approve",
			OpenAPIOperationID:   "post_approve_carrier_late_submission_request",
			RBACPolicy:           "PolicyBuyerManage",
			SuccessStatus:        http.StatusOK,
			IdempotencyRequired:  true,
			FeatureFlagProtected: true,
		},
		{
			Name:                 "reject_request",
			Method:               http.MethodPost,
			ServiceChiPath:       "/{id}/late-submission-requests/{request_id}/reject",
			ServicePath:          "/v1/rfx-events/{id}/late-submission-requests/{request_id}/reject",
			GatewayPath:          "/api/v1/rfx-events/{id}/late-submission-requests/{request_id}/reject",
			OpenAPIPath:          "/api/v1/rfx-events/{id}/late-submission-requests/{request_id}/reject",
			OpenAPIOperationID:   "post_reject_carrier_late_submission_request",
			RBACPolicy:           "PolicyBuyerManage",
			SuccessStatus:        http.StatusOK,
			IdempotencyRequired:  true,
			FeatureFlagProtected: true,
		},
	}
}
