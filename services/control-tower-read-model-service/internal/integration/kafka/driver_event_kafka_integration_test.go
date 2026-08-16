//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/freight-platform/control-tower-read-model-service/internal/config"
	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/driverconsumer"
	pgintegration "github.com/freight-platform/control-tower-read-model-service/internal/integration/postgres"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
)

func TestDriverDelayEventLiveKafkaPathIntegration(t *testing.T) {
	topic := "driver.events.v1.test." + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(1, topic))
	require.NoError(t, err)
	t.Cleanup(cluster.Close)
	brokers := cluster.ListenAddrs()

	env := pgintegration.SetupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	carrierID := uuid.New()
	shipmentID := uuid.New()
	seedShipmentFixture(t, ctx, env.Pool, tenantID, userID, driverID, carrierID, shipmentID)

	autoRepo := repository.NewAutomationRepository(env.Pool)
	workflowRepo := repository.NewWorkflowRepository(env.Pool)
	automationSvc := service.NewAutomationService(autoRepo)
	operatorID := uuid.New()
	p, pv, err := autoRepo.CreatePlaybook(ctx, tenantID, operatorID, domain.CreatePlaybookInput{
		Name: "Kafka delay", Steps: []domain.PlaybookStepInput{{Sequence: 1, Title: "Notify", StepType: domain.StepTypeInstruction, ActionCode: "contact_driver", Required: true}},
	})
	require.NoError(t, err)
	_, err = env.Pool.Exec(ctx, `UPDATE control_tower.operational_playbook SET status='active', current_version=1 WHERE tenant_id=$1 AND id=$2`, tenantID, p.ID)
	require.NoError(t, err)
	_, err = env.Pool.Exec(ctx, `UPDATE control_tower.operational_playbook_version SET status='active' WHERE tenant_id=$1 AND id=$2`, tenantID, pv.ID)
	require.NoError(t, err)
	rule, err := autoRepo.CreateRule(ctx, tenantID, operatorID, domain.CreateRuleInput{
		Name: "Kafka delay rule", TriggerType: domain.DriverTriggerDelayReported, ExecutionMode: domain.ExecutionModeRecommend, PlaybookID: &p.ID,
		Conditions: domain.ConditionGroup{Logic: "ALL", Conditions: []domain.ConditionClause{
			{Field: "eventType", Operator: "eq", Value: json.RawMessage(`"driver.delay.reported"`)},
		}},
	})
	require.NoError(t, err)
	_, err = autoRepo.SetRuleStatus(ctx, tenantID, operatorID, rule.ID, domain.RuleStatusActive)
	require.NoError(t, err)

	driverEventRepo := repository.NewDriverEventRepository(env.Pool)
	ingress := service.NewAutomationTriggerIngress(automationSvc, nil, nil)
	driverEventSvc := service.NewDriverEventService(driverEventRepo, workflowRepo, automationSvc, ingress, slog.Default())
	metrics := driverconsumer.NewMetrics()

	eventID := uuid.New()
	payload, _ := json.Marshal(map[string]any{
		"eventId": eventID.String(), "eventType": "driver.delay.reported", "schemaVersion": 1,
		"occurredAt": time.Now().UTC().Format(time.RFC3339Nano),
		"tenantId": tenantID.String(), "shipmentId": shipmentID.String(), "driverId": driverID.String(),
		"source": "driver", "sourceEventId": uuid.NewString(), "reasonCode": "TRAFFIC",
		"aggregate": map[string]any{"type": "SHIPMENT", "id": shipmentID.String(), "version": 1},
	})

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	t.Cleanup(producer.Close)
	record := &kgo.Record{
		Topic: topic, Key: []byte(shipmentID.String()), Value: payload,
		Headers: []kgo.RecordHeader{{Key: "event_type", Value: []byte("driver.delay.reported")}},
	}
	require.NoError(t, producer.ProduceSync(ctx, record).FirstErr())

	group := "ct-driver-it-" + uuid.NewString()
	consClient, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
	)
	require.NoError(t, err)
	t.Cleanup(consClient.Close)

	cfg := config.Config{
		DriverConsumer: config.DriverConsumerConfig{
			Enabled: true, ProcessTimeout: 10 * time.Second, CommitTimeout: 5 * time.Second,
			Kafka: config.DriverKafkaConfig{Brokers: brokers, Topic: topic, GroupID: group, ClientID: "ct-driver-it", DialTimeout: 5 * time.Second},
		},
	}
	consumerSvc := driverconsumer.NewService(consClient, driverEventSvc, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), metrics)

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		fetches := consClient.PollFetches(ctx)
		if fetches.Err() != nil || fetches.NumRecords() == 0 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		fetches.EachRecord(func(rec *kgo.Record) {
			consumerSvc.ProcessRecordForIntegration(ctx, rec)
		})
		break
	}

	var inboxCount int64
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.driver_event_inbox WHERE tenant_id=$1 AND event_id=$2`, tenantID, eventID).Scan(&inboxCount))
	require.Equal(t, int64(1), inboxCount)

	var recCount int64
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.automation_recommendation WHERE tenant_id=$1`, tenantID).Scan(&recCount))
	require.Equal(t, int64(1), recCount)

	var auditCount int64
	require.NoError(t, env.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.automation_audit_event WHERE tenant_id=$1`, tenantID).Scan(&auditCount))
	require.GreaterOrEqual(t, auditCount, int64(1))
}

func seedShipmentFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID, driverID, carrierID, shipmentID uuid.UUID) {
	t.Helper()
	shipper, consignee, origin, dest, orderID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO core.tenants (id,code,name) VALUES ($1,'t','Tenant')`, tenantID)
	require.NoError(t, err)
	for _, row := range []struct {
		id uuid.UUID
		typ, name string
	}{
		{carrierID, "CARRIER", "Carrier"}, {shipper, "SHIPPER", "Shipper"}, {consignee, "CONSIGNEE", "Consignee"},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO core.companies (id,tenant_id,legal_name,company_type) VALUES ($1,$2,$3,$4)`, row.id, tenantID, row.name, row.typ)
		require.NoError(t, err)
	}
	for _, loc := range []struct {
		id uuid.UUID
		name string
	}{
		{origin, "O"}, {dest, "D"},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO transport.locations (id,tenant_id,location_type,name,country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`, loc.id, tenantID, loc.name)
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO transport.transport_orders (id,tenant_id,order_number,status,shipper_company_id,consignee_company_id,origin_location_id,destination_location_id,transport_mode) VALUES ($1,$2,'TO','ASSIGNED',$3,$4,$5,$6,'ROAD')`,
		orderID, tenantID, shipper, consignee, origin, dest)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO transport.drivers (id,tenant_id,carrier_company_id,user_id,full_name,status) VALUES ($1,$2,$3,$4,'Driver','ACTIVE')`,
		driverID, tenantID, carrierID, userID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO transport.shipments (id,tenant_id,shipment_number,transport_order_id,shipper_company_id,consignee_company_id,carrier_company_id,driver_id,origin_location_id,destination_location_id,transport_mode,status,version) VALUES ($1,$2,'SHP',$3,$4,$5,$6,$7,$8,$9,'ROAD','PICKUP_SLOT_BOOKED',1)`,
		shipmentID, tenantID, orderID, shipper, consignee, carrierID, driverID, origin, dest)
	require.NoError(t, err)
}
