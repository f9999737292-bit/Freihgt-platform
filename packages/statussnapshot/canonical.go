package statussnapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"io"
)

// EmptyStreamChecksumSHA256 is SHA-256 over zero canonical shipment lines.
const EmptyStreamChecksumSHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// canonicalShipmentPayload is the fixed field order used for checksum encoding.
type canonicalShipmentPayload struct {
	RecordType        string  `json:"recordType"`
	SchemaVersion     int     `json:"schemaVersion"`
	SnapshotID        string  `json:"snapshotId"`
	TenantID          string  `json:"tenantId"`
	ShipmentID        string  `json:"shipmentId"`
	CurrentStatus     string  `json:"currentStatus"`
	PreviousStatus    *string `json:"previousStatus,omitempty"`
	AggregateVersion  int64   `json:"aggregateVersion"`
	LastEventID       *string `json:"lastEventId,omitempty"`
	LastSourceEventID *string `json:"lastSourceEventId,omitempty"`
	SourceUpdatedAt   string  `json:"sourceUpdatedAt"`
}

func toCanonicalPayload(rec ShipmentRecord) canonicalShipmentPayload {
	payload := canonicalShipmentPayload{
		RecordType:       RecordTypeShipment,
		SchemaVersion:    rec.SchemaVersion,
		SnapshotID:       rec.SnapshotID.String(),
		TenantID:         rec.TenantID.String(),
		ShipmentID:       rec.ShipmentID.String(),
		CurrentStatus:    rec.CurrentStatus,
		PreviousStatus:   rec.PreviousStatus,
		AggregateVersion: rec.AggregateVersion,
		SourceUpdatedAt:  rec.SourceUpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if rec.LastEventID != nil {
		s := rec.LastEventID.String()
		payload.LastEventID = &s
	}
	if rec.LastSourceEventID != nil {
		s := rec.LastSourceEventID.String()
		payload.LastSourceEventID = &s
	}
	return payload
}

func EncodeCanonicalShipment(w io.Writer, record ShipmentRecord) error {
	raw, err := json.Marshal(toCanonicalPayload(record))
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	_, err = w.Write(raw)
	return err
}

type Checksummer struct {
	h hash.Hash
}

func NewChecksummer() *Checksummer {
	return &Checksummer{h: sha256.New()}
}

func (c *Checksummer) AddCanonicalShipment(record ShipmentRecord) error {
	return EncodeCanonicalShipment(c.h, record)
}

func (c *Checksummer) SumHex() string {
	return hex.EncodeToString(c.h.Sum(nil))
}
