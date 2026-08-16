package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

type ExceptionIntegrator struct {
	readModel *controltowerreadmodel.Client
	enabled   bool
	timeout   time.Duration
}

func NewExceptionIntegrator(readModel *controltowerreadmodel.Client, enabled bool, timeout time.Duration) *ExceptionIntegrator {
	return &ExceptionIntegrator{
		readModel: readModel,
		enabled:   enabled,
		timeout:   timeout,
	}
}

type ExceptionIntegrationInput struct {
	ExceptionID string
	ShipmentID  string
	Category    string
	OccurredAt  time.Time
	Replayed    bool
}

func (i *ExceptionIntegrator) Integrate(ctx context.Context, reqCtx RequestContext, input ExceptionIntegrationInput) error {
	if i == nil || !i.enabled || i.readModel == nil || input.Replayed {
		return nil
	}

	severity := mapDriverExceptionSeverity(input.Category)
	seed := controltowerreadmodel.EnsureExceptionSeed{
		EventID:    normalizeDriverExceptionEventID(input.ExceptionID),
		ShipmentID: input.ShipmentID,
		EventType:  mapDriverExceptionEventType(input.Category),
		Source:     "driver",
		OccurredAt: input.OccurredAt,
		Severity:   severity,
	}

	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), i.timeout)
	defer cancel()

	createdIDs, depErr := i.readModel.EnsureExceptionWorkflows(rmCtx, reqCtx.TenantID, reqCtx.RequestID, []controltowerreadmodel.EnsureExceptionSeed{seed})
	if depErr != nil {
		return fmt.Errorf("ensure exception workflow: %w", depErr.Err)
	}

	created := map[string]struct{}{}
	for _, id := range createdIDs {
		created[id] = struct{}{}
	}
	if _, ok := created[seed.EventID]; !ok {
		return nil
	}

	stateVersion := fmt.Sprintf("%s:%s:%s", seed.EventID, seed.EventType, seed.Severity)
	body, _ := json.Marshal(map[string]any{
		"triggerType":   "exception_created",
		"triggerId":     fmt.Sprintf("exception:%s:%s", seed.EventID, stateVersion),
		"shipmentId":    seed.ShipmentID,
		"exceptionId":   seed.EventID,
		"workItemType":  "exception",
		"workItemId":    seed.EventID,
		"correlationId": fmt.Sprintf("driver-exception:%s", seed.EventID),
		"sourceOrigin":  "driver.exception_reported",
		"attributes": map[string]any{
			"exceptionCategory": seed.EventType,
			"priority":          seed.Severity,
			"stateVersion":      stateVersion,
			"source":            seed.Source,
		},
		"persist": true,
	})
	_, depErr = i.readModel.EvaluateAutomation(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, body)
	if depErr != nil {
		return fmt.Errorf("automation evaluate: %w", depErr.Err)
	}
	return nil
}

func mapDriverExceptionEventType(category string) string {
	category = strings.TrimSpace(strings.ToUpper(category))
	switch category {
	case "VEHICLE_BREAKDOWN":
		return "vehicle_breakdown"
	case "ACCIDENT":
		return "accident"
	case "TRAFFIC":
		return "traffic_delay"
	case "LOADING_DELAY":
		return "loading_delay"
	case "UNLOADING_DELAY":
		return "unloading_delay"
	case "CARGO_ISSUE":
		return "cargo_issue"
	case "DOCUMENT_ISSUE":
		return "document_issue"
	case "CUSTOMER_UNAVAILABLE":
		return "customer_unavailable"
	case "ROUTE_BLOCKED":
		return "route_blocked"
	default:
		return "driver_exception"
	}
}

func mapDriverExceptionSeverity(category string) string {
	switch strings.TrimSpace(strings.ToUpper(category)) {
	case "ACCIDENT", "VEHICLE_BREAKDOWN", "CARGO_ISSUE":
		return "high"
	case "TRAFFIC", "LOADING_DELAY", "UNLOADING_DELAY", "ROUTE_BLOCKED", "CUSTOMER_UNAVAILABLE", "DOCUMENT_ISSUE":
		return "medium"
	default:
		return "low"
	}
}

func normalizeDriverExceptionEventID(id string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(id)), "-", "")
}
