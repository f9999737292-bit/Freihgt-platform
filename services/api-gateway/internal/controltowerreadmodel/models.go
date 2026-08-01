package controltowerreadmodel

import "time"

const (
	SourceLegacy    = "LEGACY"
	SourceReadModel = "READ_MODEL"

	WarningUnavailable        = "CONTROL_TOWER_READ_MODEL_UNAVAILABLE"
	WarningConsumerNotRunning = "CONTROL_TOWER_READ_MODEL_CONSUMER_NOT_RUNNING"
	WarningPartial            = "CONTROL_TOWER_READ_MODEL_PARTIAL"
	WarningFallbackUsed       = "CONTROL_TOWER_READ_MODEL_FALLBACK_USED"
	WarningLegacyLimited      = "CONTROL_TOWER_LEGACY_STATUS_SUMMARY_LIMITED"
)

type StatusSummary struct {
	TotalShipments        int64            `json:"totalShipments"`
	CountedShipments      int64            `json:"countedShipments,omitempty"`
	ByStatus              map[string]int64 `json:"byStatus"`
	IncompleteProjections int64            `json:"incompleteProjections"`
	Source                string           `json:"source"`
	LimitedDataset        bool             `json:"limitedDataset,omitempty"`
}

type FreshnessSnapshot struct {
	ConsumerRunning         bool       `json:"consumerRunning"`
	LastRecordReceivedAt    *time.Time `json:"lastRecordReceivedAt,omitempty"`
	LastProjectionAppliedAt *time.Time `json:"lastProjectionAppliedAt,omitempty"`
}

type StatusSummaryFreshness struct {
	Loaded                  bool       `json:"loaded"`
	FallbackUsed            bool       `json:"fallbackUsed"`
	Partial                 bool       `json:"partial"`
	Source                  string     `json:"source,omitempty"`
	ConsumerRunning         *bool      `json:"consumerRunning,omitempty"`
	LastRecordReceivedAt    *time.Time `json:"lastRecordReceivedAt,omitempty"`
	LastProjectionAppliedAt *time.Time `json:"lastProjectionAppliedAt,omitempty"`
}

type RemoteStatusSummary struct {
	TotalShipments            int64             `json:"totalShipments"`
	ByStatus                  map[string]int64  `json:"byStatus"`
	IncompleteProjections     int64             `json:"incompleteProjections"`
	OldestProjectionUpdatedAt *string           `json:"oldestProjectionUpdatedAt,omitempty"`
	LatestProjectionUpdatedAt *string           `json:"latestProjectionUpdatedAt,omitempty"`
	Freshness                 FreshnessSnapshot `json:"freshness"`
}

type LegacyStatusInput struct {
	TotalShipments   int64
	CountedShipments int64
	ByStatus         map[string]int64
	LimitedDataset   bool
}

type MergeInput struct {
	Mode                   Mode
	Legacy                 LegacyStatusInput
	ReadModel              *RemoteStatusSummary
	ReadModelErr           *DependencyError
	RequireConsumerRunning bool
}

type MergeOutput struct {
	StatusSummary          *StatusSummary
	StatusSummaryFreshness *StatusSummaryFreshness
	Warnings               []string
	Comparison             ComparisonResult
	FailureReason          FailureReason
}
