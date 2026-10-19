package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/company-service/internal/domain"
)

const (
	rfxSourceAuthorityConfirmPath    = "/internal/v1/rfx-source-authority/confirm"
	rfxSourceAuthorityRevalidatePath = "/internal/v1/rfx-source-authority/revalidate"
	rfxSourceAuthorityHeader         = "X-Company-Relationship-Revalidation-Token"
)

// RfxSourceAuthorityClient is the company-service caller for the RFx owner callback.
// Shipment sources are not this client's authority. An empty URL or token is not a client.
// The call is not retried: acceptance revalidation runs inside the company transaction.
type RfxSourceAuthorityClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewRfxSourceAuthorityClient(baseURL, token string, client *http.Client) *RfxSourceAuthorityClient {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second, CheckRedirect: refuseRfxSourceRedirect}
	}
	return &RfxSourceAuthorityClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		client:  client,
	}
}

// WireRfxSourceAuthority keeps the fail-closed owner unless both the RFx callback URL and the revalidation token are configured.
func WireRfxSourceAuthority(baseURL, token string) (domain.SourceRevalidator, domain.SourceCreateAuthority) {
	baseURL = strings.TrimSpace(baseURL)
	token = strings.TrimSpace(token)
	if baseURL == "" || token == "" {
		return FailClosedSourceRevalidator{}, FailClosedSourceAuthority{}
	}
	client := NewRfxSourceAuthorityClient(baseURL, token, nil)
	return client, client
}

func (c *RfxSourceAuthorityClient) ConfirmCreate(ctx context.Context, subject domain.SourceCreateSubject) (domain.SourceCreateDecision, error) {
	if c == nil || !rfxSourceType(subject.SourceType) {
		return domain.SourceCreateUnavailable, nil
	}
	decision, ok := c.post(ctx, rfxSourceAuthorityConfirmPath, map[string]string{
		"source_type":                   subject.SourceType,
		"source_id":                     subject.SourceID.String(),
		"source_tenant_id":              subject.SourceTenantID.String(),
		"source_version_or_fingerprint": subject.Fingerprint,
		"shipper_carrier_company_id":    subject.ShipperCarrierCompanyID.String(),
		"requesting_company_id":         subject.RequestingCompanyID.String(),
		"requesting_user_id":            subject.RequestingUserID.String(),
	})
	if !ok {
		return domain.SourceCreateUnavailable, nil
	}
	switch domain.SourceCreateDecision(decision) {
	case domain.SourceCreateConfirmed, domain.SourceCreateNotFound, domain.SourceCreateTenantMismatch, domain.SourceCreateStale, domain.SourceCreateActorMismatch, domain.SourceCreateUnavailable:
		return domain.SourceCreateDecision(decision), nil
	default:
		return domain.SourceCreateUnavailable, nil
	}
}

func (c *RfxSourceAuthorityClient) Revalidate(ctx context.Context, source domain.SourceSnapshot) (domain.SourceDecision, error) {
	if c == nil || !rfxSourceType(source.SourceType) {
		return domain.SourceUnavailable, nil
	}
	decision, ok := c.post(ctx, rfxSourceAuthorityRevalidatePath, map[string]string{
		"source_type":                   source.SourceType,
		"source_id":                     source.SourceID.String(),
		"source_tenant_id":              source.SourceTenantID.String(),
		"source_version_or_fingerprint": source.Fingerprint,
		"shipper_carrier_company_id":    source.ShipperCarrierCompanyID.String(),
	})
	if !ok {
		return domain.SourceUnavailable, nil
	}
	switch domain.SourceDecision(decision) {
	case domain.SourceMatch, domain.SourceNotFound, domain.SourceChanged, domain.SourceUnavailable:
		return domain.SourceDecision(decision), nil
	default:
		return domain.SourceUnavailable, nil
	}
}

func rfxSourceType(sourceType string) bool {
	return sourceType == domain.SourceTypeRfxAward || sourceType == domain.SourceTypeAwardLink
}

func (c *RfxSourceAuthorityClient) post(ctx context.Context, path string, payload map[string]string) (string, bool) {
	if c == nil || c.client == nil || c.baseURL == "" || c.token == "" || ctx == nil {
		return "", false
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return "", false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(rfxSourceAuthorityHeader, c.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", false
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		return "", false
	}
	var decoded struct {
		Decision string `json:"decision"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", false
	}
	decision := strings.TrimSpace(decoded.Decision)
	if decision == "" {
		return "", false
	}
	return decision, true
}

func refuseRfxSourceRedirect(*http.Request, []*http.Request) error {
	return errors.New("rfx source authority redirect refused")
}
