//go:build integration

package transportexecution

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/repository"
)

func TestShipmentRuntimeLoginFoundation(t *testing.T) {
	env := startPostgres(t)
	freightBefore := loginRoleSnapshot(t, env.pool, "freight")
	applyNumberedMigrations(t, env, 91)
	if loginRoleSnapshot(t, env.pool, "freight") != freightBefore {
		t.Fatal("applying role foundation changed the shared application login")
	}
	cycleShipmentRoleFoundation(t, env)
	if loginRoleSnapshot(t, env.pool, "freight") != freightBefore {
		t.Fatal("migration cycle changed the shared application login")
	}
	assertRuntimeNoLogin(t, env.pool, "shipment_runtime")
	if countTriggers(t, env.pool, "trg_new_activation_authorization") != 0 {
		t.Fatal("role foundation installed the enforcement trigger")
	}

	login := connectShipmentLogin(t, env.pool, "shipment_login", "shipment_runtime")
	assertShipmentSession(t, login)
	ctx := env.ctx
	graph := seedShipmentGraph(t, env)
	shipments := repository.NewShipmentRepository(login)
	transition := domain.NewUserTransitionContext(uuid.New(), nil, time.Now().UTC())
	cargoID := graph.cargoID
	created, err := shipments.CreateShipment(ctx, repository.CreateShipmentParams{
		TenantID: graph.tenantID, ShipmentNumber: "SHP-LOGIN-" + uuid.NewString()[:8],
		TransportOrderID: graph.orderID, ShipperCompanyID: graph.shipperID, ConsigneeCompanyID: graph.consigneeID,
		CarrierCompanyID: graph.carrierID, OriginLocationID: graph.originID, DestinationLocationID: graph.destID,
		CargoID: &cargoID, TransportMode: "ROAD",
	}, transition)
	if err != nil || created.TenantID != graph.tenantID || created.Status != domain.ShipmentStatusCarrierAssigned {
		t.Fatalf("create shipment: %v %+v", err, created)
	}
	evidence, err := shipments.UpdateStatusWithCargoEvidence(ctx, created.ID, graph.tenantID, created.Status, domain.ShipmentStatusAcceptedByCarrier, nil, nil, created.Version, transition, domain.CargoEvidenceIntent{
		State: domain.CargoEvidenceConfirmedOnboard, Source: domain.CargoEvidenceSourceDriver,
		SourceEventType: "PICKUP_COMPLETED", ActorID: uuid.New(), DriverID: uuid.New(), OccurredAt: time.Now().UTC(),
	})
	if err != nil || evidence.Status != domain.ShipmentStatusAcceptedByCarrier {
		t.Fatalf("cargo evidence: %v %+v", err, evidence)
	}

	driver, err := repository.NewDriverRepository(login).Create(ctx, graph.tenantID, domain.CreateDriverInput{
		CarrierCompanyID: graph.carrierID, FullName: "Login Driver", LicenseCountry: "RU", PreferredLocale: "ru-RU",
	})
	if err != nil {
		t.Fatalf("create driver: %v", err)
	}
	vehicle, err := repository.NewVehicleRepository(login).Create(ctx, graph.tenantID, domain.CreateVehicleInput{
		CarrierCompanyID: graph.carrierID, PlateNumber: "L" + uuid.NewString()[:6], VehicleType: "TRUCK", RegistrationCountry: "RU",
	})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}
	assigned, err := shipments.AssignDriver(ctx, evidence.ID, graph.tenantID, driver.ID, evidence.Status, domain.ShipmentStatusDriverAssigned, evidence.Version, transition)
	if err != nil || assigned.DriverID == nil || *assigned.DriverID != driver.ID {
		t.Fatalf("assign driver: %v %+v", err, assigned)
	}
	withVehicle, err := shipments.AssignVehicle(ctx, assigned.ID, graph.tenantID, vehicle.ID, assigned.Status, domain.ShipmentStatusDriverAssigned, assigned.Version, transition)
	if err != nil || withVehicle.VehicleID == nil || *withVehicle.VehicleID != vehicle.ID {
		t.Fatalf("assign vehicle: %v %+v", err, withVehicle)
	}

	var otherTenant uuid.UUID
	if err := env.pool.QueryRow(ctx, `SELECT tenant_id FROM transport.shipments WHERE id <> $1 LIMIT 1`, created.ID).Scan(&otherTenant); err == nil && otherTenant == graph.tenantID {
		otherTenant = uuid.Nil
	}
	foreignShipment, err := shipments.GetByIDAndTenant(ctx, created.ID, uuid.New())
	if err == nil || (foreignShipment != nil && foreignShipment.ID == created.ID) {
		t.Fatal("shipment read crossed tenants")
	}

	source := repository.NewShipmentSourceAuthorityRepository(login)
	record, err := source.ReadShipmentSource(ctx, created.ID)
	if err != nil || record.ID != created.ID || record.TenantID != graph.tenantID {
		t.Fatalf("source shipment: %v %+v", err, record)
	}
	userID := uuid.New()
	insertShipmentUser(t, env, userID, graph.tenantID)
	if _, err := env.pool.Exec(ctx, `INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status) VALUES ($1,$2,$3,'ACTIVE')`, graph.tenantID, graph.shipperID, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id)
		SELECT $1, $2, $3, id FROM core.roles WHERE tenant_id IS NULL AND code = 'SHIPPER_ADMIN' AND deleted_at IS NULL
	`, graph.tenantID, userID, graph.shipperID); err != nil {
		t.Fatal(err)
	}
	creator, err := source.ReadShipmentCreator(ctx, graph.tenantID, userID, graph.shipperID)
	if err != nil || !creator.ActiveMembership || len(creator.CompanyRoles) == 0 {
		t.Fatalf("source creator: %v %+v", err, creator)
	}

	bidID := seedBid(t, env, graph)
	bid, err := shipments.GetBid(ctx, bidID, graph.tenantID)
	if err != nil || bid.ID != bidID {
		t.Fatalf("bid read: %v %+v", err, bid)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO documents.documents (
			id, tenant_id, document_number, document_type, document_status,
			owner_company_id, related_entity_type, related_entity_id, created_at, updated_at
		) VALUES ($1,$2,$3,'POD','SIGNED',$4,'SHIPMENT',$5,now(),now())
	`, uuid.New(), graph.tenantID, "POD-"+uuid.NewString()[:8], graph.shipperID, created.ID); err != nil {
		t.Fatal(err)
	}
	docs, err := repository.NewOrderExecutionRepository(login).ListShipmentPODDocuments(ctx, graph.tenantID, created.ID)
	if err != nil || len(docs) != 1 {
		t.Fatalf("pod documents: %v %+v", err, docs)
	}
	if _, err := repository.NewPODRequirementRepository(login).LoadPODRequirement(ctx, uuid.New(), uuid.New(), uuid.New()); sqlStateOf(err) == "42501" {
		t.Fatalf("pod requirement read denied: %v", err)
	}

	tag, err := login.Exec(ctx, `
		UPDATE transport.transport_orders
		SET status = 'CANCELLED', version = version + 1, updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = 'ASSIGNED'
	`, graph.orderID, graph.tenantID)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("order status update: %v rows=%d", err, tag.RowsAffected())
	}
	expectShipmentSQLState(t, login, `
		UPDATE transport.transport_orders SET order_number = 'denied' WHERE id = $1
	`, "42501", graph.orderID)
	tx, err := login.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var locked uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM transport.transport_orders WHERE id = $1 FOR UPDATE`, graph.orderID).Scan(&locked)
	_ = tx.Rollback(ctx)
	if err != nil || locked != graph.orderID {
		t.Fatalf("order lock under column update: %v", err)
	}

	expectShipmentSQLState(t, login, `SELECT id FROM rfx.rfx_award_transport_orders WHERE false FOR UPDATE`, "42501")
	expectShipmentSQLState(t, login, `UPDATE rfx.rfx_award_transport_orders SET transport_order_id = transport_order_id WHERE false`, "42501")
	if _, err := login.Exec(ctx, `SELECT id FROM rfx.rfx_award_transport_orders WHERE false`); err != nil {
		t.Fatalf("award select: %v", err)
	}
	var functions int
	if err := login.QueryRow(ctx, `
		SELECT count(*) FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = 'rfx' AND p.proname = 'lock_award_transport_order_for_execution'
	`).Scan(&functions); err != nil || functions != 0 {
		t.Fatalf("award lock function count=%d err=%v", functions, err)
	}

	expectShipmentSQLState(t, login, `DELETE FROM transport.transport_execution_carrier_relationship_pins WHERE false`, "42501")
	expectShipmentSQLState(t, login, `DELETE FROM company.carrier_relationship_activation_authorizations WHERE false`, "42501")
	expectShipmentSQLState(t, login, `DELETE FROM company.carrier_relationship_activation_subjects WHERE false`, "42501")
	expectShipmentSQLState(t, login, `DELETE FROM transport.transport_execution_revisions WHERE false`, "42501")
	expectShipmentSQLState(t, login, `UPDATE transport.transport_execution_revisions SET pin_set_fingerprint = 'denied' WHERE false`, "42501")

	execRepo := repository.NewTransportExecutionRepository(login)
	shipA := seedShipment(t, env, "proj-a", "IN_TRANSIT")
	shipB := seedShipment(t, env, "proj-b", "IN_TRANSIT")
	operating := seedTenant(t, env, "proj-carrier")
	carrierID := seedCompany(t, env, operating, "CARRIER", "Projection Carrier")
	projected, err := execRepo.Project(ctx, routeCommand(operating, carrierID, shipA, shipB))
	if err != nil || projected.ExecutionID == uuid.Nil {
		t.Fatalf("project: %v %+v", err, projected)
	}
	route := loadSeededRoute(t, env, operating, projected.ExecutionID, projected.RevisionID, shipA, shipB)
	successor, err := execRepo.CreateSuccessorRevision(ctx, successorCommand(route, "login-successor"))
	if err != nil || successor.RevisionID == uuid.Nil || successor.RevisionID == projected.RevisionID {
		t.Fatalf("successor: %v %+v", err, successor)
	}

	seed := seedDelivery(t, env, 10, true)
	arriveStop(t, env, seed.stopID)
	dispositionRepo := repository.NewTransportExecutionRepository(login)
	partial := recordDelivery(t, dispositionRepo, env, seed, 7, 3, domain.ReasonDamage, "", "login-partial", domain.ActorKindOperator)
	if partial.CaseID == nil || partial.RejectedQuantity != 3 {
		t.Fatalf("disposition: %+v", partial)
	}
	foreign := recordCommand(seed, 0, 1, domain.ReasonDamage, "", "login-foreign", domain.ActorKindOperator, domain.UOMPallet)
	foreign.OperatingTenantID = uuid.New()
	if _, err := dispositionRepo.RecordDeliveryDisposition(ctx, foreign); reasonOf(err) != domain.ReasonTenantDenied {
		t.Fatalf("foreign disposition: %v", err)
	}

	key := "login-task-" + uuid.NewString()[:8]
	shipmentID := created.ID
	task, _, err := repository.NewDriverTaskRepository(login).CreateTask(ctx, repository.CreateDriverTaskParams{
		ShipmentVersion: withVehicle.Version,
		Task: domain.DriverTask{
			TenantID: graph.tenantID, DriverID: driver.ID, ShipmentID: &shipmentID,
			TaskType: domain.DriverTaskTypeGeneralOperationalNotice, Status: domain.DriverTaskStatusPending,
			Priority: domain.DriverTaskPriorityNormal, Title: "Login task", Payload: []byte(`{}`),
			AvailableAt: time.Now().UTC(), CreatedByType: domain.DriverTaskCreatorControlTower,
			Source: domain.DriverTaskSourceControlTower, IdempotencyKey: &key,
		},
	})
	if err != nil || task == nil {
		t.Fatalf("driver task: %v", err)
	}
	if _, err := repository.NewDriverDeviceRepository(login).RegisterDevice(ctx, repository.RegisterDeviceInput{
		TenantID: graph.tenantID, DriverID: driver.ID, Platform: "ANDROID",
		PushToken: "login-token", DeviceInstanceID: uuid.NewString(),
	}); err != nil {
		t.Fatalf("driver device: %v", err)
	}
	ops := repository.NewDriverOperationsRepository(login)
	if err := ops.CommitIdempotency(ctx, domain.DriverOperationIdempotencyRecord{
		TenantID: graph.tenantID, DriverID: driver.ID, OperationType: "STATUS", IdempotencyKey: "login-op",
		ResourceType: "SHIPMENT", ResourceID: created.ID, ResponseStatusCode: 200, ResponseBody: []byte(`{}`),
	}); err != nil {
		t.Fatalf("driver idempotency: %v", err)
	}
	now := time.Now().UTC()
	if _, _, err := ops.ReportException(ctx, repository.ReportDriverExceptionParams{
		ShipmentVersion: withVehicle.Version,
		Exception: domain.DriverReportedException{
			TenantID: graph.tenantID, ShipmentID: created.ID, DriverID: driver.ID, Category: "ACCIDENT",
			OccurredAt: now, ReceivedAt: now, Source: domain.DriverExceptionSource, IdempotencyKey: "login-ex",
		},
	}); err != nil {
		t.Fatalf("driver exception: %v", err)
	}
	if _, _, err := ops.ReportDelay(ctx, repository.ReportDriverDelayParams{
		ShipmentVersion: withVehicle.Version,
		Delay: domain.DriverReportedDelay{
			TenantID: graph.tenantID, ShipmentID: created.ID, DriverID: driver.ID, ReasonCode: "TRAFFIC",
			OccurredAt: now, ReceivedAt: now, IdempotencyKey: "login-delay",
		},
	}); err != nil {
		t.Fatalf("driver delay: %v", err)
	}
}

type shipmentGraph struct {
	tenantID, shipperID, consigneeID, carrierID uuid.UUID
	originID, destID, cargoID, orderID          uuid.UUID
}

func seedShipmentGraph(t *testing.T, env *execEnv) shipmentGraph {
	t.Helper()
	tenantID := seedTenant(t, env, "login")
	graph := shipmentGraph{
		tenantID:    tenantID,
		shipperID:   seedCompany(t, env, tenantID, "SHIPPER", "Login shipper"),
		consigneeID: seedCompany(t, env, tenantID, "CONSIGNEE", "Login consignee"),
		carrierID:   seedCompany(t, env, tenantID, "CARRIER", "Login carrier"),
		originID:    uuid.New(), destID: uuid.New(), cargoID: uuid.New(), orderID: uuid.New(),
	}
	for _, row := range []struct {
		id   uuid.UUID
		name string
	}{{graph.originID, "Origin"}, {graph.destID, "Destination"}} {
		if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`, row.id, tenantID, row.name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description) VALUES ($1,$2,'GENERAL','Cargo')`, graph.cargoID, tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_orders (
			id, tenant_id, order_number, status, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode
		) VALUES ($1,$2,$3,'ASSIGNED',$4,$5,$6,$7,$8,'ROAD')
	`, graph.orderID, tenantID, "TO-"+graph.orderID.String()[:8], graph.shipperID, graph.consigneeID, graph.originID, graph.destID, graph.cargoID); err != nil {
		t.Fatal(err)
	}
	return graph
}

func seedBid(t *testing.T, env *execEnv, graph shipmentGraph) uuid.UUID {
	t.Helper()
	requestID := uuid.New()
	bidID := uuid.New()
	deadline := time.Now().UTC().Add(48 * time.Hour)
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO rfx.freight_requests (
			id, tenant_id, transport_order_id, freight_request_number, request_type, shipper_company_id, status, response_deadline, currency_code
		) VALUES ($1,$2,$3,$4,'MINI_TENDER',$5,'AWARDED',$6,'RUB')
	`, requestID, graph.tenantID, graph.orderID, "FR-"+requestID.String()[:8], graph.shipperID, deadline); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO rfx.bids (
			id, tenant_id, freight_request_id, carrier_company_id, bid_number, status, total_amount, currency_code, total_amount_with_vat
		) VALUES ($1,$2,$3,$4,$5,'ACCEPTED',100,'RUB',120)
	`, bidID, graph.tenantID, requestID, graph.carrierID, "BID-"+bidID.String()[:8]); err != nil {
		t.Fatal(err)
	}
	return bidID
}

func insertShipmentUser(t *testing.T, env *execEnv, id, tenant uuid.UUID) {
	t.Helper()
	_, err := env.pool.Exec(env.ctx, `
		INSERT INTO core.users (id, tenant_id, email, full_name, status)
		VALUES ($1, $2, $3, 'Login User', 'ACTIVE')
	`, id, tenant, id.String()+"@example.test")
	if err != nil {
		t.Fatal(err)
	}
}

func loadSeededRoute(t *testing.T, env *execEnv, operating, executionID, revisionID uuid.UUID, shipA, shipB seededShipment) seededRoute {
	t.Helper()
	rows, err := env.pool.Query(env.ctx, `
		SELECT s.id, s.stop_role
		FROM transport.transport_execution_stops s
		WHERE s.execution_id = $1
		ORDER BY s.ordinal
	`, executionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var startID, cargoID, endID uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		var role string
		if err := rows.Scan(&id, &role); err != nil {
			t.Fatal(err)
		}
		switch role {
		case "START":
			startID = id
		case "CARGO":
			cargoID = id
		case "END":
			endID = id
		}
	}
	var actionA, actionB uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT id FROM transport.transport_execution_actions WHERE execution_stop_id=$1 AND shipment_id=$2`, cargoID, shipA.shipmentID).Scan(&actionA); err != nil {
		t.Fatal(err)
	}
	if err := env.pool.QueryRow(env.ctx, `SELECT id FROM transport.transport_execution_actions WHERE execution_stop_id=$1 AND shipment_id=$2`, cargoID, shipB.shipmentID).Scan(&actionB); err != nil {
		t.Fatal(err)
	}
	return seededRoute{
		operating: operating, executionID: executionID, revisionID: revisionID,
		shipA: shipA, shipB: shipB, startID: startID, cargoID: cargoID, endID: endID, actionA: actionA, actionB: actionB,
	}
}

func applyNumberedMigrations(t *testing.T, env *execEnv, min int) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(env.migrations, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, file := range files {
		if migrationNumber(filepath.Base(file)) < min {
			continue
		}
		if err := execSQLFile(env.ctx, env.pool, file); err != nil {
			t.Fatalf("%s: %v", filepath.Base(file), err)
		}
	}
}

func cycleShipmentRoleFoundation(t *testing.T, env *execEnv) {
	t.Helper()
	down := filepath.Join(env.migrations, "000105_platform_postgres_role_foundation_v0_2_i1.down.sql")
	up := filepath.Join(env.migrations, "000105_platform_postgres_role_foundation_v0_2_i1.up.sql")
	for i := 0; i < 2; i++ {
		if err := execSQLFile(env.ctx, env.pool, down); err != nil {
			t.Fatal(err)
		}
		if shipmentInsertGranted(t, env.pool) {
			t.Fatal("down left shipment insert")
		}
		if err := execSQLFile(env.ctx, env.pool, up); err != nil {
			t.Fatal(err)
		}
	}
	if !shipmentInsertGranted(t, env.pool) {
		t.Fatal("up/down/up did not restore shipment insert")
	}
}

func shipmentInsertGranted(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var ok bool
	if err := pool.QueryRow(context.Background(), `SELECT has_table_privilege('shipment_runtime', 'transport.shipments', 'INSERT')`).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

type loginRole struct {
	exists bool
	login  bool
	super  bool
	bypass bool
}

func loginRoleSnapshot(t *testing.T, pool *pgxpool.Pool, name string) loginRole {
	t.Helper()
	var snap loginRole
	err := pool.QueryRow(context.Background(), `SELECT rolcanlogin, rolsuper, rolbypassrls FROM pg_roles WHERE rolname = $1`, name).Scan(&snap.login, &snap.super, &snap.bypass)
	if errors.Is(err, pgx.ErrNoRows) {
		return loginRole{}
	}
	if err != nil {
		t.Fatal(err)
	}
	snap.exists = true
	return snap
}

func assertRuntimeNoLogin(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()
	snap := loginRoleSnapshot(t, pool, name)
	if !snap.exists || snap.login || snap.super || snap.bypass {
		t.Fatalf("%s %+v", name, snap)
	}
}

func countTriggers(t *testing.T, pool *pgxpool.Pool, name string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_trigger WHERE tgname = $1 AND NOT tgisinternal`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func connectShipmentLogin(t *testing.T, admin *pgxpool.Pool, loginName, runtimeName string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	password := strings.ReplaceAll(uuid.NewString(), "-", "")
	var stmt string
	if err := admin.QueryRow(ctx, `
		SELECT format(
			'CREATE ROLE %I LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD %L',
			$1::text, $2::text)
	`, loginName, password).Scan(&stmt); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, stmt); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, fmt.Sprintf("GRANT %s TO %s", runtimeName, loginName)); err != nil {
		t.Fatal(err)
	}
	var db string
	if err := admin.QueryRow(ctx, `SELECT current_database()`).Scan(&db); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", quoteSQLIdent(db), loginName)); err != nil {
		t.Fatal(err)
	}
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = loginName
	cfg.ConnConfig.Password = password
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP OWNED BY "+loginName)
		_, _ = admin.Exec(context.Background(), "DROP ROLE IF EXISTS "+loginName)
	})
	return pool
}

func quoteSQLIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func assertShipmentSession(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var current string
	var super, bypass bool
	if err := pool.QueryRow(context.Background(), `
		SELECT current_user, rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user
	`).Scan(&current, &super, &bypass); err != nil {
		t.Fatal(err)
	}
	if current != "shipment_login" || super || bypass {
		t.Fatalf("session current=%s super=%v bypass=%v", current, super, bypass)
	}
}

func expectShipmentSQLState(t *testing.T, pool *pgxpool.Pool, query, code string, args ...any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), query, args...)
	if sqlStateOf(err) != code {
		t.Fatalf("query %s state=%s err=%v", query, sqlStateOf(err), err)
	}
}

func sqlStateOf(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
