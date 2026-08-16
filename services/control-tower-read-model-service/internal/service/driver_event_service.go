package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type DriverEventService struct {
	events     *repository.DriverEventRepository
	workflow   *repository.WorkflowRepository
	automation *AutomationService
	ingress    *AutomationTriggerIngress
	log        *slog.Logger
}

func NewDriverEventService(
	events *repository.DriverEventRepository,
	workflow *repository.WorkflowRepository,
	automation *AutomationService,
	ingress *AutomationTriggerIngress,
	log *slog.Logger,
) *DriverEventService {
	return &DriverEventService{
		events: events, workflow: workflow, automation: automation, ingress: ingress, log: log,
	}
}

type DriverEventHandleResult struct {
	Outcome   string
	Duplicate bool
}

func (s *DriverEventService) Handle(ctx context.Context, meta domain.KafkaRecordMeta, payload []byte, receivedAt time.Time) (DriverEventHandleResult, error) {
	env, permErr := domain.ParseDriverEventEnvelope(payload)
	if permErr != nil {
		return DriverEventHandleResult{Outcome: permErr.Code}, permErr
	}
	event, err := domain.NormalizeDriverEvent(env)
	if err != nil {
		return DriverEventHandleResult{Outcome: domain.DriverEventErrorInvalidJSON}, &domain.PermanentError{Code: domain.DriverEventErrorInvalidJSON}
	}

	sourceEventID, _ := uuid.Parse(strings.TrimSpace(env.SourceEventID))
	processResult, err := s.events.ProcessEvent(ctx, repository.DriverEventProcessInput{
		TenantID: event.TenantID, EventID: event.ID, EventType: event.Type, ShipmentID: &event.ShipmentID,
		SourceEventID: optionalUUID(sourceEventID), KafkaTopic: meta.Topic, KafkaPartition: meta.Partition,
		KafkaOffset: meta.Offset, PayloadSHA256: domain.DriverEventPayloadSHA256(payload),
		ProcessingOutcome: "ACCEPTED", ReceivedAt: receivedAt,
	})
	if err != nil {
		if repository.IsTenantMismatch(err) {
			return DriverEventHandleResult{Outcome: domain.DriverEventErrorTenantMismatch}, &domain.PermanentError{Code: domain.DriverEventErrorTenantMismatch}
		}
		return DriverEventHandleResult{}, err
	}
	if processResult.Duplicate {
		return DriverEventHandleResult{Outcome: "DUPLICATE", Duplicate: true}, nil
	}
	if !processResult.Inserted {
		return DriverEventHandleResult{Outcome: "IGNORED_UNKNOWN_SHIPMENT"}, nil
	}

	if event.Type == "driver.problem.reported" && s.workflow != nil {
		seed := domain.BuildDriverProblemExceptionSeed(event, env)
		if _, err := s.workflow.EnsureExceptionWorkflows(ctx, event.TenantID, []domain.EnsureExceptionSeed{seed}); err != nil {
			s.log.Warn("driver problem workflow seed failed", slog.String("event_id", event.ID.String()), slog.String("error", err.Error()))
		}
	}

	if trigger, ok := domain.MapDriverAutomationTrigger(event, env); ok && s.ingress != nil {
		if _, err := s.ingress.HandleTrigger(ctx, event.TenantID, trigger, true); err != nil {
			s.log.Warn("driver event automation failed", slog.String("event_type", event.Type), slog.String("event_id", event.ID.String()), slog.String("error", err.Error()))
		}
	}

	return DriverEventHandleResult{Outcome: "PROCESSED"}, nil
}

func optionalUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
