package statussnapshot

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

const SchemaVersionV1 = 1

const (
	RecordTypeManifest   = "manifest"
	RecordTypeShipment   = "shipment"
	RecordTypeCompletion = "complete"
)

const (
	SourceShipmentService = "SHIPMENT_SERVICE"
	IsolationRepeatableRead = "REPEATABLE_READ"
)

type Scope string

const (
	ScopeAll    Scope = "ALL"
	ScopeTenant Scope = "TENANT"
)

const DefaultMaxLineBytes = 1 << 20

var KnownShipmentStatuses = map[string]struct{}{
	"CARRIER_ASSIGNED": {}, "ACCEPTED_BY_CARRIER": {}, "VEHICLE_ASSIGNED": {},
	"DRIVER_ASSIGNED": {}, "PICKUP_SLOT_BOOKED": {}, "DELIVERY_SLOT_BOOKED": {},
	"IN_PICKUP": {}, "LOADED": {}, "IN_TRANSIT": {}, "ARRIVED_AT_CONSIGNEE": {},
	"UNLOADING": {}, "DELIVERED": {}, "DELIVERY_CONFIRMED": {},
	"DOCUMENTS_COMPLETED": {}, "READY_FOR_BILLING": {}, "INCLUDED_IN_BILLING_REGISTER": {},
	"FINANCIALLY_CLOSED": {}, "CANCELLED": {},
}

var KnownEventTypes = map[string]struct{}{
	"shipment.created": {}, "shipment.status.changed": {}, "shipment.cancelled": {},
	"shipment.ready_for_billing": {}, "shipment.documents_completed": {}, "shipment.financially_closed": {},
}

func IsKnownEventType(eventType string) bool {
	_, ok := KnownEventTypes[strings.TrimSpace(eventType)]
	return ok
}

type ManifestRecord struct {
	RecordType           string     `json:"recordType"`
	SchemaVersion        int        `json:"schemaVersion"`
	SnapshotID           uuid.UUID  `json:"snapshotId"`
	Scope                Scope      `json:"scope"`
	TenantID             *uuid.UUID `json:"tenantId,omitempty"`
	Ordering             Ordering   `json:"ordering"`
	StartedAt            time.Time  `json:"startedAt"`
	TransactionIsolation string     `json:"transactionIsolation"`
	Source               string     `json:"source"`
}

type ShipmentRecord struct {
	RecordType        string     `json:"recordType"`
	SchemaVersion     int        `json:"schemaVersion"`
	SnapshotID        uuid.UUID  `json:"snapshotId"`
	TenantID          uuid.UUID  `json:"tenantId"`
	ShipmentID        uuid.UUID  `json:"shipmentId"`
	CurrentStatus     string     `json:"currentStatus"`
	PreviousStatus    *string    `json:"previousStatus,omitempty"`
	AggregateVersion  int64      `json:"aggregateVersion"`
	LastEventID       *uuid.UUID `json:"lastEventId,omitempty"`
	LastSourceEventID *uuid.UUID `json:"lastSourceEventId,omitempty"`
	LastEventType     *string    `json:"lastEventType,omitempty"`
	SourceUpdatedAt   time.Time  `json:"sourceUpdatedAt"`
}

type CompletionRecord struct {
	RecordType    string    `json:"recordType"`
	SchemaVersion int       `json:"schemaVersion"`
	SnapshotID    uuid.UUID `json:"snapshotId"`
	RowCount      int64     `json:"rowCount"`
	TenantCount   int64     `json:"tenantCount"`
	SHA256        string    `json:"sha256"`
	CompletedAt   time.Time `json:"completedAt"`
}

type Record interface {
	recordType() string
}

func (ManifestRecord) recordType() string   { return RecordTypeManifest }
func (ShipmentRecord) recordType() string   { return RecordTypeShipment }
func (CompletionRecord) recordType() string { return RecordTypeCompletion }

func MarshalNDJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	return raw, nil
}

func decodeTypedRecord(line []byte) (Record, error) {
	var header struct {
		RecordType string `json:"recordType"`
	}
	if err := json.Unmarshal(line, &header); err != nil {
		return nil, &ValidationError{Code: CodeInvalidJSON, Err: err}
	}
	switch header.RecordType {
	case RecordTypeManifest:
		var rec ManifestRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, &ValidationError{Code: CodeInvalidJSON, Err: err}
		}
		return rec, nil
	case RecordTypeShipment:
		var rec ShipmentRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, &ValidationError{Code: CodeInvalidJSON, Err: err}
		}
		return rec, nil
	case RecordTypeCompletion:
		var rec CompletionRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, &ValidationError{Code: CodeInvalidJSON, Err: err}
		}
		return rec, nil
	default:
		return nil, &ValidationError{Code: CodeUnknownRecordType}
	}
}

func IsKnownStatus(status string) bool {
	_, ok := KnownShipmentStatuses[strings.TrimSpace(status)]
	return ok
}

func validateUUID(id uuid.UUID, code string) error {
	if id == uuid.Nil {
		return &ValidationError{Code: code}
	}
	return nil
}

func ValidateManifest(rec ManifestRecord) error {
	if rec.SchemaVersion != SchemaVersionV1 {
		return &ValidationError{Code: CodeUnsupportedSchemaVersion}
	}
	if err := validateUUID(rec.SnapshotID, CodeInvalidUUID); err != nil {
		return err
	}
	if rec.Scope != ScopeAll && rec.Scope != ScopeTenant {
		return &ValidationError{Code: CodeInvalidScope}
	}
	if rec.Ordering != OrderingTenantIDShipmentID {
		return &ValidationError{Code: CodeInvalidOrdering}
	}
	if rec.Source != SourceShipmentService {
		return &ValidationError{Code: CodeInvalidScope}
	}
	if rec.TransactionIsolation != IsolationRepeatableRead {
		return &ValidationError{Code: CodeInvalidScope}
	}
	if rec.StartedAt.IsZero() {
		return &ValidationError{Code: CodeInvalidTimestamp}
	}
	switch rec.Scope {
	case ScopeAll:
		if rec.TenantID != nil {
			return &ValidationError{Code: CodeInvalidScope}
		}
	case ScopeTenant:
		if rec.TenantID == nil {
			return &ValidationError{Code: CodeInvalidScope}
		}
		if err := validateUUID(*rec.TenantID, CodeInvalidUUID); err != nil {
			return err
		}
	}
	return nil
}

func ValidateShipment(rec ShipmentRecord, manifest ManifestRecord) error {
	if rec.SchemaVersion != SchemaVersionV1 {
		return &ValidationError{Code: CodeUnsupportedSchemaVersion}
	}
	if rec.SnapshotID != manifest.SnapshotID {
		return &ValidationError{Code: CodeSnapshotIDMismatch}
	}
	if err := validateUUID(rec.SnapshotID, CodeInvalidUUID); err != nil {
		return err
	}
	if err := validateUUID(rec.TenantID, CodeInvalidUUID); err != nil {
		return err
	}
	if err := validateUUID(rec.ShipmentID, CodeInvalidUUID); err != nil {
		return err
	}
	if !IsKnownStatus(rec.CurrentStatus) {
		return &ValidationError{Code: CodeUnknownStatus}
	}
	if rec.PreviousStatus != nil && !IsKnownStatus(*rec.PreviousStatus) {
		return &ValidationError{Code: CodeUnknownStatus}
	}
	if rec.AggregateVersion < 1 {
		return &ValidationError{Code: CodeInvalidAggregateVersion}
	}
	if rec.SourceUpdatedAt.IsZero() {
		return &ValidationError{Code: CodeInvalidTimestamp}
	}
	if rec.LastEventID != nil && rec.LastSourceEventID == nil {
		return &ValidationError{Code: CodeInconsistentMetadata}
	}
	if rec.LastEventID != nil {
		if rec.LastEventType == nil || !IsKnownEventType(*rec.LastEventType) {
			return &ValidationError{Code: CodeUnknownEventType}
		}
	}
	if rec.LastEventType != nil && !IsKnownEventType(*rec.LastEventType) {
		return &ValidationError{Code: CodeUnknownEventType}
	}
	if manifest.Scope == ScopeTenant {
		if manifest.TenantID == nil || rec.TenantID != *manifest.TenantID {
			return &ValidationError{Code: CodeTenantScopeMismatch}
		}
	}
	return nil
}

func ValidateCompletion(rec CompletionRecord, manifest ManifestRecord, stats StreamStats) error {
	if rec.SchemaVersion != SchemaVersionV1 {
		return &ValidationError{Code: CodeUnsupportedSchemaVersion}
	}
	if rec.SnapshotID != manifest.SnapshotID {
		return &ValidationError{Code: CodeSnapshotIDMismatch}
	}
	if rec.RowCount < 0 || rec.TenantCount < 0 {
		return &ValidationError{Code: CodeRowCountMismatch}
	}
	if rec.RowCount > 0 && rec.TenantCount > rec.RowCount {
		return &ValidationError{Code: CodeTenantCountMismatch}
	}
	if rec.RowCount != stats.RowCount {
		return &ValidationError{Code: CodeRowCountMismatch}
	}
	if rec.TenantCount != stats.TenantCount {
		return &ValidationError{Code: CodeTenantCountMismatch}
	}
	if len(rec.SHA256) != 64 {
		return &ValidationError{Code: CodeChecksumMismatch}
	}
	for _, ch := range rec.SHA256 {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return &ValidationError{Code: CodeChecksumMismatch}
		}
	}
	if stats.ChecksumHex != rec.SHA256 {
		return &ValidationError{Code: CodeChecksumMismatch}
	}
	if rec.CompletedAt.Before(manifest.StartedAt) {
		return &ValidationError{Code: CodeInvalidTimestamp}
	}
	return nil
}

type StreamStats struct {
	RowCount    int64
	TenantCount int64
	ChecksumHex string
}
