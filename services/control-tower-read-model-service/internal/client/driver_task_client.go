package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type DriverTaskClient interface {
	CreateTask(ctx context.Context, req CreateDriverTaskRequest) (CreateDriverTaskResponse, error)
}

type CreateDriverTaskRequest struct {
	TenantID       uuid.UUID
	DriverID       uuid.UUID
	ShipmentID     *uuid.UUID
	TaskType       string
	Priority       string
	ExpiresAt      *time.Time
	Source         string
	SourceEventID  string
	CorrelationID  string
	IdempotencyKey string
	CreatedByType  string
	CreatedByID    *uuid.UUID
}

type CreateDriverTaskResponse struct {
	TaskID   uuid.UUID
	Status   string
	TaskType string
}

type HTTPDriverTaskClient struct {
	baseURL       string
	internalToken string
	httpClient    *http.Client
}

func NewHTTPDriverTaskClient(baseURL, internalToken string) *HTTPDriverTaskClient {
	return &HTTPDriverTaskClient{
		baseURL:       strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		internalToken: strings.TrimSpace(internalToken),
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *HTTPDriverTaskClient) CreateTask(ctx context.Context, req CreateDriverTaskRequest) (CreateDriverTaskResponse, error) {
	if c.baseURL == "" {
		return CreateDriverTaskResponse{}, fmt.Errorf("driver task service URL is not configured")
	}
	body := map[string]any{
		"tenantId":       req.TenantID.String(),
		"driverId":       req.DriverID.String(),
		"type":           req.TaskType,
		"priority":       req.Priority,
		"source":         req.Source,
		"idempotencyKey": req.IdempotencyKey,
		"createdByType":  req.CreatedByType,
	}
	if req.ShipmentID != nil {
		body["shipmentId"] = req.ShipmentID.String()
	}
	if req.ExpiresAt != nil {
		body["expiresAt"] = req.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if req.SourceEventID != "" {
		body["sourceEventId"] = req.SourceEventID
	}
	if req.CorrelationID != "" {
		body["correlationId"] = req.CorrelationID
	}
	if req.CreatedByID != nil {
		body["createdById"] = req.CreatedByID.String()
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return CreateDriverTaskResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/driver/tasks", bytes.NewReader(raw))
	if err != nil {
		return CreateDriverTaskResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Service-Token", c.internalToken)
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return CreateDriverTaskResponse{}, err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		var parsed struct {
			TaskID   string `json:"taskId"`
			Status   string `json:"status"`
			TaskType string `json:"taskType"`
		}
		if err := json.Unmarshal(payload, &parsed); err != nil {
			return CreateDriverTaskResponse{}, fmt.Errorf("decode driver task response: %w", err)
		}
		taskID, err := uuid.Parse(strings.TrimSpace(parsed.TaskID))
		if err != nil {
			return CreateDriverTaskResponse{}, fmt.Errorf("invalid task id in response")
		}
		return CreateDriverTaskResponse{TaskID: taskID, Status: parsed.Status, TaskType: parsed.TaskType}, nil
	}
	return CreateDriverTaskResponse{}, fmt.Errorf("driver task service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
}
