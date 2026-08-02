package rebuild

import (
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/statussnapshot"
)

const (
	StateImporting   = "IMPORTING"
	StateValidated   = "VALIDATED"
	StateActivating  = "ACTIVATING"
	StateActive      = "ACTIVE"
	StateRollingBack = "ROLLING_BACK"
	StateRolledBack  = "ROLLED_BACK"
	StateFailed      = "FAILED"
	StateCancelled   = "CANCELLED"
	StateCleaned     = "CLEANED"

	ProjectionSourceLiveEvent             = "LIVE_EVENT"
	ProjectionSourceAuthoritativeSnapshot = "AUTHORITATIVE_SNAPSHOT"

	SupportedRebuildSchemaVersion = 1
)

type Manifest struct {
	SnapshotID    uuid.UUID
	SchemaVersion int
	Scope         statussnapshot.Scope
	TenantID      *uuid.UUID
	StartedAt     time.Time
}

type StageRow struct {
	SnapshotID        uuid.UUID
	TenantID          uuid.UUID
	ShipmentID        uuid.UUID
	CurrentStatus     string
	PreviousStatus    *string
	AggregateVersion  int64
	LastEventID       *uuid.UUID
	LastSourceEventID *uuid.UUID
	LastEventType     *string
	SourceUpdatedAt   time.Time
	RecordSequence    int64
}

type ValidationResult struct {
	SnapshotID     uuid.UUID
	ExpectedRows   int64
	TenantCount    int64
	ExpectedSHA256 string
	ActualSHA256   string
}

type JobStatus struct {
	SnapshotID       uuid.UUID
	State            string
	Scope            string
	ExpectedRows     *int64
	ImportedRows     int64
	TenantCount      *int64
	ActivatedRows    *int64
	BackupRows       *int64
	RollbackEligible *bool
	ChecksumMatched  bool
	StartedAt        time.Time
	ValidatedAt      *time.Time
	ActivatedAt      *time.Time
	RolledBackAt     *time.Time
	ErrorCode        *string
}

type ActivationResult struct {
	State            string
	Scope            string
	ActivatedRows    int64
	BackupRows       int64
	RollbackEligible bool
	ActivatedAt      time.Time
}

type RollbackResult struct {
	State        string
	RestoredRows int64
	RolledBackAt time.Time
}

type CleanupResult struct {
	State             string
	StageRowsRemoved  int64
	BackupRowsRemoved int64
}

type RollbackEligibility struct {
	Eligible bool
	Reason   string
}

type StatusResponse struct {
	State            string  `json:"state"`
	Scope            string  `json:"scope"`
	ExpectedRows     *int64  `json:"expectedRows"`
	ImportedRows     int64   `json:"importedRows"`
	TenantCount      *int64  `json:"tenantCount"`
	ActivatedRows    *int64  `json:"activatedRows,omitempty"`
	BackupRows       *int64  `json:"backupRows,omitempty"`
	RollbackEligible *bool   `json:"rollbackEligible,omitempty"`
	ChecksumMatched  bool    `json:"checksumMatched"`
	StartedAt        string  `json:"startedAt"`
	ValidatedAt      *string `json:"validatedAt,omitempty"`
	ActivatedAt      *string `json:"activatedAt,omitempty"`
	RolledBackAt     *string `json:"rolledBackAt,omitempty"`
	ErrorCode        *string `json:"errorCode,omitempty"`
}

type DryRunReport struct {
	SchemaVersion    int                  `json:"schemaVersion"`
	Scope            statussnapshot.Scope `json:"scope"`
	RowCount         int64                `json:"rowCount"`
	TenantCount      int64                `json:"tenantCount"`
	ChecksumMatched  bool                 `json:"checksumMatched"`
	ValidationResult string               `json:"validationResult"`
}
