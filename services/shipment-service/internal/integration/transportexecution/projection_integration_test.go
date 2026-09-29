//go:build integration

package transportexecution

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

func TestTransportExecutionFoundation(t *testing.T) {
	env := startPostgres(t)
	t.Run("migration up down up", func(t *testing.T) {
		if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, "000089_tms_transport_execution_commands_v0_1b.down.sql")); err != nil {
			t.Fatal(err)
		}
		if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, "000088_tms_transport_execution_foundation_v0_1a.down.sql")); err != nil {
			t.Fatal(err)
		}
		if tableExists(t, env, "transport_executions") {
			t.Fatal("down left transport_executions")
		}
		if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, "000088_tms_transport_execution_foundation_v0_1a.up.sql")); err != nil {
			t.Fatal(err)
		}
		if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, "000088_tms_transport_execution_foundation_v0_1a.down.sql")); err != nil {
			t.Fatal(err)
		}
		if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, "000088_tms_transport_execution_foundation_v0_1a.up.sql")); err != nil {
			t.Fatal(err)
		}
		if err := execSQLFile(env.ctx, env.pool, filepath.Join(env.migrations, "000089_tms_transport_execution_commands_v0_1b.up.sql")); err != nil {
			t.Fatal(err)
		}
		if !tableExists(t, env, "transport_executions") {
			t.Fatal("up did not create transport_executions")
		}
		if !relationExists(t, env, "documents", "document_packages") {
			t.Fatal("000088 down removed EDO document_packages")
		}
		if !relationExists(t, env, "network_optimizer", "route_plan_activations") {
			t.Fatal("000088 down removed NLO route_plan_activations")
		}
	})

	t.Run("schema ownership", func(t *testing.T) {
		assertColumns(t, env, "transport_executions", []string{"id", "operating_tenant_id", "carrier_company_id"}, []string{"shipment_id"})
		assertColumns(t, env, "transport_execution_stops", []string{"execution_id"}, []string{"execution_revision_id", "revision_id", "source_route_plan_stop_id"})
		assertColumns(t, env, "transport_execution_actions", []string{"execution_stop_id"}, []string{"execution_revision_id", "revision_id", "source_route_plan_action_id"})
		assertColumns(t, env, "transport_execution_revision_stops", []string{"revision_id", "stop_id", "source_route_plan_stop_id", "membership"}, nil)
		assertColumns(t, env, "transport_execution_revision_actions", []string{"revision_id", "action_id", "source_route_plan_action_id", "membership"}, nil)
	})

	t.Run("one route many shippers", func(t *testing.T) {
		operating := seedTenant(t, env, "carrier")
		carrierID := seedCompany(t, env, operating, "CARRIER", "Carrier C")
		shipperA := seedShipment(t, env, "shipper-a", "IN_TRANSIT")
		shipperB := seedShipment(t, env, "shipper-b", "IN_TRANSIT")
		idle := seedAssignedShipment(t, env, shipperA.tenantID, shipperA.shipperID, shipperA.consigneeID)
		statusA := shipmentStatus(t, env, shipperA.shipmentID, shipperA.tenantID)
		statusB := shipmentStatus(t, env, shipperB.shipmentID, shipperB.tenantID)
		tenantA := shipmentTenant(t, env, shipperA.shipmentID)
		tenantB := shipmentTenant(t, env, shipperB.shipmentID)

		cmd := routeCommand(operating, carrierID, shipperA, shipperB)
		first, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		if !first.Created {
			t.Fatal("expected a new execution")
		}
		second, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		if second.Created || second.ExecutionID != first.ExecutionID || second.RevisionID != first.RevisionID {
			t.Fatalf("replay = %+v first = %+v", second, first)
		}
		if countWhere(t, env, "transport.transport_executions", "operating_tenant_id=$1", operating) != 1 {
			t.Fatal("execution count")
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "execution_id=$1", first.ExecutionID) != 1 {
			t.Fatal("revision count")
		}
		if countWhere(t, env, "transport.transport_execution_participants", "execution_id=$1", first.ExecutionID) != 2 {
			t.Fatal("participant count")
		}
		if shipmentTenant(t, env, shipperA.shipmentID) != tenantA || shipmentTenant(t, env, shipperB.shipmentID) != tenantB {
			t.Fatal("shipment owner tenant changed")
		}
		if shipmentStatus(t, env, shipperA.shipmentID, shipperA.tenantID) != statusA || shipmentStatus(t, env, shipperB.shipmentID, shipperB.tenantID) != statusB {
			t.Fatal("projection changed shipment status")
		}
		if countWhere(t, env, "transport.shipments", "tenant_id=$1", operating) != 0 {
			t.Fatal("shipment was created in the carrier tenant")
		}

		var stopID, linkStop, source uuid.UUID
		err = env.pool.QueryRow(env.ctx, `
			SELECT stop.id, link.stop_id, link.source_route_plan_stop_id
			FROM transport.transport_execution_stops AS stop
			JOIN transport.transport_execution_revision_stops AS link ON link.stop_id = stop.id
			WHERE stop.execution_id = $1 AND link.revision_id = $2 AND link.membership = 'INTRODUCED'
			ORDER BY stop.ordinal
			LIMIT 1
		`, first.ExecutionID, first.RevisionID).Scan(&stopID, &linkStop, &source)
		if err != nil {
			t.Fatal(err)
		}
		if linkStop != stopID || source == uuid.Nil {
			t.Fatalf("stop link stop=%s link=%s source=%s", stopID, linkStop, source)
		}

		var actionID, linkAction, sourceAction uuid.UUID
		var actionShipment uuid.UUID
		err = env.pool.QueryRow(env.ctx, `
			SELECT action.id, link.action_id, link.source_route_plan_action_id, action.shipment_id
			FROM transport.transport_execution_actions AS action
			JOIN transport.transport_execution_revision_actions AS link ON link.action_id = action.id
			WHERE link.revision_id = $1
			ORDER BY action.ordinal
		`, first.RevisionID).Scan(&actionID, &linkAction, &sourceAction, &actionShipment)
		if err != nil {
			t.Fatal(err)
		}
		if linkAction != actionID || sourceAction == uuid.Nil {
			t.Fatal("action lineage is not on the link")
		}
		if countWhere(t, env, "transport.transport_execution_actions", "shipment_id=$1", shipperA.shipmentID) != 1 {
			t.Fatal("missing shipment A action")
		}
		if countWhere(t, env, "transport.transport_execution_actions", "shipment_id=$1", shipperB.shipmentID) != 1 {
			t.Fatal("missing shipment B action")
		}

		changed := cmd
		changed.Stops[1].Latitude = 1.23
		_, err = env.svc.CreateExecutionProjectionFromActivation(env.ctx, changed)
		if reason(err) != domain.ReasonActivationBodyConflict {
			t.Fatalf("body conflict reason=%s err=%v", reason(err), err)
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "execution_id=$1", first.ExecutionID) != 1 {
			t.Fatal("body conflict created a revision")
		}

		if _, err := env.pool.Exec(env.ctx, `
			UPDATE transport.transport_execution_stops
			SET status = 'COMPLETED', completed_at = now(), updated_at = now()
			WHERE id = $1
		`, stopID); err != nil {
			t.Fatal(err)
		}
		var completedID, parent uuid.UUID
		if err := env.pool.QueryRow(env.ctx, `SELECT id, execution_id FROM transport.transport_execution_stops WHERE id=$1`, stopID).Scan(&completedID, &parent); err != nil {
			t.Fatal(err)
		}
		if completedID != stopID || parent != first.ExecutionID {
			t.Fatal("completed stop identity changed")
		}
		otherExecution := uuid.New()
		if _, err := env.pool.Exec(env.ctx, `
			INSERT INTO transport.transport_executions (id, operating_tenant_id, carrier_company_id, created_at, updated_at)
			VALUES ($1,$2,$3,now(),now())
		`, otherExecution, operating, carrierID); err != nil {
			t.Fatal(err)
		}
		_, err = env.pool.Exec(env.ctx, `UPDATE transport.transport_execution_stops SET execution_id=$2 WHERE id=$1`, stopID, otherExecution)
		if err == nil || !strings.Contains(err.Error(), "STOP_PARENT_SET_ONCE") {
			t.Fatalf("reparent err=%v", err)
		}
		if err := env.pool.QueryRow(env.ctx, `SELECT execution_id FROM transport.transport_execution_stops WHERE id=$1`, stopID).Scan(&parent); err != nil {
			t.Fatal(err)
		}
		if parent != first.ExecutionID {
			t.Fatal("stop parent changed")
		}

		_, err = env.pool.Exec(env.ctx, `
			INSERT INTO transport.transport_execution_revisions (
				id, execution_id, operating_tenant_id, source_route_plan_id, source_route_plan_version,
				source_activation_id, source_activation_version, evaluation_fingerprint, planning_mode,
				status, version, contract_sha256, created_at, updated_at
			) VALUES ($1,$2,$3,$4,1,$5,1,'fp','CURRENT_TRIP','ACTIVE',1,$6,now(),now())
		`, uuid.New(), first.ExecutionID, operating, uuid.New(), uuid.New(), strings.Repeat("ab", 32))
		if !uniqueViolation(err) {
			t.Fatalf("second active revision err=%v", err)
		}

		_, err = env.pool.Exec(env.ctx, `
			INSERT INTO transport.transport_execution_revisions (
				id, execution_id, operating_tenant_id, source_route_plan_id, source_route_plan_version,
				source_activation_id, source_activation_version, evaluation_fingerprint, planning_mode,
				status, version, contract_sha256, created_at, updated_at
			) VALUES ($1,$2,$3,$4,1,$5,1,'fp','CURRENT_TRIP','SUPERSEDED',1,$6,now(),now())
		`, uuid.New(), first.ExecutionID, operating, uuid.New(), cmd.ActivationID, strings.Repeat("cd", 32))
		if !uniqueViolation(err) {
			t.Fatalf("duplicate activation err=%v", err)
		}

		overlap := routeCommand(operating, carrierID, shipperA, shipperB)
		overlap.ActivationID = uuid.New()
		_, err = env.svc.CreateExecutionProjectionFromActivation(env.ctx, overlap)
		if reason(err) != domain.ReasonExecutionPlanConflict {
			t.Fatalf("second route reason=%s err=%v", reason(err), err)
		}
		if countWhere(t, env, "transport.transport_executions", "operating_tenant_id=$1 AND id<>$2", operating, otherExecution) != 1 {
			t.Fatal("conflict created another execution")
		}

		updated, err := env.shipments.UpdateStatus(env.ctx, idle.tenantID, idle.shipmentID, domain.UpdateShipmentStatusInput{
			Status: domain.ShipmentStatusPickupSlotBooked,
		}, domain.NewUserTransitionContext(uuid.New(), nil, time.Now().UTC()))
		if err != nil {
			t.Fatal(err)
		}
		if updated.Status != domain.ShipmentStatusPickupSlotBooked {
			t.Fatalf("single-leg status=%s", updated.Status)
		}
		if shipmentStatus(t, env, shipperA.shipmentID, shipperA.tenantID) != statusA {
			t.Fatal("participant status changed with the unrelated shipment")
		}
	})

	t.Run("depot start has no fabricated shipment", func(t *testing.T) {
		operating := seedTenant(t, env, "depot-carrier")
		carrierID := seedCompany(t, env, operating, "CARRIER", "Depot Carrier")
		location := uuid.New()
		duration := 300
		cmd := domain.ProjectionCommand{
			ActivationID:          uuid.New(),
			ActivationVersion:     1,
			ActivationStatus:      domain.ActivationStatusPendingExecution,
			RoutePlanID:           uuid.New(),
			RoutePlanVersion:      1,
			PlanningMode:          domain.PlanningModeDepotStart,
			OperatingTenantID:     operating,
			CarrierCompanyID:      carrierID,
			EvaluationFingerprint: "depot-fp",
			Stops: []domain.ProjectionStop{
				{RoutePlanStopID: uuid.New(), Ordinal: 0, StopRole: domain.StopRoleStart, PointKind: domain.PointKindPositionAnchor, Latitude: 55.7, Longitude: 37.6},
				{RoutePlanStopID: uuid.New(), Ordinal: 1, StopRole: domain.StopRoleEnd, PointKind: domain.PointKindCanonicalLocation, LocationID: &location, Latitude: 55.9, Longitude: 37.8, ServiceDurationSeconds: &duration},
			},
		}
		result, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if err != nil {
			t.Fatal(err)
		}
		if countWhere(t, env, "transport.transport_execution_participants", "execution_id=$1", result.ExecutionID) != 0 {
			t.Fatal("depot start created a participant")
		}
		if countWhere(t, env, "transport.shipments", "tenant_id=$1", operating) != 0 {
			t.Fatal("depot start fabricated a shipment")
		}
	})

	t.Run("unmaterialized subject writes nothing", func(t *testing.T) {
		operating := seedTenant(t, env, "partial-carrier")
		carrierID := seedCompany(t, env, operating, "CARRIER", "Partial Carrier")
		shipperA := seedShipment(t, env, "partial-a", "IN_TRANSIT")
		missingCargo := uuid.New()
		version := 1
		stopID := uuid.New()
		cmd := domain.ProjectionCommand{
			ActivationID:            uuid.New(),
			ActivationVersion:       1,
			ActivationStatus:        domain.ActivationStatusPendingExecution,
			RoutePlanID:             uuid.New(),
			RoutePlanVersion:        1,
			PlanningMode:            domain.PlanningModeCurrentTrip,
			OperatingTenantID:       operating,
			ContextShipmentID:       &shipperA.shipmentID,
			ContextShipmentTenantID: &shipperA.tenantID,
			ContextShipmentVersion:  &version,
			CarrierCompanyID:        carrierID,
			EvaluationFingerprint:   "partial-fp",
			ExecutionSubjects: []domain.ProjectionSubject{
				materialSubject(shipperA, version),
				{
					RouteSubjectType:         domain.RouteSubjectLoadOpportunity,
					RouteSubjectID:           uuid.New(),
					RouteSubjectVersion:      1,
					ExecutionShipmentID:      &shipperA.shipmentID,
					ShipmentTenantID:         &shipperA.tenantID,
					ExecutionShipmentVersion: &version,
					CargoID:                  &missingCargo,
					CargoVersion:             &version,
				},
			},
			Stops: []domain.ProjectionStop{
				{RoutePlanStopID: stopID, Ordinal: 1, StopRole: domain.StopRoleCargo, PointKind: domain.PointKindCanonicalLocation, LocationID: &shipperA.originID, Latitude: 1, Longitude: 2},
			},
			Actions: []domain.ProjectionAction{
				materialAction(stopID, shipperA, version, 0),
			},
		}
		cmd.Actions[0].CargoID = &missingCargo
		cmd.Actions[0].RouteSubjectID = cmd.ExecutionSubjects[1].RouteSubjectID
		cmd.Actions[0].RouteSubjectType = domain.RouteSubjectLoadOpportunity
		_, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if reason(err) != domain.ReasonExecutionSubjectUnmaterialized {
			t.Fatalf("reason=%s err=%v", reason(err), err)
		}
		if countWhere(t, env, "transport.transport_executions", "operating_tenant_id=$1", operating) != 0 {
			t.Fatal("partial execution was committed")
		}
		if countWhere(t, env, "transport.transport_execution_stops", "operating_tenant_id=$1", operating) != 0 {
			t.Fatal("partial stops were committed")
		}
	})

	t.Run("foreign shipment tenant is not a scan", func(t *testing.T) {
		operating := seedTenant(t, env, "foreign-carrier")
		carrierID := seedCompany(t, env, operating, "CARRIER", "Foreign Carrier")
		owner := seedShipment(t, env, "foreign-owner", "CREATED")
		before := shipmentTenant(t, env, owner.shipmentID)
		version := 1
		stopID := uuid.New()
		cmd := domain.ProjectionCommand{
			ActivationID:            uuid.New(),
			ActivationVersion:       1,
			ActivationStatus:        domain.ActivationStatusPendingExecution,
			RoutePlanID:             uuid.New(),
			RoutePlanVersion:        1,
			PlanningMode:            domain.PlanningModeCurrentTrip,
			OperatingTenantID:       operating,
			ContextShipmentID:       &owner.shipmentID,
			ContextShipmentTenantID: &operating,
			ContextShipmentVersion:  &version,
			CarrierCompanyID:        carrierID,
			EvaluationFingerprint:   "foreign-fp",
			ExecutionSubjects: []domain.ProjectionSubject{{
				RouteSubjectType:         domain.RouteSubjectShipmentCargo,
				RouteSubjectID:           uuid.New(),
				RouteSubjectVersion:      1,
				ExecutionShipmentID:      &owner.shipmentID,
				ShipmentTenantID:         &operating,
				ExecutionShipmentVersion: &version,
				CargoID:                  &owner.cargoID,
				CargoVersion:             &version,
			}},
			Stops: []domain.ProjectionStop{
				{RoutePlanStopID: stopID, Ordinal: 1, StopRole: domain.StopRoleCargo, PointKind: domain.PointKindCanonicalLocation, LocationID: &owner.originID, Latitude: 1, Longitude: 2},
			},
		}
		cmd.Actions = []domain.ProjectionAction{materialAction(stopID, owner, version, 0)}
		cmd.Actions[0].ShipmentTenantID = &operating
		cmd.Actions[0].RouteSubjectID = cmd.ExecutionSubjects[0].RouteSubjectID
		_, err := env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
		if reason(err) != domain.ReasonExecutionSubjectUnmaterialized {
			t.Fatalf("reason=%s err=%v", reason(err), err)
		}
		if shipmentTenant(t, env, owner.shipmentID) != before {
			t.Fatal("foreign lookup rehomed the shipment")
		}
		if countWhere(t, env, "transport.transport_executions", "operating_tenant_id=$1", operating) != 0 {
			t.Fatal("foreign lookup created an execution")
		}
	})

	t.Run("foreign key denies dangling stop", func(t *testing.T) {
		_, err := env.pool.Exec(env.ctx, `
			INSERT INTO transport.transport_execution_stops (
				id, execution_id, operating_tenant_id, ordinal, stop_role, point_kind,
				latitude, longitude, status, version, created_at, updated_at
			) VALUES ($1,$2,$3,0,'START','POSITION_ANCHOR',1,2,'PLANNED',1,now(),now())
		`, uuid.New(), uuid.New(), uuid.New())
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
			t.Fatalf("fk err=%v", err)
		}
	})

	t.Run("concurrent same activation", func(t *testing.T) {
		operating := seedTenant(t, env, "race-carrier")
		carrierID := seedCompany(t, env, operating, "CARRIER", "Race Carrier")
		shipperA := seedShipment(t, env, "race-a", "IN_TRANSIT")
		shipperB := seedShipment(t, env, "race-b", "IN_TRANSIT")
		cmd := routeCommand(operating, carrierID, shipperA, shipperB)
		var wg sync.WaitGroup
		results := make([]domain.ProjectionResult, 2)
		errs := make([]error, 2)
		wg.Add(2)
		for i := range results {
			go func(i int) {
				defer wg.Done()
				results[i], errs[i] = env.svc.CreateExecutionProjectionFromActivation(env.ctx, cmd)
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("caller %d: %v", i, err)
			}
		}
		if results[0].ExecutionID != results[1].ExecutionID || results[0].RevisionID != results[1].RevisionID {
			t.Fatalf("concurrent activation diverged: %+v %+v", results[0], results[1])
		}
		created := 0
		for _, result := range results {
			if result.Created {
				created++
			}
		}
		if created != 1 {
			t.Fatalf("created count=%d", created)
		}
		if countWhere(t, env, "transport.transport_execution_revisions", "execution_id=$1", results[0].ExecutionID) != 1 {
			t.Fatal("concurrent activation created two revisions")
		}
	})
}

type execEnv struct {
	ctx        context.Context
	pool       *pgxpool.Pool
	svc        *service.TransportExecutionService
	shipments  *service.ShipmentService
	migrations string
}

type seededShipment struct {
	tenantID    uuid.UUID
	shipperID   uuid.UUID
	consigneeID uuid.UUID
	originID    uuid.UUID
	cargoID     uuid.UUID
	shipmentID  uuid.UUID
	subjectID   uuid.UUID
}

func startPostgres(t *testing.T) *execEnv {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	t.Cleanup(cancel)
	port := freePort(t)
	runtimePath := filepath.Join(t.TempDir(), "pg-runtime")
	dataPath := filepath.Join(t.TempDir(), "pg-data")
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		RuntimePath(runtimePath).
		DataPath(dataPath).
		Database("freight_mstop_test").
		Username("freight").
		Password("freight").
		Version(embeddedpostgres.V16))
	if err := pg.Start(); err != nil {
		t.Fatalf("start embedded postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Stop() })
	adminURL := fmt.Sprintf("postgres://freight:freight@127.0.0.1:%d/postgres?sslmode=disable", port)
	dbName := "mstop_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	testURL, cleanup := createDatabase(t, ctx, adminURL, dbName)
	t.Cleanup(func() { cleanup() })
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	migrations, err := locateMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if err := applySelectedMigrations(ctx, pool, migrations); err != nil {
		t.Fatal(err)
	}
	shipmentRepo := repository.NewShipmentRepository(pool)
	return &execEnv{
		ctx:        ctx,
		pool:       pool,
		svc:        service.NewTransportExecutionService(repository.NewTransportExecutionRepository(pool)),
		shipments:  service.NewShipmentService(shipmentRepo, repository.NewDriverRepository(pool), repository.NewVehicleRepository(pool)),
		migrations: migrations,
	}
}

func applySelectedMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		base := filepath.Base(file)
		num := migrationNumber(base)
		if num > 89 {
			continue
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("%s: %w", base, err)
		}
	}
	return nil
}

func migrationNumber(filename string) int {
	var n int
	fmt.Sscanf(filename, "%d", &n)
	return n
}

func locateMigrations() (string, error) {
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	return "", os.ErrNotExist
}

func execSQLFile(ctx context.Context, pool *pgxpool.Pool, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, string(content))
	return err
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func createDatabase(t *testing.T, ctx context.Context, adminURL, dbName string) (string, func()) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	testURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		testCfg.ConnConfig.User, testCfg.ConnConfig.Password, testCfg.ConnConfig.Host, testCfg.ConnConfig.Port, dbName)
	return testURL, func() {
		cadmin, err := pgxpool.NewWithConfig(context.Background(), adminCfg)
		if err != nil {
			return
		}
		defer cadmin.Close()
		_, _ = cadmin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	}
}

func seedTenant(t *testing.T, env *execEnv, prefix string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := env.pool.Exec(env.ctx, `INSERT INTO core.tenants (id, code, name) VALUES ($1,$2,$3)`,
		id, prefix+"-"+id.String()[:8], prefix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func seedCompany(t *testing.T, env *execEnv, tenantID uuid.UUID, kind, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := env.pool.Exec(env.ctx, `INSERT INTO core.companies (id, tenant_id, legal_name, company_type) VALUES ($1,$2,$3,$4)`,
		id, tenantID, name, kind)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func seedShipment(t *testing.T, env *execEnv, prefix, status string) seededShipment {
	t.Helper()
	tenantID := seedTenant(t, env, prefix)
	shipperID := seedCompany(t, env, tenantID, "SHIPPER", prefix+" shipper")
	consigneeID := seedCompany(t, env, tenantID, "CONSIGNEE", prefix+" consignee")
	originID := uuid.New()
	destID := uuid.New()
	for _, row := range []struct {
		id   uuid.UUID
		name string
	}{{originID, "Origin"}, {destID, "Destination"}} {
		if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`,
			row.id, tenantID, row.name); err != nil {
			t.Fatal(err)
		}
	}
	cargoID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.cargoes (id, tenant_id, cargo_type, description) VALUES ($1,$2,'GENERAL','Cargo')`, cargoID, tenantID); err != nil {
		t.Fatal(err)
	}
	orderID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_orders (id, tenant_id, order_number, status, shipper_company_id, consignee_company_id, origin_location_id, destination_location_id, transport_mode)
		VALUES ($1,$2,$3,'ASSIGNED',$4,$5,$6,$7,'ROAD')
	`, orderID, tenantID, "TO-"+orderID.String()[:8], shipperID, consigneeID, originID, destID); err != nil {
		t.Fatal(err)
	}
	shipmentID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
			origin_location_id, destination_location_id, cargo_id, transport_mode, status, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ROAD',$10,1)
	`, shipmentID, tenantID, "SHP-"+shipmentID.String()[:8], orderID, shipperID, consigneeID, originID, destID, cargoID, status); err != nil {
		t.Fatal(err)
	}
	return seededShipment{
		tenantID: tenantID, shipperID: shipperID, consigneeID: consigneeID,
		originID: originID, cargoID: cargoID, shipmentID: shipmentID, subjectID: uuid.New(),
	}
}

func seedAssignedShipment(t *testing.T, env *execEnv, tenantID, shipperID, consigneeID uuid.UUID) seededShipment {
	t.Helper()
	carrierID := seedCompany(t, env, tenantID, "CARRIER", "Idle Carrier")
	originID := uuid.New()
	destID := uuid.New()
	for _, row := range []struct {
		id   uuid.UUID
		name string
	}{{originID, "Idle origin"}, {destID, "Idle destination"}} {
		if _, err := env.pool.Exec(env.ctx, `INSERT INTO transport.locations (id, tenant_id, location_type, name, country_code) VALUES ($1,$2,'WAREHOUSE',$3,'RU')`, row.id, tenantID, row.name); err != nil {
			t.Fatal(err)
		}
	}
	orderID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.transport_orders (id, tenant_id, order_number, status, shipper_company_id, consignee_company_id, origin_location_id, destination_location_id, transport_mode)
		VALUES ($1,$2,$3,'ASSIGNED',$4,$5,$6,$7,'ROAD')
	`, orderID, tenantID, "TO-"+orderID.String()[:8], shipperID, consigneeID, originID, destID); err != nil {
		t.Fatal(err)
	}
	driverID := uuid.New()
	vehicleID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.drivers (id, tenant_id, carrier_company_id, full_name, status)
		VALUES ($1,$2,$3,'Idle Driver','ACTIVE')
	`, driverID, tenantID, carrierID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.vehicles (id, tenant_id, carrier_company_id, plate_number, vehicle_type, status)
		VALUES ($1,$2,$3,$4,'TRUCK','ACTIVE')
	`, vehicleID, tenantID, carrierID, "P-"+vehicleID.String()[:8]); err != nil {
		t.Fatal(err)
	}
	shipmentID := uuid.New()
	if _, err := env.pool.Exec(env.ctx, `
		INSERT INTO transport.shipments (
			id, tenant_id, shipment_number, transport_order_id, shipper_company_id, consignee_company_id,
			carrier_company_id, driver_id, vehicle_id, origin_location_id, destination_location_id,
			transport_mode, status, version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'ROAD','DRIVER_ASSIGNED',1)
	`, shipmentID, tenantID, "SHP-"+shipmentID.String()[:8], orderID, shipperID, consigneeID, carrierID, driverID, vehicleID, originID, destID); err != nil {
		t.Fatal(err)
	}
	return seededShipment{tenantID: tenantID, shipperID: shipperID, consigneeID: consigneeID, originID: originID, shipmentID: shipmentID}
}

func routeCommand(operating, carrierID uuid.UUID, a, b seededShipment) domain.ProjectionCommand {
	version := 1
	start := uuid.New()
	cargoStop := uuid.New()
	end := uuid.New()
	duration := 900
	return domain.ProjectionCommand{
		ActivationID:            uuid.New(),
		ActivationVersion:       2,
		ActivationStatus:        domain.ActivationStatusPendingExecution,
		RoutePlanID:             uuid.New(),
		RoutePlanVersion:        4,
		PlanningMode:            domain.PlanningModeCurrentTrip,
		OperatingTenantID:       operating,
		ContextShipmentID:       &a.shipmentID,
		ContextShipmentTenantID: &a.tenantID,
		ContextShipmentVersion:  &version,
		CarrierCompanyID:        carrierID,
		EvaluationFingerprint:   "route-fingerprint",
		ExecutionSubjects: []domain.ProjectionSubject{
			materialSubject(a, version),
			materialSubject(b, version),
		},
		Stops: []domain.ProjectionStop{
			{RoutePlanStopID: start, Ordinal: 0, StopRole: domain.StopRoleStart, PointKind: domain.PointKindPositionAnchor, Latitude: 55.75, Longitude: 37.62},
			{RoutePlanStopID: cargoStop, Ordinal: 1, StopRole: domain.StopRoleCargo, PointKind: domain.PointKindCanonicalLocation, LocationID: &a.originID, Latitude: 55.8, Longitude: 37.7, ServiceDurationSeconds: &duration},
			{RoutePlanStopID: end, Ordinal: 2, StopRole: domain.StopRoleEnd, PointKind: domain.PointKindCanonicalLocation, LocationID: &b.originID, Latitude: 56.1, Longitude: 38.1},
		},
		Actions: []domain.ProjectionAction{
			materialAction(cargoStop, a, version, 0),
			materialAction(cargoStop, b, version, 1),
		},
	}
}

func materialSubject(row seededShipment, version int) domain.ProjectionSubject {
	return domain.ProjectionSubject{
		RouteSubjectType:         domain.RouteSubjectShipmentCargo,
		RouteSubjectID:           row.subjectID,
		RouteSubjectVersion:      version,
		ExecutionShipmentID:      &row.shipmentID,
		ShipmentTenantID:         &row.tenantID,
		ExecutionShipmentVersion: &version,
		CargoID:                  &row.cargoID,
		CargoVersion:             &version,
	}
}

func materialAction(stopID uuid.UUID, row seededShipment, version, ordinal int) domain.ProjectionAction {
	return domain.ProjectionAction{
		RoutePlanActionID:        uuid.New(),
		RoutePlanStopID:          stopID,
		ActionOrdinal:            ordinal,
		ActionType:               domain.ActionTypePickup,
		RouteSubjectType:         domain.RouteSubjectShipmentCargo,
		RouteSubjectID:           row.subjectID,
		ExecutionShipmentID:      &row.shipmentID,
		ShipmentTenantID:         &row.tenantID,
		ExecutionShipmentVersion: &version,
		CargoID:                  &row.cargoID,
		CargoVersion:             &version,
		EvidenceState:            "PLANNED",
		EvidenceStateVersion:     &version,
	}
}

func shipmentStatus(t *testing.T, env *execEnv, shipmentID, tenantID uuid.UUID) string {
	t.Helper()
	var status string
	if err := env.pool.QueryRow(env.ctx, `SELECT status FROM transport.shipments WHERE id=$1 AND tenant_id=$2`, shipmentID, tenantID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func shipmentTenant(t *testing.T, env *execEnv, shipmentID uuid.UUID) uuid.UUID {
	t.Helper()
	var tenantID uuid.UUID
	if err := env.pool.QueryRow(env.ctx, `SELECT tenant_id FROM transport.shipments WHERE id=$1`, shipmentID).Scan(&tenantID); err != nil {
		t.Fatal(err)
	}
	return tenantID
}

func countWhere(t *testing.T, env *execEnv, table, where string, args ...any) int {
	t.Helper()
	var n int
	query := "SELECT count(*) FROM " + table + " WHERE " + where
	if err := env.pool.QueryRow(env.ctx, query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func relationExists(t *testing.T, env *execEnv, schema, name string) bool {
	t.Helper()
	var exists bool
	err := env.pool.QueryRow(env.ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = $1 AND table_name = $2
		)
	`, schema, name).Scan(&exists)
	if err != nil {
		t.Fatal(err)
	}
	return exists
}

func tableExists(t *testing.T, env *execEnv, name string) bool {
	t.Helper()
	var exists bool
	err := env.pool.QueryRow(env.ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'transport' AND table_name = $1
		)
	`, name).Scan(&exists)
	if err != nil {
		t.Fatal(err)
	}
	return exists
}

func assertColumns(t *testing.T, env *execEnv, table string, present, absent []string) {
	t.Helper()
	rows, err := env.pool.Query(env.ctx, `
		SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'transport' AND table_name = $1
	`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		found[name] = true
	}
	for _, name := range present {
		if !found[name] {
			t.Fatalf("%s missing %s", table, name)
		}
	}
	for _, name := range absent {
		if found[name] {
			t.Fatalf("%s has forbidden %s", table, name)
		}
	}
}

func reason(err error) string {
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		return ""
	}
	value, _ := appErr.Details["reason"].(string)
	return value
}

func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
