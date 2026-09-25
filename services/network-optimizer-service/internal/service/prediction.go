package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
	apperrors "github.com/freight-platform/network-optimizer-service/internal/platform/errors"
	bnometrics "github.com/freight-platform/network-optimizer-service/internal/platform/metrics"
	"github.com/freight-platform/network-optimizer-service/internal/predict"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

func (s *Service) GeneratePrediction(ctx context.Context, actor Actor, shipmentID uuid.UUID, key, hash string) (Result, error) {
	return s.recompute(ctx, actor, shipmentID, uuid.Nil, 0, false, key, hash)
}

func (s *Service) RefreshPrediction(ctx context.Context, actor Actor, predictionID uuid.UUID, version int, key, hash string) (Result, error) {
	current, err := s.currentPrediction(ctx, actor.TenantID, predictionID)
	if err != nil {
		return Result{}, err
	}
	if current.Version != version {
		return Result{}, apperrors.Conflict("prediction version conflict", map[string]any{"reason": domain.ReasonPredictionSuperseded})
	}
	return s.recompute(ctx, actor, current.ShipmentID, predictionID, version, true, key, hash)
}

func (s *Service) GetPrediction(ctx context.Context, actor Actor, id uuid.UUID) (Result, error) {
	var body []byte
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		prediction, capacity, err := s.ownPrediction(ctx, tx, actor.TenantID, id)
		if err != nil {
			return err
		}
		body, err = marshalPrediction(prediction, capacity)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Status: 200, Body: body, AggregateID: id}, nil
}

func (s *Service) ListPredictions(ctx context.Context, actor Actor, limit, offset int) (Result, error) {
	limit, offset, err := normalizePage(limit, offset)
	if err != nil {
		return Result{}, err
	}
	var items []json.RawMessage
	err = s.store.Within(ctx, func(tx repository.Tx) error {
		rows, err := tx.ListOwnPredictions(ctx, actor.TenantID, limit, offset)
		if err != nil {
			return err
		}
		items = make([]json.RawMessage, 0, len(rows))
		for _, row := range rows {
			capacity, err := tx.GetCapacity(ctx, row.CapacityID)
			if err != nil {
				return err
			}
			raw, err := marshalPrediction(row, capacity)
			if err != nil {
				return err
			}
			items = append(items, raw)
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	body, err := marshalList(items, limit, offset)
	if err != nil {
		return Result{}, err
	}
	return Result{Status: 200, Body: body}, nil
}

func (s *Service) ActivatePrediction(ctx context.Context, actor Actor, id uuid.UUID, version int, key, hash string) (Result, error) {
	current, err := s.currentPrediction(ctx, actor.TenantID, id)
	if err != nil {
		return Result{}, err
	}
	if !current.IsCurrent {
		return Result{}, apperrors.Conflict(domain.ReasonPredictionSuperseded, map[string]any{"reason": domain.ReasonPredictionSuperseded})
	}
	if current.Version != version {
		return Result{}, apperrors.Conflict("prediction version conflict", nil)
	}
	now := s.now()
	if now.After(current.AvailabilityWindowEnd) {
		bnometrics.Prediction("reject", domain.ReasonPredictionExpired)
		return Result{}, reasonError(domain.ReasonPredictionExpired)
	}
	draft, err := s.forecast(ctx, actor.TenantID, current.ShipmentID)
	if err != nil {
		return Result{}, err
	}
	if draft.Reason == domain.ReasonShipmentCancelled {
		if err := s.withdrawPrediction(ctx, actor, current, now); err != nil {
			return Result{}, err
		}
		return Result{}, reasonError(draft.Reason)
	}
	if draft.Reason != "" {
		bnometrics.Prediction("reject", draft.Reason)
		return Result{}, reasonError(draft.Reason)
	}
	if draft.Prediction.InputFingerprint != current.InputFingerprint {
		return Result{}, apperrors.Conflict("prediction inputs changed; refresh is required", map[string]any{"reason": "PREDICTION_INPUTS_CHANGED"})
	}
	if draft.Prediction.Confidence < s.policy.ConfidenceFloor {
		bnometrics.Prediction("reject", domain.ReasonConfidenceBelowFloor)
		return Result{}, reasonError(domain.ReasonConfidenceBelowFloor)
	}
	return s.mutate(ctx, actor.TenantID, key, hash, func(tx repository.Tx, result *Result) error {
		if done, err := takeIdempotency(ctx, tx, actor.TenantID, key, hash, result); done || err != nil {
			return err
		}
		prediction, capacity, err := s.ownPrediction(ctx, tx, actor.TenantID, id)
		if err != nil {
			return err
		}
		if !prediction.IsCurrent {
			return apperrors.Conflict(domain.ReasonPredictionSuperseded, map[string]any{"reason": domain.ReasonPredictionSuperseded})
		}
		if prediction.Version != version || capacity.Status != domain.CapacityPredicted {
			return apperrors.Conflict("prediction version conflict", nil)
		}
		previous := capacity.Version
		capacity.Status = domain.CapacityAvailable
		capacity.VisibilityScope = domain.CapVisPrivate
		capacity.Version++
		capacity.UpdatedAt = now
		if err := tx.UpdateCapacity(ctx, capacity, previous); err != nil {
			return err
		}
		if err := s.audit(ctx, tx, actor, "predicted_capacity", prediction.ID, "prediction_activated", domain.CapacityPredicted, domain.CapacityAvailable, domain.CapVisPrivate, domain.SourceCurrentShipmentPrediction, &prediction.ShipmentID, now); err != nil {
			return err
		}
		if err := s.emit(ctx, tx, domain.EventCapacityUpdated, actor.TenantID, capacity.ID, capacity.Version, capacity.Status, capacity.VisibilityScope, capacity.LocationID, now); err != nil {
			return err
		}
		body, err := marshalPrediction(prediction, capacity)
		if err != nil {
			return err
		}
		result.Status = 200
		result.Body = body
		result.AggregateID = prediction.ID
		bnometrics.Prediction("activated", "ok")
		return saveIdempotency(ctx, tx, actor.TenantID, key, hash, result.Status, body)
	})
}

func (s *Service) recompute(ctx context.Context, actor Actor, shipmentID, refreshID uuid.UUID, expectedVersion int, refreshing bool, key, hash string) (Result, error) {
	draft, err := s.forecast(ctx, actor.TenantID, shipmentID)
	if err != nil {
		return Result{}, err
	}
	now := s.now()
	if draft.Reason == domain.ReasonShipmentCancelled {
		current, readErr := s.predictionByShipment(ctx, actor.TenantID, shipmentID)
		if readErr == nil {
			if err := s.withdrawPrediction(ctx, actor, current, now); err != nil {
				return Result{}, err
			}
		}
		bnometrics.Prediction("reject", draft.Reason)
		return Result{}, reasonError(draft.Reason)
	}
	if draft.Reason != "" {
		bnometrics.Prediction("reject", draft.Reason)
		return Result{}, reasonError(draft.Reason)
	}
	return s.mutate(ctx, actor.TenantID, key, hash, func(tx repository.Tx, result *Result) error {
		if done, err := takeIdempotency(ctx, tx, actor.TenantID, key, hash, result); done || err != nil {
			return err
		}
		current, err := tx.CurrentPredictionByShipment(ctx, actor.TenantID, shipmentID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if err == nil {
			if refreshing && (current.ID != refreshID || current.Version != expectedVersion) {
				return apperrors.Conflict(domain.ReasonPredictionSuperseded, map[string]any{"reason": domain.ReasonPredictionSuperseded})
			}
			if current.InputFingerprint == draft.Prediction.InputFingerprint {
				capacity, err := tx.GetCapacity(ctx, current.CapacityID)
				if err != nil {
					return err
				}
				body, err := marshalPrediction(current, capacity)
				if err != nil {
					return err
				}
				result.Status = 200
				result.Body = body
				result.AggregateID = current.ID
				return saveIdempotency(ctx, tx, actor.TenantID, key, hash, result.Status, body)
			}
			current.IsCurrent = false
			if err := tx.UpdatePrediction(ctx, current); err != nil {
				return err
			}
			if err := s.audit(ctx, tx, actor, "predicted_capacity", current.ID, "prediction_superseded", domain.CapacityPredicted, domain.CapacityPredicted, domain.CapVisPrivate, domain.SourceCurrentShipmentPrediction, &shipmentID, now); err != nil {
				return err
			}
			draft.Prediction.SupersedesPredictionID = &current.ID
		} else if refreshing {
			return apperrors.Conflict(domain.ReasonPredictionSuperseded, map[string]any{"reason": domain.ReasonPredictionSuperseded})
		}
		return s.insertPrediction(ctx, tx, actor, draft.Prediction, refreshing, now, result, key, hash)
	})
}

func (s *Service) insertPrediction(ctx context.Context, tx repository.Tx, actor Actor, prediction domain.PredictedCapacity, refreshed bool, now time.Time, result *Result, key, hash string) error {
	capacityID := uuid.New()
	predictionID := uuid.New()
	prediction.ID = predictionID
	prediction.CapacityID = capacityID
	prediction.GeneratedAt = now
	prediction.CreatedAt = now
	bodyType := ""
	if prediction.BodyType != nil {
		bodyType = *prediction.BodyType
	}
	var vehicleID *uuid.UUID
	if prediction.VehicleID != uuid.Nil {
		id := prediction.VehicleID
		vehicleID = &id
	}
	status := domain.CapacityPredicted
	if s.policy.AutoActivate && prediction.Confidence >= s.policy.ConfidenceFloor {
		status = domain.CapacityAvailable
	}
	capacity := domain.Capacity{
		ID: capacityID, OwnerTenantID: actor.TenantID, CarrierCompanyID: actor.CompanyID, VehicleID: vehicleID,
		LocationID: &prediction.DestinationLocationID,
		Latitude:   prediction.DestinationLatitude, Longitude: prediction.DestinationLongitude,
		AvailableFrom: prediction.AvailabilityWindowStart, AvailableUntil: prediction.AvailabilityWindowEnd,
		Source: domain.SourceCurrentShipmentPrediction, BodyType: bodyType,
		PayloadRemainingKg: prediction.CapacityWeightKg, VolumeRemainingM3: prediction.CapacityVolumeM3,
		VisibilityScope: domain.CapVisPrivate, Status: status, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.InsertCapacity(ctx, capacity); err != nil {
		return err
	}
	if err := tx.InsertPrediction(ctx, prediction); err != nil {
		return err
	}
	action := "prediction_generated"
	if refreshed || prediction.SupersedesPredictionID != nil {
		action = "prediction_refreshed"
	}
	if err := s.audit(ctx, tx, actor, "predicted_capacity", prediction.ID, action, "", status, domain.CapVisPrivate, domain.SourceCurrentShipmentPrediction, &prediction.ShipmentID, now); err != nil {
		return err
	}
	if err := s.emitPredicted(ctx, tx, actor.TenantID, prediction, capacity.Version, now); err != nil {
		return err
	}
	body, err := marshalPrediction(prediction, capacity)
	if err != nil {
		return err
	}
	result.Status = 201
	result.Body = body
	result.AggregateID = prediction.ID
	bnometrics.Prediction("success", "ok")
	return saveIdempotency(ctx, tx, actor.TenantID, key, hash, result.Status, body)
}

func (s *Service) forecast(ctx context.Context, tenant, shipmentID uuid.UUID) (predict.Draft, error) {
	if s.sources == nil {
		return predict.Draft{}, apperrors.ServiceUnavailable("prediction sources are not configured")
	}
	shipment, err := s.sources.Shipment(ctx, tenant, shipmentID)
	if err != nil {
		return predict.Draft{}, mapSource(err)
	}
	if shipment.TenantID != tenant || shipment.ID != shipmentID {
		return predict.Draft{}, apperrors.NotFound("shipment not found")
	}
	var vehicle *predict.VehicleFact
	var eta *predict.ETAFact
	if shipment.VehicleID != nil && domain.ShipmentEligible(shipment.Status) && shipment.OtherActiveAssignments == 0 && shipment.DestinationLocationID != uuid.Nil && shipment.Status != "CANCELLED" {
		loaded, err := s.sources.Vehicle(ctx, tenant, *shipment.VehicleID)
		if err != nil {
			if errors.Is(err, predict.ErrNotFound) {
				return predict.Draft{Reason: domain.ReasonVehicleCapacityUnavailable}, nil
			}
			return predict.Draft{}, mapSource(err)
		}
		if loaded.ID != *shipment.VehicleID {
			return predict.Draft{Reason: domain.ReasonVehicleCapacityUnavailable}, nil
		}
		vehicle = &loaded
		loadedETA, err := s.sources.ETA(ctx, tenant, shipmentID)
		if err != nil {
			if errors.Is(err, predict.ErrNotFound) {
				eta = &predict.ETAFact{}
			} else {
				return predict.Draft{}, mapSource(err)
			}
		} else {
			eta = &loadedETA
		}
	}
	return predict.Evaluate(s.now(), s.policy, shipment, vehicle, eta), nil
}

func (s *Service) withdrawPrediction(ctx context.Context, actor Actor, prediction domain.PredictedCapacity, now time.Time) error {
	_, err := s.mutate(ctx, actor.TenantID, "", "", func(tx repository.Tx, result *Result) error {
		current, capacity, err := s.ownPrediction(ctx, tx, actor.TenantID, prediction.ID)
		if err != nil {
			return err
		}
		if current.IsCurrent {
			current.IsCurrent = false
			if err := tx.UpdatePrediction(ctx, current); err != nil {
				return err
			}
		}
		if capacity.Source == domain.SourceCurrentShipmentPrediction && (capacity.Status == domain.CapacityPredicted || capacity.Status == domain.CapacityAvailable) {
			previous := capacity.Version
			from := capacity.Status
			capacity.Status = domain.CapacityWithdrawn
			capacity.Version++
			capacity.UpdatedAt = now
			if err := tx.UpdateCapacity(ctx, capacity, previous); err != nil {
				return err
			}
			if err := s.audit(ctx, tx, actor, "predicted_capacity", current.ID, "prediction_withdrawn", from, domain.CapacityWithdrawn, capacity.VisibilityScope, domain.SourceCurrentShipmentPrediction, &current.ShipmentID, now); err != nil {
				return err
			}
			if err := s.emit(ctx, tx, domain.EventCapacityWithdrawn, actor.TenantID, capacity.ID, capacity.Version, capacity.Status, capacity.VisibilityScope, capacity.LocationID, now); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (s *Service) currentPrediction(ctx context.Context, tenant, id uuid.UUID) (domain.PredictedCapacity, error) {
	var prediction domain.PredictedCapacity
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		loaded, _, err := s.ownPrediction(ctx, tx, tenant, id)
		prediction = loaded
		return err
	})
	return prediction, err
}

func (s *Service) predictionByShipment(ctx context.Context, tenant, shipmentID uuid.UUID) (domain.PredictedCapacity, error) {
	var prediction domain.PredictedCapacity
	err := s.store.Within(ctx, func(tx repository.Tx) error {
		loaded, err := tx.CurrentPredictionByShipment(ctx, tenant, shipmentID)
		if errors.Is(err, repository.ErrNotFound) {
			return apperrors.NotFound("prediction not found")
		}
		prediction = loaded
		return err
	})
	return prediction, err
}

func (s *Service) ownPrediction(ctx context.Context, tx repository.Tx, tenant, id uuid.UUID) (domain.PredictedCapacity, domain.Capacity, error) {
	prediction, err := tx.GetPrediction(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return domain.PredictedCapacity{}, domain.Capacity{}, apperrors.NotFound("prediction not found")
		}
		return domain.PredictedCapacity{}, domain.Capacity{}, err
	}
	if prediction.OwnerTenantID != tenant {
		return domain.PredictedCapacity{}, domain.Capacity{}, apperrors.NotFound("prediction not found")
	}
	capacity, err := s.ownCapacity(ctx, tx, tenant, prediction.CapacityID)
	if err != nil {
		return domain.PredictedCapacity{}, domain.Capacity{}, err
	}
	return prediction, capacity, nil
}

func (s *Service) emitPredicted(ctx context.Context, tx repository.Tx, tenant uuid.UUID, prediction domain.PredictedCapacity, capacityVersion int, now time.Time) error {
	id := uuid.New()
	payload, err := json.Marshal(map[string]any{
		"eventId": id, "eventName": domain.EventCapacityPredicted, "schemaVersion": 1,
		"tenantId": tenant, "aggregateId": prediction.CapacityID, "aggregateVersion": capacityVersion,
		"occurredAt": now.UTC().Format(time.RFC3339Nano), "status": domain.CapacityPredicted,
		"visibilityScope": domain.CapVisPrivate, "predictionId": prediction.ID,
		"capacityId": prediction.CapacityID, "shipmentId": prediction.ShipmentID,
		"locationId":  prediction.DestinationLocationID,
		"ruleVersion": prediction.RuleVersion, "confidence": prediction.Confidence,
		"predictionMethod": prediction.PredictionMethod,
	})
	if err != nil {
		return err
	}
	return tx.InsertOutbox(ctx, repository.OutboxEvent{
		ID: id, EventName: domain.EventCapacityPredicted, SchemaVersion: 1, TenantID: tenant,
		AggregateID: prediction.CapacityID, AggregateVersion: capacityVersion, OccurredAt: now, Payload: payload,
	})
}

func marshalPrediction(prediction domain.PredictedCapacity, capacity domain.Capacity) ([]byte, error) {
	return json.Marshal(struct {
		Prediction domain.PredictedCapacity `json:"prediction"`
		Capacity   domain.Capacity          `json:"capacity"`
	}{Prediction: prediction, Capacity: capacity})
}

func reasonError(reason string) error {
	return apperrors.Validation(reason, map[string]any{"reason": reason})
}

func mapSource(err error) error {
	switch {
	case errors.Is(err, predict.ErrNotFound):
		return apperrors.NotFound("shipment not found")
	case errors.Is(err, predict.ErrUnavailable):
		return apperrors.ServiceUnavailable("prediction source could not be read")
	default:
		return apperrors.ServiceUnavailable("prediction source could not be read")
	}
}
