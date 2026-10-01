package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const (
	CommandRecordDeliveryDisposition = "RECORD_DELIVERY_DISPOSITION"
	CommandAuthorizeReturn           = "AUTHORIZE_RETURN"
	CommandAuthorizeRedirect         = "AUTHORIZE_REDIRECT"
	CommandHoldDisposition           = "HOLD_DISPOSITION"
	CommandCompleteDisposition       = "COMPLETE_DISPOSITION"

	UOMPallet = "PALLET"

	DispositionReturnToOrigin = "RETURN_TO_ORIGIN"
	DispositionRedirect       = "REDIRECT"
	DispositionHold           = "HOLD_PENDING_DISPOSITION"

	DispositionStatusOpen               = "OPEN"
	DispositionStatusPending            = "DISPOSITION_PENDING"
	DispositionStatusReturnAuthorized   = "RETURN_AUTHORIZED"
	DispositionStatusRedirectAuthorized = "REDIRECT_AUTHORIZED"
	DispositionStatusInReturnTransit    = "IN_RETURN_TRANSIT"
	DispositionStatusInRedirectTransit  = "IN_REDIRECT_TRANSIT"
	DispositionStatusResolved           = "RESOLVED"
	DispositionStatusCancelled          = "CANCELLED"

	ReasonDamage               = "DAMAGE"
	ReasonMisSort              = "MIS_SORT"
	ReasonShortage             = "SHORTAGE"
	ReasonOverage              = "OVERAGE"
	ReasonPackagingDamage      = "PACKAGING_DAMAGE"
	ReasonTemperatureDeviation = "TEMPERATURE_DEVIATION"
	ReasonQualityRejection     = "QUALITY_REJECTION"
	ReasonDocumentProblem      = "DOCUMENT_PROBLEM"
	ReasonWrongProduct         = "WRONG_PRODUCT"
	ReasonExpiredProduct       = "EXPIRED_PRODUCT"
	ReasonCustomerRefusal      = "CUSTOMER_REFUSAL"
	ReasonOther                = "OTHER"

	ReasonQuantityInvalid           = "QUANTITY_INVALID"
	ReasonQuantityUnknown           = "QUANTITY_UNKNOWN"
	ReasonQuantityExceeded          = "QUANTITY_EXCEEDED"
	ReasonUOMInvalid                = "UOM_INVALID"
	ReasonCommentRequired           = "REASON_COMMENT_REQUIRED"
	ReasonCargoNotOnboard           = "CARGO_NOT_ONBOARD"
	ReasonStopNotInRevision         = "STOP_NOT_IN_REVISION"
	ReasonStopNotReady              = "STOP_NOT_READY"
	ReasonActionNotFound            = "ACTION_NOT_FOUND"
	ReasonDispositionConflict       = "DISPOSITION_CONFLICT"
	ReasonFutureStopOmitted         = "FUTURE_STOP_OMITTED"
	ReasonFutureStopUnknown         = "FUTURE_STOP_UNKNOWN"
	ReasonLocationDenied            = "LOCATION_DENIED"
	ReasonCompletionEvidenceMissing = "COMPLETION_EVIDENCE_MISSING"

	EvidencePhoto          = "PHOTO"
	EvidenceDocument       = "DOCUMENT"
	EvidenceDiscrepancyAct = "DISCREPANCY_ACT"
	EvidenceTemperature    = "TEMPERATURE"
	EvidenceDriverNote     = "DRIVER_NOTE"
	EvidenceConsigneeNote  = "CONSIGNEE_NOTE"

	EventDeliveryPartiallyRejected = "shipment.delivery.partially_rejected"
	EventDeliveryRejected          = "shipment.delivery.rejected"
	EventCargoDispositionPending   = "shipment.cargo.disposition_pending"
	EventCargoReturnAuthorized     = "shipment.cargo.return_authorized"
	EventCargoRedirectAuthorized   = "shipment.cargo.redirect_authorized"
	EventCargoReturnCompleted      = "shipment.cargo.return_completed"
	EventCargoRedirectCompleted    = "shipment.cargo.redirect_completed"

	DispositionFingerprint = "TMS_RETURN_DISPOSITION"
)

// DeliveryEvidenceRef is a metadata pointer. Binary content stays outside this table.
type DeliveryEvidenceRef struct {
	EvidenceType string
	Source       string
	ReferenceID  string
}

// RecordDeliveryDispositionCommand records accepted and rejected quantity at one stop.
// Rejected quantity stays onboard. A zero rejection does not open a disposition case.
type RecordDeliveryDispositionCommand struct {
	ExecutionID        uuid.UUID
	ExpectedRevisionID uuid.UUID
	OperatingTenantID  uuid.UUID
	SourceStopID       uuid.UUID
	ShipmentID         uuid.UUID
	CargoID            uuid.UUID
	AcceptedQuantity   int
	RejectedQuantity   int
	UOM                string
	ReasonCode         string
	ReasonComment      string
	IdempotencyKey     string
	OccurredAt         time.Time
	ActorKind          string
	ActorID            uuid.UUID
	Evidence           []DeliveryEvidenceRef
}

// AuthorizeDispositionCommand plans a return or redirect through the 0.1F successor revision.
// FutureStopIDs must be the complete set of still-open stops. InsertAt nil appends the new stop.
type AuthorizeDispositionCommand struct {
	CaseID             uuid.UUID
	ExecutionID        uuid.UUID
	ExpectedRevisionID uuid.UUID
	OperatingTenantID  uuid.UUID
	TargetLocationID   uuid.UUID
	FutureStopIDs      []uuid.UUID
	InsertAt           *int
	Instruction        string
	IdempotencyKey     string
	OccurredAt         time.Time
	ActorKind          string
	ActorID            uuid.UUID
}

// HoldDispositionCommand keeps the rejected quantity onboard without a route change.
type HoldDispositionCommand struct {
	CaseID             uuid.UUID
	ExecutionID        uuid.UUID
	ExpectedRevisionID uuid.UUID
	OperatingTenantID  uuid.UUID
	Instruction        string
	IdempotencyKey     string
	OccurredAt         time.Time
	ActorKind          string
	ActorID            uuid.UUID
}

// CompleteDispositionCommand resolves a case only when a matching execution action is already completed.
type CompleteDispositionCommand struct {
	CaseID             uuid.UUID
	ExecutionID        uuid.UUID
	ExpectedRevisionID uuid.UUID
	OperatingTenantID  uuid.UUID
	ActionID           uuid.UUID
	IdempotencyKey     string
	OccurredAt         time.Time
	ActorKind          string
	ActorID            uuid.UUID
}

// DispositionResult is the idempotent command outcome.
type DispositionResult struct {
	CommandID        uuid.UUID  `json:"command_id"`
	ExecutionID      uuid.UUID  `json:"execution_id"`
	CaseID           *uuid.UUID `json:"case_id,omitempty"`
	Status           string     `json:"status"`
	AcceptedQuantity int        `json:"accepted_quantity"`
	RejectedQuantity int        `json:"rejected_quantity"`
	RevisionID       uuid.UUID  `json:"revision_id"`
	Replayed         bool       `json:"replayed"`
}

// DispositionView is the tenant-scoped read model returned by the internal API.
type DispositionView struct {
	CaseID           uuid.UUID  `json:"caseId"`
	ExecutionID      uuid.UUID  `json:"executionId"`
	SourceStopID     uuid.UUID  `json:"sourceStopId"`
	ShipmentID       uuid.UUID  `json:"shipmentId"`
	CargoID          uuid.UUID  `json:"cargoId"`
	AcceptedQuantity int        `json:"acceptedQuantity"`
	RejectedQuantity int        `json:"rejectedQuantity"`
	UOM              string     `json:"uom"`
	ReasonCode       string     `json:"reasonCode"`
	DispositionType  string     `json:"dispositionType,omitempty"`
	Status           string     `json:"status"`
	TargetLocationID *uuid.UUID `json:"targetLocationId,omitempty"`
	Version          int        `json:"version"`
}

func KnownRejectionReason(code string) bool {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case ReasonDamage, ReasonMisSort, ReasonShortage, ReasonOverage, ReasonPackagingDamage,
		ReasonTemperatureDeviation, ReasonQualityRejection, ReasonDocumentProblem,
		ReasonWrongProduct, ReasonExpiredProduct, ReasonCustomerRefusal, ReasonOther:
		return true
	default:
		return false
	}
}

func KnownEvidenceType(kind string) bool {
	switch strings.ToUpper(strings.TrimSpace(kind)) {
	case EvidencePhoto, EvidenceDocument, EvidenceDiscrepancyAct, EvidenceTemperature, EvidenceDriverNote, EvidenceConsigneeNote:
		return true
	default:
		return false
	}
}

// ValidateDeliveryQuantities enforces accepted + rejected = attempted, with attempted > 0.
func ValidateDeliveryQuantities(accepted, rejected int) error {
	if accepted < 0 || rejected < 0 || accepted+rejected <= 0 {
		return ExecutionCommandError(ReasonQuantityInvalid, false)
	}
	return nil
}

func (c RecordDeliveryDispositionCommand) Validate() error {
	if c.ExecutionID == uuid.Nil || c.ExpectedRevisionID == uuid.Nil || c.OperatingTenantID == uuid.Nil || c.SourceStopID == uuid.Nil || c.ShipmentID == uuid.Nil || c.CargoID == uuid.Nil {
		return apperrors.Validation("delivery identity is required", map[string]any{"field": "execution_id"})
	}
	if err := validateDispositionActor(c.ActorKind, c.ActorID, true); err != nil {
		return err
	}
	if err := validateIdempotency(c.IdempotencyKey, c.OccurredAt); err != nil {
		return err
	}
	if strings.ToUpper(strings.TrimSpace(c.UOM)) != UOMPallet {
		return ExecutionCommandError(ReasonUOMInvalid, false)
	}
	if err := ValidateDeliveryQuantities(c.AcceptedQuantity, c.RejectedQuantity); err != nil {
		return err
	}
	if c.RejectedQuantity > 0 {
		reason := strings.ToUpper(strings.TrimSpace(c.ReasonCode))
		if !KnownRejectionReason(reason) {
			return ExecutionCommandError(ReasonReasonUnknown, false)
		}
		if reason == ReasonOther && strings.TrimSpace(c.ReasonComment) == "" {
			return ExecutionCommandError(ReasonCommentRequired, false)
		}
	}
	for i, item := range c.Evidence {
		if !KnownEvidenceType(item.EvidenceType) || strings.TrimSpace(item.Source) == "" {
			return apperrors.Validation("evidence is invalid", map[string]any{"field": "evidence", "index": i})
		}
		ref := strings.TrimSpace(item.ReferenceID)
		if ref == "" || len(ref) > 512 {
			return apperrors.Validation("evidence reference is invalid", map[string]any{"field": "reference_id", "index": i})
		}
	}
	return nil
}

func (c AuthorizeDispositionCommand) Validate() error {
	if c.CaseID == uuid.Nil || c.ExecutionID == uuid.Nil || c.ExpectedRevisionID == uuid.Nil || c.OperatingTenantID == uuid.Nil || c.TargetLocationID == uuid.Nil {
		return apperrors.Validation("disposition identity is required", map[string]any{"field": "case_id"})
	}
	if err := validateDispositionActor(c.ActorKind, c.ActorID, false); err != nil {
		return err
	}
	if err := validateIdempotency(c.IdempotencyKey, c.OccurredAt); err != nil {
		return err
	}
	if c.InsertAt != nil && *c.InsertAt < 0 {
		return apperrors.Validation("insert_at is invalid", map[string]any{"field": "insert_at"})
	}
	return nil
}

func (c HoldDispositionCommand) Validate() error {
	if c.CaseID == uuid.Nil || c.ExecutionID == uuid.Nil || c.ExpectedRevisionID == uuid.Nil || c.OperatingTenantID == uuid.Nil {
		return apperrors.Validation("disposition identity is required", map[string]any{"field": "case_id"})
	}
	if err := validateDispositionActor(c.ActorKind, c.ActorID, false); err != nil {
		return err
	}
	return validateIdempotency(c.IdempotencyKey, c.OccurredAt)
}

func (c CompleteDispositionCommand) Validate() error {
	if c.CaseID == uuid.Nil || c.ExecutionID == uuid.Nil || c.ExpectedRevisionID == uuid.Nil || c.OperatingTenantID == uuid.Nil || c.ActionID == uuid.Nil {
		return apperrors.Validation("completion identity is required", map[string]any{"field": "action_id"})
	}
	if err := validateDispositionActor(c.ActorKind, c.ActorID, false); err != nil {
		return err
	}
	return validateIdempotency(c.IdempotencyKey, c.OccurredAt)
}

func validateDispositionActor(kind string, actorID uuid.UUID, driverAllowed bool) error {
	switch kind {
	case ActorKindDriver:
		if !driverAllowed {
			return ExecutionCommandError(ReasonActorDenied, true)
		}
		if actorID == uuid.Nil {
			return apperrors.Validation("actor_id is required", map[string]any{"field": "actor_id"})
		}
	case ActorKindOperator:
		if actorID == uuid.Nil {
			return apperrors.Validation("actor_id is required", map[string]any{"field": "actor_id"})
		}
	case ActorKindSystem:
	default:
		return apperrors.Validation("actor_kind is invalid", map[string]any{"field": "actor_kind"})
	}
	return nil
}

func validateIdempotency(key string, occurred time.Time) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" || len(trimmed) > 128 {
		return apperrors.Validation("idempotency_key is required", map[string]any{"field": "idempotency_key"})
	}
	if occurred.IsZero() {
		return apperrors.Validation("occurred_at is required", map[string]any{"field": "occurred_at"})
	}
	return nil
}

func RecordDispositionDigest(cmd RecordDeliveryDispositionCommand) (string, error) {
	evidence := make([]evidenceDigest, len(cmd.Evidence))
	for i, item := range cmd.Evidence {
		evidence[i] = evidenceDigest{
			Type:        strings.ToUpper(strings.TrimSpace(item.EvidenceType)),
			Source:      strings.TrimSpace(item.Source),
			ReferenceID: strings.TrimSpace(item.ReferenceID),
		}
	}
	return hashDisposition(recordDigestBody{
		ExecutionID:        cmd.ExecutionID.String(),
		ExpectedRevisionID: cmd.ExpectedRevisionID.String(),
		OperatingTenantID:  cmd.OperatingTenantID.String(),
		SourceStopID:       cmd.SourceStopID.String(),
		ShipmentID:         cmd.ShipmentID.String(),
		CargoID:            cmd.CargoID.String(),
		Accepted:           cmd.AcceptedQuantity,
		Rejected:           cmd.RejectedQuantity,
		UOM:                strings.ToUpper(strings.TrimSpace(cmd.UOM)),
		Reason:             strings.ToUpper(strings.TrimSpace(cmd.ReasonCode)),
		Comment:            strings.TrimSpace(cmd.ReasonComment),
		OccurredAt:         cmd.OccurredAt.UTC().Format(time.RFC3339Nano),
		ActorKind:          cmd.ActorKind,
		ActorID:            commandUUID(cmd.ActorID),
		Evidence:           evidence,
	})
}

func AuthorizeDispositionDigest(cmd AuthorizeDispositionCommand, kind string) (string, error) {
	stops := make([]string, len(cmd.FutureStopIDs))
	for i, id := range cmd.FutureStopIDs {
		stops[i] = id.String()
	}
	insertAt := -1
	if cmd.InsertAt != nil {
		insertAt = *cmd.InsertAt
	}
	return hashDisposition(authorizeDigestBody{
		Kind:               kind,
		CaseID:             cmd.CaseID.String(),
		ExecutionID:        cmd.ExecutionID.String(),
		ExpectedRevisionID: cmd.ExpectedRevisionID.String(),
		OperatingTenantID:  cmd.OperatingTenantID.String(),
		TargetLocationID:   cmd.TargetLocationID.String(),
		FutureStopIDs:      stops,
		InsertAt:           insertAt,
		Instruction:        strings.TrimSpace(cmd.Instruction),
		OccurredAt:         cmd.OccurredAt.UTC().Format(time.RFC3339Nano),
		ActorKind:          cmd.ActorKind,
		ActorID:            commandUUID(cmd.ActorID),
	})
}

func HoldDispositionDigest(cmd HoldDispositionCommand) (string, error) {
	return hashDisposition(holdDigestBody{
		CaseID:             cmd.CaseID.String(),
		ExecutionID:        cmd.ExecutionID.String(),
		ExpectedRevisionID: cmd.ExpectedRevisionID.String(),
		OperatingTenantID:  cmd.OperatingTenantID.String(),
		Instruction:        strings.TrimSpace(cmd.Instruction),
		OccurredAt:         cmd.OccurredAt.UTC().Format(time.RFC3339Nano),
		ActorKind:          cmd.ActorKind,
		ActorID:            commandUUID(cmd.ActorID),
	})
}

func CompleteDispositionDigest(cmd CompleteDispositionCommand) (string, error) {
	return hashDisposition(completeDigestBody{
		CaseID:             cmd.CaseID.String(),
		ExecutionID:        cmd.ExecutionID.String(),
		ExpectedRevisionID: cmd.ExpectedRevisionID.String(),
		OperatingTenantID:  cmd.OperatingTenantID.String(),
		ActionID:           cmd.ActionID.String(),
		OccurredAt:         cmd.OccurredAt.UTC().Format(time.RFC3339Nano),
		ActorKind:          cmd.ActorKind,
		ActorID:            commandUUID(cmd.ActorID),
	})
}

type evidenceDigest struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	ReferenceID string `json:"reference_id"`
}

type recordDigestBody struct {
	ExecutionID        string           `json:"execution_id"`
	ExpectedRevisionID string           `json:"expected_revision_id"`
	OperatingTenantID  string           `json:"operating_tenant_id"`
	SourceStopID       string           `json:"source_stop_id"`
	ShipmentID         string           `json:"shipment_id"`
	CargoID            string           `json:"cargo_id"`
	Accepted           int              `json:"accepted"`
	Rejected           int              `json:"rejected"`
	UOM                string           `json:"uom"`
	Reason             string           `json:"reason"`
	Comment            string           `json:"comment"`
	OccurredAt         string           `json:"occurred_at"`
	ActorKind          string           `json:"actor_kind"`
	ActorID            string           `json:"actor_id"`
	Evidence           []evidenceDigest `json:"evidence"`
}

type authorizeDigestBody struct {
	Kind               string   `json:"kind"`
	CaseID             string   `json:"case_id"`
	ExecutionID        string   `json:"execution_id"`
	ExpectedRevisionID string   `json:"expected_revision_id"`
	OperatingTenantID  string   `json:"operating_tenant_id"`
	TargetLocationID   string   `json:"target_location_id"`
	FutureStopIDs      []string `json:"future_stop_ids"`
	InsertAt           int      `json:"insert_at"`
	Instruction        string   `json:"instruction"`
	OccurredAt         string   `json:"occurred_at"`
	ActorKind          string   `json:"actor_kind"`
	ActorID            string   `json:"actor_id"`
}

type holdDigestBody struct {
	CaseID             string `json:"case_id"`
	ExecutionID        string `json:"execution_id"`
	ExpectedRevisionID string `json:"expected_revision_id"`
	OperatingTenantID  string `json:"operating_tenant_id"`
	Instruction        string `json:"instruction"`
	OccurredAt         string `json:"occurred_at"`
	ActorKind          string `json:"actor_kind"`
	ActorID            string `json:"actor_id"`
}

type completeDigestBody struct {
	CaseID             string `json:"case_id"`
	ExecutionID        string `json:"execution_id"`
	ExpectedRevisionID string `json:"expected_revision_id"`
	OperatingTenantID  string `json:"operating_tenant_id"`
	ActionID           string `json:"action_id"`
	OccurredAt         string `json:"occurred_at"`
	ActorKind          string `json:"actor_kind"`
	ActorID            string `json:"actor_id"`
}

func hashDisposition(body any) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
