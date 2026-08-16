package controltowerreadmodel

import "time"

const (
	assignCriticalEventPath  = "/internal/v1/control-tower/critical-events/%s/assign"
	resolveCriticalEventPath = "/internal/v1/control-tower/critical-events/%s/resolve"
	reopenCriticalEventPath  = "/internal/v1/control-tower/critical-events/%s/reopen"
	listCriticalEventActions = "/internal/v1/control-tower/critical-events/%s/actions"
	lookupWorkflowsPath      = "/internal/v1/control-tower/critical-events/workflows/lookup"
)

type AssignCriticalEventInput struct {
	TenantID       string
	UserID         string
	RequestID      string
	EventID        string
	AssignedToUser string
}

type ResolveCriticalEventInput struct {
	TenantID          string
	UserID            string
	RequestID         string
	EventID           string
	ResolutionCode    string
	ResolutionComment *string
}

type ReopenCriticalEventInput struct {
	TenantID  string
	UserID    string
	RequestID string
	EventID   string
}

type RemoteWorkflowSummary struct {
	Acknowledgement *RemoteWorkflowAcknowledgement `json:"acknowledgement,omitempty"`
	Assignment      *RemoteWorkflowAssignment      `json:"assignment,omitempty"`
	Resolution      *RemoteWorkflowResolution      `json:"resolution,omitempty"`
}

type RemoteWorkflowAcknowledgement struct {
	AcknowledgedAt string `json:"acknowledgedAt"`
	UserID         string `json:"userId"`
}

type RemoteWorkflowAssignment struct {
	AssignedToUserID string `json:"assignedToUserId"`
	AssignedByUserID string `json:"assignedByUserId"`
	AssignedAt       string `json:"assignedAt"`
}

type RemoteWorkflowResolution struct {
	ResolvedByUserID string  `json:"resolvedByUserId"`
	ResolvedAt       string  `json:"resolvedAt"`
	ResolutionCode   string  `json:"resolutionCode"`
	Comment          *string `json:"comment,omitempty"`
}

type RemoteWorkflow struct {
	EventID string `json:"eventId"`
	Status  string `json:"status"`
	RemoteWorkflowSummary
	Exception RemoteExceptionDetails `json:"exception,omitempty"`
}

type RemoteWorkflowAction struct {
	ActionType  string         `json:"actionType"`
	ActorUserID string         `json:"actorUserId"`
	OccurredAt  string         `json:"occurredAt"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type RemoteWorkflowLookupItem struct {
	EventID string
	Status  string
	RemoteWorkflowSummary
	Exception            RemoteExceptionDetails
	AcknowledgedAt       time.Time
	AcknowledgedByUserID string
}
