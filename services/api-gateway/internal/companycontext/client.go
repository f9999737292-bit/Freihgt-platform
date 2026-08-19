package companycontext

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type UserCompany struct {
	CompanyID   string
	CompanyType string
	RoleCodes   []string
}

type IdentityClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewIdentityClient(httpClient *http.Client, identityURL string) *IdentityClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &IdentityClient{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(identityURL, "/"),
	}
}

type listUserCompaniesResponse struct {
	Items []struct {
		CompanyID   string `json:"company_id"`
		CompanyType string `json:"company_type"`
		Roles       []struct {
			Code string `json:"code"`
		} `json:"roles"`
	} `json:"items"`
}

func (c *IdentityClient) ListUserCompanies(ctx context.Context, reqCtx routeauth.RequestContext, userID string) ([]UserCompany, error) {
	endpoint := fmt.Sprintf("%s/v1/users/%s/companies?tenant_id=%s&status=ACTIVE", c.baseURL, userID, reqCtx.TenantID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req, reqCtx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, routeauth.ErrIdentityUnauthorized
	case http.StatusForbidden:
		return nil, routeauth.ErrIdentityForbidden
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("identity service returned %d", resp.StatusCode)
	}

	var payload listUserCompaniesResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	result := make([]UserCompany, 0, len(payload.Items))
	for _, item := range payload.Items {
		roles := make([]string, 0, len(item.Roles))
		for _, role := range item.Roles {
			if code := strings.TrimSpace(role.Code); code != "" {
				roles = append(roles, code)
			}
		}
		result = append(result, UserCompany{
			CompanyID:   item.CompanyID,
			CompanyType: item.CompanyType,
			RoleCodes:   roles,
		})
	}
	return result, nil
}

func (c *IdentityClient) applyHeaders(req *http.Request, reqCtx routeauth.RequestContext) {
	if reqCtx.AuthToken != "" {
		req.Header.Set("Authorization", reqCtx.AuthToken)
	}
	if reqCtx.TenantID != "" {
		req.Header.Set("X-Tenant-ID", reqCtx.TenantID)
	}
	if reqCtx.UserID != "" {
		req.Header.Set("X-User-ID", reqCtx.UserID)
	}
}
