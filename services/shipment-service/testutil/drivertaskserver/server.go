package drivertaskserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/http/handlers"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

type Env struct {
	Server  *httptest.Server
	Tasks   *service.DriverTaskService
	TaskRepo *repository.DriverTaskRepository
}

func New(pool *pgxpool.Pool, internalToken string) *Env {
	driverRepo := repository.NewDriverRepository(pool)
	shipmentRepo := repository.NewShipmentRepository(pool)
	taskRepo := repository.NewDriverTaskRepository(pool)
	deviceRepo := repository.NewDriverDeviceRepository(pool)
	taskSvc := service.NewDriverTaskService(driverRepo, shipmentRepo, taskRepo, deviceRepo)
	internalHandler := handlers.NewInternalDriverTaskHandler(taskSvc, internalToken)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/v1/driver/tasks", internalHandler.CreateTask)
	server := httptest.NewServer(mux)
	return &Env{Server: server, Tasks: taskSvc, TaskRepo: taskRepo}
}

func (e *Env) Close() {
	e.Server.Close()
}

func (e *Env) ListTasks(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.DriverTask, int, error) {
	return e.Tasks.ListTasks(ctx, tenantID, userID, domain.ListDriverTasksFilter{})
}

func (e *Env) SubmitDelayReason(ctx context.Context, tenantID, userID, taskID uuid.UUID, idempotencyKey string) error {
	body, _ := json.Marshal(map[string]string{"reason": "TRAFFIC", "comment": "Heavy traffic"})
	_, _, err := e.Tasks.SubmitResponse(ctx, service.SubmitTaskResponseInput{
		TenantID: tenantID, UserID: userID, TaskID: taskID, IdempotencyKey: idempotencyKey, Body: body,
	})
	return err
}

func (e *Env) ExpireDueTasks(ctx context.Context, limit int) (int, error) {
	return e.TaskRepo.ExpireDueTasks(ctx, limit)
}

func (e *Env) HTTPClient(internalToken string) string {
	return e.Server.URL
}
