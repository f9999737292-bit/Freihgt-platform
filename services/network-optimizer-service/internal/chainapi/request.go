package chainapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/google/uuid"
)

const (
	PublicSeedManualCapacity    = "MANUAL_CAPACITY"
	PublicSeedCurrentTrip       = "CURRENT_TRIP"
	PublicSeedPredictedCapacity = "PREDICTED_CAPACITY"

	maxIdempotencyKey = 128
)

// publicSeed is the only caller-supplied chain input.
type publicSeed struct {
	SeedType   string
	CapacityID *uuid.UUID
	ShipmentID *uuid.UUID
}

func parsePublicSeed(raw []byte) (publicSeed, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var body map[string]json.RawMessage
	if err := dec.Decode(&body); err != nil {
		return publicSeed{}, badRequest("request body is invalid")
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return publicSeed{}, badRequest("request body is invalid")
	}
	if body == nil {
		return publicSeed{}, badRequest("request body is invalid")
	}
	for key := range body {
		switch key {
		case "seed_type", "capacity_id", "shipment_id":
		case "candidate_load_ids", "l1", "L1", "l2", "L2", "chain_search_id", "chain_candidate_id":
			return publicSeed{}, unprocessable("client-selected chain pair is not accepted", map[string]any{"field": key})
		case "predicted_capacity_id", "predicted_capacity":
			return publicSeed{}, unprocessable("predicted capacity is not a chain seed", map[string]any{"field": key})
		default:
			return publicSeed{}, badRequest("unsupported request field")
		}
	}
	seedType, err := rawString(body["seed_type"])
	if err != nil {
		return publicSeed{}, badRequest("seed_type is invalid")
	}
	seedType = strings.TrimSpace(seedType)
	if seedType == PublicSeedPredictedCapacity {
		return publicSeed{}, unprocessable("predicted capacity is not a chain seed", map[string]any{"field": "seed_type"})
	}
	capacityID, err := rawUUID(body, "capacity_id")
	if err != nil {
		return publicSeed{}, err
	}
	shipmentID, err := rawUUID(body, "shipment_id")
	if err != nil {
		return publicSeed{}, err
	}
	switch seedType {
	case PublicSeedManualCapacity:
		if capacityID == nil || shipmentID != nil {
			return publicSeed{}, badRequest("MANUAL_CAPACITY requires capacity_id only")
		}
	case PublicSeedCurrentTrip:
		if shipmentID == nil || capacityID != nil {
			return publicSeed{}, badRequest("CURRENT_TRIP requires shipment_id only")
		}
	default:
		return publicSeed{}, badRequest("seed_type is not supported")
	}
	return publicSeed{SeedType: seedType, CapacityID: capacityID, ShipmentID: shipmentID}, nil
}

func parseIdempotencyKey(raw string, supplied bool) (string, bool, error) {
	if !supplied {
		return "server:" + uuid.NewString(), false, nil
	}
	key := strings.TrimSpace(raw)
	if key == "" || len(key) > maxIdempotencyKey {
		return "", false, badRequest("Idempotency-Key must be 1..128 characters")
	}
	return key, true, nil
}

func rawString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", badRequest("seed_type is required")
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return value, nil
}

func rawUUID(body map[string]json.RawMessage, field string) (*uuid.UUID, error) {
	raw, ok := body[field]
	if !ok {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil, badRequest(field + " must be a uuid")
	}
	id, err := uuid.Parse(strings.TrimSpace(text))
	if err != nil || id == uuid.Nil {
		return nil, badRequest(field + " must be a uuid")
	}
	return &id, nil
}
