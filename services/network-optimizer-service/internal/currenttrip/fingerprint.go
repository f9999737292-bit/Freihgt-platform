package currenttrip

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type fingerprintDocument struct {
	ShipmentID            uuid.UUID        `json:"shipment_id"`
	ShipmentVersion       int              `json:"shipment_version"`
	ShipmentStatus        string           `json:"shipment_status"`
	VehicleID             *uuid.UUID       `json:"vehicle_id"`
	VehicleVersion        int              `json:"vehicle_version"`
	Evidence              []fingerprintRow `json:"evidence"`
	CargoVersions         []int            `json:"cargo_versions"`
	PositionFreshness     string           `json:"position_freshness"`
	PositionRecordedAt    *time.Time       `json:"position_recorded_at"`
	ETAStatus             string           `json:"eta_status"`
	ETAFreshness          string           `json:"eta_freshness"`
	ETAObservedAt         *time.Time       `json:"eta_observed_at"`
	ResidualPayload       string           `json:"residual_payload"`
	ResidualVolume        string           `json:"residual_volume"`
	ResidualPallets       string           `json:"residual_pallets"`
	ResidualLinear        string           `json:"residual_linear"`
	ResidualHeight        string           `json:"residual_height"`
	TemperatureAllocation string           `json:"temperature_allocation"`
}

type fingerprintRow struct {
	CargoID      uuid.UUID `json:"cargo_id"`
	State        string    `json:"state"`
	StateVersion int       `json:"state_version"`
	OccurredAt   time.Time `json:"occurred_at"`
	CargoVersion int       `json:"cargo_version"`
}

func inputFingerprint(ctx CurrentTripContext) string {
	doc := fingerprintDocument{
		ShipmentID: ctx.ShipmentID, ShipmentVersion: ctx.ShipmentVersion, ShipmentStatus: ctx.ShipmentStatus,
		VehicleID: ctx.VehicleID, VehicleVersion: ctx.VehicleVersion,
		PositionFreshness: ctx.PositionFreshnessStatus, PositionRecordedAt: ctx.PositionObservedAt,
		ETAFreshness: ctx.ETAFreshnessStatus, ETAObservedAt: ctx.ETAObservedAt,
		ResidualPayload:       ctx.ResidualCapacity.Payload.Status,
		ResidualVolume:        ctx.ResidualCapacity.Volume.Status,
		ResidualPallets:       ctx.ResidualCapacity.PalletPositions.Status,
		ResidualLinear:        ctx.ResidualCapacity.LinearMeters.Status,
		ResidualHeight:        ctx.ResidualCapacity.Height.Status,
		TemperatureAllocation: ctx.ResidualCapacity.TemperatureAllocation,
	}
	if ctx.ETA != nil {
		doc.ETAStatus = ctx.ETA.Status
	}
	for _, unit := range ctx.OnboardCargoUnits {
		doc.Evidence = append(doc.Evidence, fingerprintRow{
			CargoID: unit.CargoID, State: unit.EvidenceState, StateVersion: unit.EvidenceStateVersion,
			OccurredAt: unit.EvidenceOccurredAt.UTC(), CargoVersion: unit.CargoVersion,
		})
		doc.CargoVersions = append(doc.CargoVersions, unit.CargoVersion)
	}
	if doc.Evidence == nil {
		doc.Evidence = []fingerprintRow{}
		doc.CargoVersions = []int{}
	}
	raw, _ := json.Marshal(doc)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
