//go:build integration

package driverplatform

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/notification"
	"github.com/freight-platform/shipment-service/internal/push"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

func setupTaskEnv(t *testing.T) (*testEnv, *service.DriverTaskService, *push.FakeProvider, *notification.Worker) {
	t.Helper()
	env := setupTestEnv(t)
	driverRepo := repository.NewDriverRepository(env.pool)
	taskRepo := repository.NewDriverTaskRepository(env.pool)
	deviceRepo := repository.NewDriverDeviceRepository(env.pool)
	taskSvc := service.NewDriverTaskService(driverRepo, env.shipmentRepo, taskRepo, deviceRepo)
	fakePush := push.NewFakeProvider()
	worker := notification.NewWorker(notification.WorkerConfig{
		Enabled: true, WorkerID: "test-worker", PollInterval: time.Second,
		BatchSize: 10, LeaseTimeout: 30 * time.Second, MaxAttempts: 3, RetryBackoff: time.Millisecond,
	}, deviceRepo, taskRepo, fakePush, nil)
	return env, taskSvc, fakePush, worker
}

func TestDriverTaskCreateAndInbox(t *testing.T) {
	env, taskSvc, _, _ := setupTaskEnv(t)
	fix := seedDriverFixture(t, env.pool)
	ctx := context.Background()
	expires := time.Now().UTC().Add(2 * time.Hour)
	corr := "CORR-CT-1"
	sourceEvent := "CT-EVENT-X"
	task, err := taskSvc.CreateTaskInternal(ctx, domain.CreateDriverTaskInput{
		TenantID: fix.TenantID, DriverID: fix.DriverID, ShipmentID: &fix.ShipmentID,
		TaskType: domain.DriverTaskTypeRequestDelayReason, Priority: domain.DriverTaskPriorityHigh,
		ExpiresAt: &expires, Source: domain.DriverTaskSourceControlTower,
		SourceEventID: &sourceEvent, CorrelationID: &corr, IdempotencyKey: "idem-task-1",
		CreatedByType: domain.DriverTaskCreatorControlTower,
	})
	require.NoError(t, err)
	require.NotNil(t, task)

	items, total, err := taskSvc.ListTasks(ctx, fix.TenantID, fix.UserID, domain.ListDriverTasksFilter{})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)

	fixB := seedSecondDriverFixture(t, env.pool, fix.TenantID)
	_, err = taskSvc.GetTask(ctx, fix.TenantID, fixB.UserID, task.ID)
	require.Error(t, err)
}

func TestDriverTaskConcurrentCreate(t *testing.T) {
	env, taskSvc, _, _ := setupTaskEnv(t)
	fix := seedDriverFixture(t, env.pool)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := taskSvc.CreateTaskInternal(ctx, domain.CreateDriverTaskInput{
				TenantID: fix.TenantID, DriverID: fix.DriverID, ShipmentID: &fix.ShipmentID,
				TaskType: domain.DriverTaskTypeRequestDelayReason, Source: domain.DriverTaskSourceControlTower,
				IdempotencyKey: "idem-concurrent-task",
				CreatedByType: domain.DriverTaskCreatorControlTower,
			})
			errs[idx] = err
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	var count int
	require.NoError(t, env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.driver_task WHERE tenant_id=$1 AND idempotency_key='idem-concurrent-task'`, fix.TenantID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestDriverTaskResponseAndIdempotency(t *testing.T) {
	env, taskSvc, _, _ := setupTaskEnv(t)
	fix := seedDriverFixture(t, env.pool)
	ctx := context.Background()
	task, err := taskSvc.CreateTaskInternal(ctx, domain.CreateDriverTaskInput{
		TenantID: fix.TenantID, DriverID: fix.DriverID, ShipmentID: &fix.ShipmentID,
		TaskType: domain.DriverTaskTypeRequestDelayReason, Source: domain.DriverTaskSourceControlTower,
		IdempotencyKey: "resp-task-1", CreatedByType: domain.DriverTaskCreatorControlTower,
	})
	require.NoError(t, err)

	body, _ := json.Marshal(domain.DelayReasonResponse{Reason: "TRAFFIC"})
	completed, _, err := taskSvc.SubmitResponse(ctx, service.SubmitTaskResponseInput{
		TenantID: fix.TenantID, UserID: fix.UserID, TaskID: task.ID,
		IdempotencyKey: "resp-idem-1", Body: body,
	})
	require.NoError(t, err)
	require.Equal(t, domain.DriverTaskStatusCompleted, completed.Status)

	completed2, _, err := taskSvc.SubmitResponse(ctx, service.SubmitTaskResponseInput{
		TenantID: fix.TenantID, UserID: fix.UserID, TaskID: task.ID,
		IdempotencyKey: "resp-idem-1", Body: body,
	})
	require.NoError(t, err)
	require.Equal(t, completed.ID, completed2.ID)

	var respCount int
	require.NoError(t, env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transport.driver_task_response WHERE task_id=$1`, task.ID).Scan(&respCount))
	require.Equal(t, 1, respCount)
}

func TestDriverTaskUnsupportedTypeRejected(t *testing.T) {
	env, taskSvc, _, _ := setupTaskEnv(t)
	fix := seedDriverFixture(t, env.pool)
	_, err := taskSvc.CreateTaskInternal(context.Background(), domain.CreateDriverTaskInput{
		TenantID: fix.TenantID, DriverID: fix.DriverID, ShipmentID: &fix.ShipmentID,
		TaskType: "CANCEL_SHIPMENT", Source: domain.DriverTaskSourceControlTower,
		IdempotencyKey: "bad-type", CreatedByType: domain.DriverTaskCreatorControlTower,
	})
	require.Error(t, err)
}

func TestDriverTaskPushDeliveryFakeProvider(t *testing.T) {
	env, taskSvc, fakePush, worker := setupTaskEnv(t)
	fix := seedDriverFixture(t, env.pool)
	ctx := context.Background()

	_, err := taskSvc.RegisterDevice(ctx, fix.TenantID, fix.UserID, "ANDROID", "device-1", "token-abc", nil, nil)
	require.NoError(t, err)

	_, err = taskSvc.CreateTaskInternal(ctx, domain.CreateDriverTaskInput{
		TenantID: fix.TenantID, DriverID: fix.DriverID, ShipmentID: &fix.ShipmentID,
		TaskType: domain.DriverTaskTypeRequestDelayReason, Source: domain.DriverTaskSourceControlTower,
		IdempotencyKey: "push-task-1", CreatedByType: domain.DriverTaskCreatorControlTower,
	})
	require.NoError(t, err)

	worker.ProcessOnce(ctx)
	require.Equal(t, 1, fakePush.SentCount())
}

func TestDriverTaskNoDeviceStillWorks(t *testing.T) {
	env, taskSvc, _, _ := setupTaskEnv(t)
	fix := seedDriverFixture(t, env.pool)
	ctx := context.Background()
	task, err := taskSvc.CreateTaskInternal(ctx, domain.CreateDriverTaskInput{
		TenantID: fix.TenantID, DriverID: fix.DriverID, ShipmentID: &fix.ShipmentID,
		TaskType: domain.DriverTaskTypeRequestDelayReason, Source: domain.DriverTaskSourceControlTower,
		IdempotencyKey: "no-device-task", CreatedByType: domain.DriverTaskCreatorControlTower,
	})
	require.NoError(t, err)

	items, _, err := taskSvc.ListTasks(ctx, fix.TenantID, fix.UserID, domain.ListDriverTasksFilter{})
	require.NoError(t, err)
	require.Len(t, items, 1)

	body, _ := json.Marshal(domain.DelayReasonResponse{Reason: "TRAFFIC"})
	completed, _, err := taskSvc.SubmitResponse(ctx, service.SubmitTaskResponseInput{
		TenantID: fix.TenantID, UserID: fix.UserID, TaskID: task.ID,
		IdempotencyKey: "no-dev-resp", Body: body,
	})
	require.NoError(t, err)
	require.Equal(t, domain.DriverTaskStatusCompleted, completed.Status)
}

func TestDriverTaskExpiration(t *testing.T) {
	env, taskSvc, _, _ := setupTaskEnv(t)
	fix := seedDriverFixture(t, env.pool)
	ctx := context.Background()
	future := time.Now().UTC().Add(time.Hour)
	task, err := taskSvc.CreateTaskInternal(ctx, domain.CreateDriverTaskInput{
		TenantID: fix.TenantID, DriverID: fix.DriverID, ShipmentID: &fix.ShipmentID,
		TaskType: domain.DriverTaskTypeRequestDelayReason, Source: domain.DriverTaskSourceControlTower,
		ExpiresAt: &future, IdempotencyKey: "expire-task-1", CreatedByType: domain.DriverTaskCreatorControlTower,
	})
	require.NoError(t, err)
	past := time.Now().UTC().Add(-time.Minute)
	_, err = env.pool.Exec(ctx, `UPDATE transport.driver_task SET expires_at=$1 WHERE id=$2`, past, task.ID)
	require.NoError(t, err)
	var eligible int
	require.NoError(t, env.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM transport.driver_task
WHERE id=$1 AND status IN ('PENDING','DELIVERED','READ','ACKNOWLEDGED') AND expires_at IS NOT NULL AND expires_at <= now()`, task.ID).Scan(&eligible))
	require.Equal(t, 1, eligible)

	taskRepo := repository.NewDriverTaskRepository(env.pool)
	n, err := taskRepo.ExpireDueTasks(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	expired, err := taskSvc.GetTask(ctx, fix.TenantID, fix.UserID, task.ID)
	require.NoError(t, err)
	require.Equal(t, domain.DriverTaskStatusExpired, expired.Status)

	body, _ := json.Marshal(domain.DelayReasonResponse{Reason: "TRAFFIC"})
	_, _, err = taskSvc.SubmitResponse(ctx, service.SubmitTaskResponseInput{
		TenantID: fix.TenantID, UserID: fix.UserID, TaskID: task.ID,
		IdempotencyKey: "expire-resp", Body: body,
	})
	require.Error(t, err)
}

func TestControlTowerCompatibilityChain(t *testing.T) {
	env, taskSvc, _, _ := setupTaskEnv(t)
	fix := seedDriverFixture(t, env.pool)
	ctx := context.Background()
	sourceEvent := "CT-EVENT-CHAIN"
	corr := "CORR-CHAIN"
	task, err := taskSvc.CreateTaskInternal(ctx, domain.CreateDriverTaskInput{
		TenantID: fix.TenantID, DriverID: fix.DriverID, ShipmentID: &fix.ShipmentID,
		TaskType: domain.DriverTaskTypeRequestDelayReason, Source: domain.DriverTaskSourceControlTower,
		SourceEventID: &sourceEvent, CorrelationID: &corr, IdempotencyKey: "ct-chain-1",
		CreatedByType: domain.DriverTaskCreatorControlTower,
	})
	require.NoError(t, err)

	body, _ := json.Marshal(domain.DelayReasonResponse{Reason: "VEHICLE_BREAKDOWN"})
	completed, _, err := taskSvc.SubmitResponse(ctx, service.SubmitTaskResponseInput{
		TenantID: fix.TenantID, UserID: fix.UserID, TaskID: task.ID,
		IdempotencyKey: "ct-resp-1", Body: body,
	})
	require.NoError(t, err)
	require.Equal(t, domain.DriverTaskStatusCompleted, completed.Status)

	var payload []byte
	require.NoError(t, env.pool.QueryRow(ctx, `
SELECT payload FROM transport.shipment_event_outbox
WHERE event_type=$1 AND tenant_id=$2 ORDER BY created_at DESC LIMIT 1`,
		domain.OutboxEventTypeDriverTaskCompleted, fix.TenantID).Scan(&payload))
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Equal(t, sourceEvent, decoded["sourceEventId"])
	require.Equal(t, corr, decoded["correlationId"])
}

func seedSecondDriverFixture(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) driverFixture {
	t.Helper()
	ctx := context.Background()
	fix := driverFixture{
		TenantID: tenantID, UserID: uuid.New(), DriverID: uuid.New(),
		CarrierID: uuid.New(), ShipmentID: uuid.New(),
	}
	shipper, consignee, origin, dest, orderID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO core.companies (id,tenant_id,legal_name,company_type) VALUES ($1,$2,'Carrier B','CARRIER')`, fix.CarrierID, tenantID)
	require.NoError(t, err)
	for _, row := range []struct{ id uuid.UUID; typ, name string }{
		{shipper, "SHIPPER", "Shipper B"}, {consignee, "CONSIGNEE", "Consignee B"},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO core.companies (id,tenant_id,legal_name,company_type) VALUES ($1,$2,$3,$4)`, row.id, tenantID, row.name, row.typ)
		require.NoError(t, err)
	}
	for _, loc := range []struct{ id uuid.UUID; name string }{{origin, "O"}, {dest, "D"}} {
		_, err = pool.Exec(ctx, `INSERT INTO transport.locations (id,tenant_id,location_type,name,country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`, loc.id, tenantID, loc.name)
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO transport.transport_orders (id,tenant_id,order_number,status,shipper_company_id,consignee_company_id,origin_location_id,destination_location_id,transport_mode) VALUES ($1,$2,'TO-B','ASSIGNED',$3,$4,$5,$6,'ROAD')`, orderID, tenantID, shipper, consignee, origin, dest)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO transport.drivers (id,tenant_id,carrier_company_id,user_id,full_name,status) VALUES ($1,$2,$3,$4,'Driver B','ACTIVE')`, fix.DriverID, tenantID, fix.CarrierID, fix.UserID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO transport.shipments (id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id, carrier_company_id, driver_id, origin_location_id, destination_location_id, transport_mode, status, version)
VALUES ($1,$2,'SHP-B',$3,$4,$5,$6,$7,$8,$9,'ROAD','PICKUP_SLOT_BOOKED',1)`,
		fix.ShipmentID, tenantID, orderID, shipper, consignee, fix.CarrierID, fix.DriverID, origin, dest)
	require.NoError(t, err)
	return fix
}
