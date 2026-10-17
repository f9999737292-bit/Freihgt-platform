package domain

import (
	"time"

	"github.com/google/uuid"
)

const PODActionTypeDelivery = "DELIVERY"

// DeliveryActionSubject is the TMS delivery action the trusted shipment-service command names.
type DeliveryActionSubject struct {
	ActionType        string
	OperatingTenantID uuid.UUID
	ExecutionID       uuid.UUID
	ExecutionActionID uuid.UUID
	ExecutionStopID   uuid.UUID
	ShipmentID        uuid.UUID
	ShipmentTenantID  uuid.UUID
	CargoID           uuid.UUID
	CarrierCompanyID  uuid.UUID
}

// PODActionIntent is the durable action-bound upload intent. Pins are fixed at insert.
type PODActionIntent struct {
	ID                uuid.UUID
	DocumentTenantID  uuid.UUID
	OperatingTenantID uuid.UUID
	ExecutionID       uuid.UUID
	ExecutionActionID uuid.UUID
	ExecutionStopID   uuid.UUID
	ShipmentID        uuid.UUID
	ShipmentTenantID  uuid.UUID
	CargoID           uuid.UUID
	OwnerCompanyID    uuid.UUID
	AuthorDriverID    uuid.UUID
	AuthorCompanyID   uuid.UUID
	DocumentID        uuid.UUID
	DocumentVersionID uuid.UUID
	ObjectKey         string
	UploadTokenHash   string
	MimeType          string
	MaxBytes          int64
	FileName          string
	Status            string
	IdempotencyKey    string
	ChecksumSHA256    string
	ByteSize          int64
	ExpiresAt         time.Time
	CompletedAt       *time.Time
	CreatedAt         time.Time
}

// PODActionEvidence is one immutable binding of a completed POD file to one delivery action.
type PODActionEvidence struct {
	ID                uuid.UUID `json:"id"`
	UploadIntentID    uuid.UUID `json:"-"`
	DocumentTenantID  uuid.UUID `json:"documentTenantId"`
	OperatingTenantID uuid.UUID `json:"operatingTenantId"`
	ExecutionID       uuid.UUID `json:"executionId"`
	ExecutionActionID uuid.UUID `json:"executionActionId"`
	ExecutionStopID   uuid.UUID `json:"executionStopId"`
	ShipmentID        uuid.UUID `json:"shipmentId"`
	ShipmentTenantID  uuid.UUID `json:"shipmentTenantId"`
	CargoID           uuid.UUID `json:"cargoId"`
	OwnerCompanyID    uuid.UUID `json:"ownerCompanyId"`
	AuthorDriverID    uuid.UUID `json:"authorDriverId"`
	AuthorCompanyID   uuid.UUID `json:"authorCompanyId"`
	DocumentID        uuid.UUID `json:"documentId"`
	DocumentVersionID uuid.UUID `json:"documentVersionId"`
	DocumentFileID    uuid.UUID `json:"documentFileId"`
	IdempotencyKey    string    `json:"idempotencyKey"`
	CreatedAt         time.Time `json:"createdAt"`
}

// SubjectConfirmation carries optional subject fields supplied again on content or complete.
// A present value that differs from the stored pin is a conflict. Absent values are ignored.
type SubjectConfirmation struct {
	DocumentTenantID  *uuid.UUID
	OperatingTenantID *uuid.UUID
	ExecutionID       *uuid.UUID
	ExecutionActionID *uuid.UUID
	ExecutionStopID   *uuid.UUID
	ShipmentID        *uuid.UUID
	ShipmentTenantID  *uuid.UUID
	CargoID           *uuid.UUID
	OwnerCompanyID    *uuid.UUID
	AuthorDriverID    *uuid.UUID
	AuthorCompanyID   *uuid.UUID
}
