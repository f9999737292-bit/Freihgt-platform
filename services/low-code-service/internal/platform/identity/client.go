package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RoleChecker interface {
	ListUserRoleCodes(ctx context.Context, userID, tenantID uuid.UUID) ([]string, error)
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type userRolesResponse struct {
	Items []struct {
		Code string `json:"code"`
	} `json:"items"`
}

func (c *Client) ListUserRoleCodes(ctx context.Context, userID, tenantID uuid.UUID) ([]string, error) {
	if c == nil || c.baseURL == "" {
		return nil, fmt.Errorf("identity service url is not configured")
	}
	url := fmt.Sprintf("%s/v1/users/%s/roles?tenant_id=%s", c.baseURL, userID.String(), tenantID.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload userRolesResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
		code := strings.TrimSpace(item.Code)
		if code != "" {
			codes = append(codes, code)
		}
	}
	return codes, nil
}
