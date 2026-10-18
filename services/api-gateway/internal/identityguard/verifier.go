package identityguard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPVerifier asks identity-service for the authenticated subject's
// tenant-global roles using that subject's bearer token.
type HTTPVerifier struct {
	baseURL string
	client  *http.Client
}

func NewHTTPVerifier(identityURL string) *HTTPVerifier {
	return &HTTPVerifier{
		baseURL: strings.TrimRight(strings.TrimSpace(identityURL), "/"),
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

type roleListResponse struct {
	Items []struct {
		Code      string  `json:"code"`
		CompanyID *string `json:"company_id"`
	} `json:"items"`
}

func (v *HTTPVerifier) IsPlatformAdmin(ctx context.Context, authorization, userID, tenantID string) (bool, error) {
	if v == nil || v.baseURL == "" || strings.TrimSpace(authorization) == "" {
		return false, fmt.Errorf("platform admin verification unavailable")
	}
	endpoint := fmt.Sprintf("%s/v1/users/%s/roles?tenant_id=%s", v.baseURL, url.PathEscape(userID), url.QueryEscape(tenantID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Accept", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("identity role lookup returned %d", resp.StatusCode)
	}
	var payload roleListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, err
	}
	for _, item := range payload.Items {
		if item.CompanyID != nil && strings.TrimSpace(*item.CompanyID) != "" {
			continue
		}
		if strings.TrimSpace(item.Code) == platformAdminRole {
			return true, nil
		}
	}
	return false, nil
}
