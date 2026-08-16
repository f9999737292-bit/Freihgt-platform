package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

type DriverTaskService struct {
	drivers   *repository.DriverRepository
	shipments *repository.ShipmentRepository
	tasks     *repository.DriverTaskRepository
	devices   *repository.DriverDeviceRepository
}

func NewDriverTaskService(
	drivers *repository.DriverRepository,
	shipments *repository.ShipmentRepository,
	tasks *repository.DriverTaskRepository,
	devices *repository.DriverDeviceRepository,
) *DriverTaskService {
	return &DriverTaskService{drivers: drivers, shipments: shipments, tasks: tasks, devices: devices}
}

func (s *DriverTaskService) CreateTaskInternal(ctx context.Context, in domain.CreateDriverTaskInput) (*domain.DriverTask, error) {
	if err := domain.ValidateCreateDriverTaskInput(in); err != nil {
		return nil, err
	}
	if _, err := s.drivers.GetByIDAndTenant(ctx, in.DriverID, in.TenantID); err != nil {
		return nil, apperrors.NotFound("driver not found")
	}
	shipmentVersion := 0
	if in.ShipmentID != nil {
		shipment, err := s.shipments.GetByIDAndTenant(ctx, *in.ShipmentID, in.TenantID)
		if err != nil {
			return nil, apperrors.NotFound("shipment not found")
		}
		if shipment.DriverID == nil || *shipment.DriverID != in.DriverID {
			return nil, apperrors.Validation("driver is not assigned to shipment", map[string]any{"field": "driverId"})
		}
		shipmentVersion = shipment.Version
	}
	priority := strings.TrimSpace(in.Priority)
	if priority == "" {
		priority = domain.DriverTaskPriorityNormal
	}
	var idemPtr *string
	if key := strings.TrimSpace(in.IdempotencyKey); key != "" {
		idemPtr = &key
	}
	task := domain.DriverTask{
		TenantID: in.TenantID, DriverID: in.DriverID, ShipmentID: in.ShipmentID,
		TaskType: in.TaskType, Status: domain.DriverTaskStatusPending, Priority: priority,
		Title: domain.TaskTitle(in.TaskType), Payload: json.RawMessage(`{}`),
		AvailableAt: time.Now().UTC(), ExpiresAt: in.ExpiresAt,
		CreatedByType: in.CreatedByType, CreatedByID: in.CreatedByID,
		Source: strings.ToUpper(strings.TrimSpace(in.Source)),
		CorrelationID: in.CorrelationID, SourceEventID: in.SourceEventID, IdempotencyKey: idemPtr,
	}
	created, _, err := s.tasks.CreateTask(ctx, repository.CreateDriverTaskParams{Task: task, ShipmentVersion: shipmentVersion})
	return created, err
}

func (s *DriverTaskService) CancelTaskInternal(ctx context.Context, tenantID, taskID uuid.UUID) (*domain.DriverTask, error) {
	task, err := s.tasks.GetTaskByID(ctx, tenantID, taskID)
	if err != nil {
		return nil, err
	}
	if domain.IsDriverTaskTerminal(task.Status) {
		return task, nil
	}
	version := 0
	if task.ShipmentID != nil {
		if shipment, err := s.shipments.GetByIDAndTenant(ctx, *task.ShipmentID, tenantID); err == nil {
			version = shipment.Version
		}
	}
	cancelled, _, err := s.tasks.CancelTask(ctx, *task, version)
	return cancelled, err
}

func (s *DriverTaskService) ListTasks(ctx context.Context, tenantID, userID uuid.UUID, filter domain.ListDriverTasksFilter) ([]domain.DriverTask, int, error) {
	resolved, err := s.resolveDriver(ctx, tenantID, userID)
	if err != nil {
		return nil, 0, err
	}
	filter.TenantID = tenantID
	filter.DriverID = resolved.ID
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	return s.tasks.ListTasks(ctx, filter)
}

func (s *DriverTaskService) GetTask(ctx context.Context, tenantID, userID, taskID uuid.UUID) (*domain.DriverTask, error) {
	resolved, err := s.resolveDriver(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	return s.tasks.GetTaskByIDAndDriver(ctx, tenantID, resolved.ID, taskID)
}

func (s *DriverTaskService) MarkRead(ctx context.Context, tenantID, userID, taskID uuid.UUID) (*domain.DriverTask, error) {
	task, err := s.GetTask(ctx, tenantID, userID, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status == domain.DriverTaskStatusRead || domain.IsDriverTaskTerminal(task.Status) {
		return task, nil
	}
	newStatus := domain.DriverTaskStatusRead
	if !domain.CanTransitionDriverTask(task.Status, newStatus) {
		return task, nil
	}
	return s.tasks.TransitionTask(ctx, repository.TransitionTaskParams{
		Task: *task, NewStatus: newStatus, SetReadAt: true,
	})
}

func (s *DriverTaskService) Acknowledge(ctx context.Context, tenantID, userID, taskID uuid.UUID) (*domain.DriverTask, error) {
	task, err := s.GetTask(ctx, tenantID, userID, taskID)
	if err != nil {
		return nil, err
	}
	if task.Status == domain.DriverTaskStatusAcknowledged || domain.IsDriverTaskTerminal(task.Status) {
		return task, nil
	}
	newStatus := domain.DriverTaskStatusAcknowledged
	if !domain.CanTransitionDriverTask(task.Status, newStatus) && task.Status != domain.DriverTaskStatusRead {
		if domain.CanTransitionDriverTask(task.Status, domain.DriverTaskStatusRead) {
			task, err = s.tasks.TransitionTask(ctx, repository.TransitionTaskParams{
				Task: *task, NewStatus: domain.DriverTaskStatusRead, SetReadAt: true,
			})
			if err != nil {
				return nil, err
			}
		}
	}
	if !domain.CanTransitionDriverTask(task.Status, newStatus) {
		return task, apperrors.Conflict("task cannot be acknowledged in current state", map[string]any{"status": task.Status})
	}
	return s.tasks.TransitionTask(ctx, repository.TransitionTaskParams{
		Task: *task, NewStatus: newStatus, SetAckAt: true, SetReadAt: true,
	})
}

type SubmitTaskResponseInput struct {
	TenantID       uuid.UUID
	UserID         uuid.UUID
	TaskID         uuid.UUID
	IdempotencyKey string
	Body           json.RawMessage
}

func (s *DriverTaskService) SubmitResponse(ctx context.Context, in SubmitTaskResponseInput) (*domain.DriverTask, *domain.DriverTaskResponse, error) {
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return nil, nil, apperrors.Validation("idempotency_key is required", map[string]any{"field": "idempotencyKey"})
	}
	resolved, err := s.resolveDriver(ctx, in.TenantID, in.UserID)
	if err != nil {
		return nil, nil, err
	}
	task, err := s.tasks.GetTaskByIDAndDriver(ctx, in.TenantID, resolved.ID, in.TaskID)
	if err != nil {
		return nil, nil, err
	}
	if existing, err := s.tasks.GetResponseByIdempotency(ctx, in.TenantID, in.TaskID, key); err != nil {
		return nil, nil, err
	} else if existing != nil {
		return task, existing, nil
	}
	if !domain.CanDriverRespondToTask(task) {
		return nil, nil, apperrors.Conflict("task is not open for response", map[string]any{"status": task.Status})
	}
	if task.ShipmentID != nil {
		shipment, err := s.shipments.GetByIDAndTenant(ctx, *task.ShipmentID, in.TenantID)
		if err != nil {
			return nil, nil, err
		}
		if shipment.DriverID == nil || *shipment.DriverID != resolved.ID {
			return nil, nil, apperrors.Validation("driver is no longer assigned to shipment", nil)
		}
	}
	responseBody, responseType, err := s.validateResponse(task.TaskType, in.Body)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	resp := domain.DriverTaskResponse{
		TenantID: in.TenantID, TaskID: task.ID, DriverID: resolved.ID,
		ResponseType: responseType, ResponseBody: responseBody,
		ReceivedAt: now, IdempotencyKey: key,
	}
	version := 0
	if task.ShipmentID != nil {
		if shipment, err := s.shipments.GetByIDAndTenant(ctx, *task.ShipmentID, in.TenantID); err == nil {
			version = shipment.Version
		}
	}
	completedTask, completedResp, _, err := s.tasks.CompleteTask(ctx, repository.CompleteTaskParams{
		Task: *task, Response: resp, ShipmentVersion: version,
	})
	return completedTask, completedResp, err
}

func (s *DriverTaskService) RegisterDevice(ctx context.Context, tenantID, userID uuid.UUID, platform, deviceInstanceID, pushToken string, appVersion, locale *string) (*domain.DriverDevice, error) {
	resolved, err := s.resolveDriver(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	platform = strings.ToUpper(strings.TrimSpace(platform))
	switch platform {
	case "ANDROID", "IOS", "WEB":
	default:
		return nil, apperrors.Validation("unsupported platform", map[string]any{"field": "platform"})
	}
	if strings.TrimSpace(pushToken) == "" {
		return nil, apperrors.Validation("pushToken is required", map[string]any{"field": "pushToken"})
	}
	if strings.TrimSpace(deviceInstanceID) == "" {
		return nil, apperrors.Validation("deviceInstanceId is required", map[string]any{"field": "deviceInstanceId"})
	}
	return s.devices.RegisterDevice(ctx, repository.RegisterDeviceInput{
		TenantID: tenantID, DriverID: resolved.ID, Platform: platform,
		PushToken: pushToken, DeviceInstanceID: deviceInstanceID,
		AppVersion: appVersion, Locale: locale,
	})
}

func (s *DriverTaskService) RevokeDevice(ctx context.Context, tenantID, userID, deviceID uuid.UUID) error {
	resolved, err := s.resolveDriver(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	return s.devices.RevokeDevice(ctx, tenantID, resolved.ID, deviceID)
}

func (s *DriverTaskService) validateResponse(taskType string, body json.RawMessage) (json.RawMessage, string, error) {
	switch taskType {
	case domain.DriverTaskTypeRequestDelayReason:
		var payload domain.DelayReasonResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, "", apperrors.Validation("invalid response body", map[string]any{"field": "body"})
		}
		payload.Reason = strings.ToUpper(strings.TrimSpace(payload.Reason))
		payload.Comment = domain.SanitizeTaskComment(payload.Comment)
		if err := domain.ValidateDelayReasonResponse(payload); err != nil {
			return nil, "", err
		}
		raw, _ := json.Marshal(payload)
		return raw, "DELAY_REASON", nil
	default:
		return nil, "", apperrors.Validation("task type does not accept responses in v0.2", map[string]any{"taskType": taskType})
	}
}

func (s *DriverTaskService) resolveDriver(ctx context.Context, tenantID, userID uuid.UUID) (*domain.Driver, error) {
	if tenantID == uuid.Nil || userID == uuid.Nil {
		return nil, apperrors.Unauthorized("authentication required")
	}
	driver, err := s.drivers.GetByUserIDAndTenant(ctx, userID, tenantID)
	if err != nil {
		return nil, err
	}
	if driver.Status != domain.DriverStatusActive {
		return nil, apperrors.Unauthorized("driver is not active")
	}
	return driver, nil
}
