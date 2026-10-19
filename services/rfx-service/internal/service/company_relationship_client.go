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

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const companyRelationshipCreatePath = "/v1/carrier-relationship-requests"

// CompanyRelationshipCreateRequest is the trusted subject rfx-service sends.
// The caller fills every field from the verified user and the current source row.
type CompanyRelationshipCreateRequest struct {
	SourceType              string
	SourceID                uuid.UUID
	SourceTenantID          uuid.UUID
	Fingerprint             string
	ShipperCarrierCompanyID uuid.UUID
	RequestingCompanyID     uuid.UUID
	RequestingUserID        uuid.UUID
	IdempotencyKey          string
}

// CompanyRelationshipCreateResult is the company-service create response.
// AcceptanceToken is set only for the first successful insert.
type CompanyRelationshipCreateResult struct {
	RequestID       uuid.UUID
	Status          string
	AcceptanceToken string
	Created         bool
}

// CompanyRelationshipCreateClient calls company-service with the RFx credential.
// It does not send X-Internal-Service-Name and it does not use INTERNAL_SERVICE_TOKEN.
// SHIPMENT is refused before any HTTP call. An empty URL or token fails closed.
// A transport error is not retried. This client never invents a replacement idempotency key.
// A lost success response is recovered by a future cancel plus a new request, which this stage does not perform.
type CompanyRelationshipCreateClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewCompanyRelationshipCreateClient(baseURL, token string, client *http.Client) *CompanyRelationshipCreateClient {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second, CheckRedirect: refuseCompanyRelationshipRedirect}
	}
	return &CompanyRelationshipCreateClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		client:  client,
	}
}

func (c *CompanyRelationshipCreateClient) Create(ctx context.Context, in CompanyRelationshipCreateRequest) (CompanyRelationshipCreateResult, error) {
	if in.SourceType != domain.RfxSourceTypeAward && in.SourceType != domain.RfxSourceTypeAwardLink {
		return CompanyRelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
	}
	if c == nil || c.client == nil || c.baseURL == "" || c.token == "" || ctx == nil {
		return CompanyRelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	payload := map[string]string{
		"source_type":                   in.SourceType,
		"source_id":                     in.SourceID.String(),
		"source_tenant_id":              in.SourceTenantID.String(),
		"source_version_or_fingerprint": in.Fingerprint,
		"shipper_carrier_company_id":    in.ShipperCarrierCompanyID.String(),
		"requesting_user_id":            in.RequestingUserID.String(),
		"requesting_company_id":         in.RequestingCompanyID.String(),
		"company_id":                    in.RequestingCompanyID.String(),
		"tenant_id":                     in.SourceTenantID.String(),
		"idempotency_key":               in.IdempotencyKey,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return CompanyRelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+companyRelationshipCreatePath, bytes.NewReader(body))
	if err != nil {
		return CompanyRelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Service-Token", c.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return CompanyRelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return CompanyRelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return CompanyRelationshipCreateResult{}, mapCompanyRelationshipError(raw)
	}
	var decoded struct {
		RequestID       string `json:"relationship_request_id"`
		Status          string `json:"request_status"`
		AcceptanceToken string `json:"acceptance_token"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return CompanyRelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	requestID, err := uuid.Parse(strings.TrimSpace(decoded.RequestID))
	if err != nil || strings.TrimSpace(decoded.Status) == "" {
		return CompanyRelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
	out := CompanyRelationshipCreateResult{RequestID: requestID, Status: decoded.Status}
	if resp.StatusCode != http.StatusCreated {
		return out, nil
	}
	if strings.TrimSpace(decoded.AcceptanceToken) == "" {
		return CompanyRelationshipCreateResult{}, apperrors.New(apperrors.CodeRelationshipStoreUnavailable, "relationship store unavailable")
	}
	out.Created = true
	out.AcceptanceToken = decoded.AcceptanceToken
	return out, nil
}

func mapCompanyRelationshipError(raw []byte) error {
	var decoded struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &decoded)
	switch strings.TrimSpace(decoded.Error.Code) {
	case "VALIDATION_ERROR":
		return apperrors.Validation("relationship request is invalid", nil)
	case "RELATIONSHIP_COMMANDS_DISABLED":
		return apperrors.New(apperrors.CodeRelationshipCommandsDisabled, "relationship commands are disabled")
	case "RELATIONSHIP_REQUEST_FORBIDDEN":
		return apperrors.New(apperrors.CodeRelationshipRequestForbidden, "relationship command refused")
	case "RELATIONSHIP_IDEMPOTENCY_CONFLICT":
		return apperrors.New(apperrors.CodeRelationshipIdempotencyConflict, "relationship idempotency conflict")
	case "RELATIONSHIP_REQUEST_PENDING_CONFLICT":
		return apperrors.New(apperrors.CodeRelationshipRequestPendingConflict, "pending relationship request already exists")
	case "RELATIONSHIP_STORE_UNAVAILABLE":
		return apperrors.New(apperrors.CodeRelationshipStoreUnavailable, "relationship store unavailable")
	default:
		return apperrors.New(apperrors.CodeRelationshipSourceUnavailable, "relationship source unavailable")
	}
}

func refuseCompanyRelationshipRedirect(*http.Request, []*http.Request) error {
	return errors.New("company relationship redirect refused")
}
