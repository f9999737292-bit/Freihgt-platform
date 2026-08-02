package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type summaryResponse struct {
	StatusSummary struct {
		Source        string         `json:"source"`
		Total         int64          `json:"total"`
		ByStatus      map[string]int `json:"byStatus"`
		LimitedDataset bool          `json:"limitedDataset"`
	} `json:"statusSummary"`
	StatusSummaryFreshness struct {
		FallbackUsed bool `json:"fallbackUsed"`
	} `json:"statusSummaryFreshness"`
}

func resolveJWT(cfg Config, client *http.Client) (string, error) {
	if cfg.JWTToken != "" {
		return cfg.JWTToken, nil
	}
	if cfg.TenantID == "" || cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		return "", fmt.Errorf("JWT_TOKEN or TENANT_ID+DEV_ADMIN_EMAIL+DEV_ADMIN_PASSWORD required")
	}
	body := fmt.Sprintf(`{"tenant_id":"%s","email":"%s","password":"%s"}`,
		cfg.TenantID, cfg.AdminEmail, cfg.AdminPassword)
	req, err := http.NewRequest(http.MethodPost, cfg.GatewayURL+"/api/v1/auth/login", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if parsed.AccessToken == "" {
		return "", fmt.Errorf("login missing access_token")
	}
	return parsed.AccessToken, nil
}

func fetchTenantBaseline(ctx context.Context, client *http.Client, cfg Config, jwt, tenantID, alias, category string) (TenantBaseline, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.GatewayURL+"/api/v1/control-tower/summary", nil)
	if err != nil {
		return TenantBaseline{}, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-Tenant-ID", tenantID)
	resp, err := client.Do(req)
	if err != nil {
		return TenantBaseline{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return TenantBaseline{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return TenantBaseline{}, fmt.Errorf("summary HTTP %d for alias %s", resp.StatusCode, alias)
	}
	var summary summaryResponse
	if err := json.Unmarshal(raw, &summary); err != nil {
		return TenantBaseline{}, err
	}
	bl := TenantBaseline{
		Alias:          alias,
		Category:       category,
		PublicSource:   summary.StatusSummary.Source,
		LegacyComplete: !summary.StatusSummary.LimitedDataset,
		LimitedDataset: summary.StatusSummary.LimitedDataset,
		FallbackUsed:   summary.StatusSummaryFreshness.FallbackUsed,
		LegacyTotal:    summary.StatusSummary.Total,
	}
	if summary.StatusSummary.Source != "LEGACY" {
		bl.ComparisonCategory = "PUBLIC_SOURCE_NOT_LEGACY"
		return bl, nil
	}
	if summary.StatusSummaryFreshness.FallbackUsed {
		bl.ComparisonCategory = "FALLBACK_USED"
		return bl, nil
	}
	bl.ComparisonCategory = inferComparisonCategory(cfg, client, jwt)
	return bl, nil
}

func inferComparisonCategory(cfg Config, client *http.Client, jwt string) string {
	_ = client
	_ = jwt
	// Shadow comparison category is exposed via Prometheus counters, not public JSON.
	// Gate uses aggregate Prometheus mismatch totals; per-tenant category is classified operationally.
	return "PENDING_PROMETHEUS_CLASSIFICATION"
}

func healthCheck(ctx context.Context, client *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health HTTP %d", resp.StatusCode)
	}
	return nil
}
