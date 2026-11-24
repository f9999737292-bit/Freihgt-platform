package routeauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/platform/httpredirect"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

var (
	ErrIdentityUnauthorized = errors.New("identity unauthorized")
	ErrIdentityForbidden    = errors.New("identity forbidden")
)

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

type meResponse struct {
	Roles []string `json:"roles"`
}

func (c *IdentityClient) FetchUserRoles(ctx context.Context, reqCtx RequestContext) ([]string, error) {
	endpoint := c.baseURL + "/v1/auth/me"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req, reqCtx)

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, ErrIdentityUnauthorized
	case http.StatusForbidden:
		return nil, ErrIdentityForbidden
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("identity service returned %d", resp.StatusCode)
	}

	var payload meResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Roles, nil
}

func (c *IdentityClient) do(req *http.Request) (*http.Response, error) {
	if c == nil || c.httpClient == nil || req == nil {
		return nil, httpredirect.ErrRejected
	}
	return httpredirect.Do(c.httpClient, c.baseURL, req)
}

func (c *IdentityClient) applyHeaders(req *http.Request, reqCtx RequestContext) {
	if reqCtx.AuthToken != "" {
		req.Header.Set("Authorization", reqCtx.AuthToken)
	}
	if reqCtx.TenantID != "" {
		req.Header.Set("X-Tenant-ID", reqCtx.TenantID)
	}
	if reqCtx.UserID != "" {
		req.Header.Set("X-User-ID", reqCtx.UserID)
	}
	if reqCtx.RequestID != "" {
		req.Header.Set(sharedmiddleware.RequestIDHeader, reqCtx.RequestID)
	}
}
