package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

// SetRelationshipActivationEnabled turns on the same-transaction authorizer path.
// The default is off. While off, Project keeps the headerless revision it commits today.
// While on, Project refuses unless the enforcement trigger is installed and the
// session is not the shared freight login. The flag does not disable that trigger.
func (r *TransportExecutionRepository) SetRelationshipActivationEnabled(enabled bool) {
	if r == nil {
		return
	}
	r.relationshipActivation = enabled
}

func (r *TransportExecutionRepository) authorizeRelationshipActivation(ctx context.Context, tx pgx.Tx, cmd domain.ProjectionCommand, digest string, subjects []domain.MaterializedSubject) (*domain.ProjectionResult, uuid.UUID, error) {
	if err := requireActivationEnforcement(ctx, tx); err != nil {
		return nil, uuid.Nil, err
	}
	if err := requireOperatingCarrier(ctx, tx, cmd.CarrierCompanyID, cmd.OperatingTenantID); err != nil {
		return nil, uuid.Nil, err
	}
	pins, err := loadActivationPinSubjects(ctx, tx, subjects)
	if err != nil {
		return nil, uuid.Nil, err
	}
	fingerprint, err := domain.PinSetFingerprint(cmd.OperatingTenantID, cmd.CarrierCompanyID, pins)
	if err != nil {
		return nil, uuid.Nil, apperrors.Internal("activation fingerprint failed", err)
	}
	outcome, authID, executionID, revisionID, err := callActivationAuthorizer(ctx, tx, cmd, digest, fingerprint, pins)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if outcome == "REPLAY" {
		if executionID == uuid.Nil || revisionID == uuid.Nil {
			storedRevision, storedExecution, _, found, findErr := findActivation(ctx, tx, cmd.OperatingTenantID, cmd.ActivationID)
			if findErr != nil {
				return nil, uuid.Nil, findErr
			}
			if !found {
				return nil, uuid.Nil, domain.ActivationRefusal(domain.ReasonRelationshipStoreUnavailable)
			}
			executionID, revisionID = storedExecution, storedRevision
		}
		return &domain.ProjectionResult{
			ExecutionID:  executionID,
			RevisionID:   revisionID,
			ActivationID: cmd.ActivationID,
			Created:      false,
		}, uuid.Nil, nil
	}
	if outcome != "AUTHORIZED" || authID == uuid.Nil {
		return nil, uuid.Nil, domain.ActivationRefusal(domain.ReasonRelationshipStoreUnavailable)
	}
	if err := recheckActivationSubjects(ctx, tx, subjects, pins); err != nil {
		return nil, uuid.Nil, err
	}
	if err := lockOperatingAssignment(ctx, tx, cmd); err != nil {
		return nil, uuid.Nil, err
	}
	return nil, authID, nil
}

func recordActivationEvidence(ctx context.Context, tx pgx.Tx, revisionID, authorizationID uuid.UUID, cmd domain.ProjectionCommand) error {
	var attached int
	if err := tx.QueryRow(ctx, `SELECT transport.attach_activation_pins($1)`, revisionID).Scan(&attached); err != nil {
		return mapActivationError(err)
	}
	if attached < 1 {
		return domain.ActivationRefusal(domain.ReasonRelationshipStoreUnavailable)
	}
	if _, err := tx.Exec(ctx, `
		SELECT company.record_activation_audit($1, $2, $3, $4, $5)
	`, authorizationID, uuid.Nil, cmd.OperatingTenantID, cmd.CarrierCompanyID, cmd.ActivationID.String()); err != nil {
		return mapActivationError(err)
	}
	return nil
}

func requireActivationEnforcement(ctx context.Context, tx pgx.Tx) error {
	var triggerInstalled, superuser, bypass, sharedFreight, ownsProtected, writesProtected bool
	err := tx.QueryRow(ctx, `
		SELECT
			EXISTS (
				SELECT 1
				FROM pg_catalog.pg_trigger AS trg
				WHERE trg.tgname = 'trg_new_activation_authorization'
				  AND NOT trg.tgisinternal
			),
			role_row.rolsuper,
			role_row.rolbypassrls,
			role_row.rolname = 'freight',
			EXISTS (
				SELECT 1
				FROM pg_catalog.pg_class AS relation
				JOIN pg_catalog.pg_roles AS owner_role ON owner_role.oid = relation.relowner
				WHERE owner_role.rolname = current_user
				  AND relation.oid IN (
					'transport.transport_execution_revisions'::regclass,
					'transport.transport_execution_carrier_relationship_pins'::regclass,
					'company.carrier_relationship_activation_authorizations'::regclass,
					'company.carrier_relationship_activation_subjects'::regclass
				  )
			),
			EXISTS (
				SELECT 1
				FROM pg_catalog.pg_class AS relation
				CROSS JOIN LATERAL pg_catalog.aclexplode(relation.relacl) AS acl
				JOIN pg_catalog.pg_roles AS grantee ON grantee.oid = acl.grantee
				WHERE grantee.rolname = current_user
				  AND acl.privilege_type IN ('INSERT', 'UPDATE', 'DELETE')
				  AND relation.oid IN (
					'transport.transport_execution_carrier_relationship_pins'::regclass,
					'company.carrier_relationship_activation_authorizations'::regclass,
					'company.carrier_relationship_activation_subjects'::regclass
				  )
			)
		FROM pg_catalog.pg_roles AS role_row
		WHERE role_row.rolname = current_user
	`).Scan(&triggerInstalled, &superuser, &bypass, &sharedFreight, &ownsProtected, &writesProtected)
	if err != nil {
		return mapActivationError(err)
	}
	if !triggerInstalled || superuser || bypass || sharedFreight || ownsProtected || writesProtected {
		return domain.ActivationRefusal(domain.ReasonRelationshipStoreUnavailable)
	}
	return nil
}

func requireOperatingCarrier(ctx context.Context, tx pgx.Tx, companyID, tenantID uuid.UUID) error {
	var found bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM core.companies
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`, companyID, tenantID).Scan(&found)
	if err != nil {
		return mapActivationError(err)
	}
	if !found {
		return domain.ExecutionCommandError(domain.ReasonTenantDenied, true)
	}
	return nil
}

func loadActivationPinSubjects(ctx context.Context, tx pgx.Tx, subjects []domain.MaterializedSubject) ([]domain.ActivationPinSubject, error) {
	pins := make([]domain.ActivationPinSubject, 0, len(subjects))
	seen := map[string]uuid.UUID{}
	for _, subject := range subjects {
		key := subject.ShipmentTenantID.String() + ":" + subject.ShipmentID.String()
		if _, ok := seen[key]; ok {
			continue
		}
		var source *uuid.UUID
		err := tx.QueryRow(ctx, `
			SELECT carrier_company_id
			FROM transport.shipments
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		`, subject.ShipmentID, subject.ShipmentTenantID).Scan(&source)
		if errors.Is(err, pgx.ErrNoRows) || source == nil || *source == uuid.Nil {
			return nil, domain.ActivationRefusal(domain.ReasonRelationshipNotFound)
		}
		if err != nil {
			return nil, mapActivationError(err)
		}
		seen[key] = *source
		pins = append(pins, domain.ActivationPinSubject{
			ShipmentTenantID: subject.ShipmentTenantID,
			ShipmentID:       subject.ShipmentID,
			SourceCompanyID:  *source,
		})
	}
	if len(pins) == 0 {
		return nil, domain.ActivationRefusal(domain.ReasonRelationshipNotFound)
	}
	return pins, nil
}

func callActivationAuthorizer(ctx context.Context, tx pgx.Tx, cmd domain.ProjectionCommand, digest, fingerprint string, pins []domain.ActivationPinSubject) (string, uuid.UUID, uuid.UUID, uuid.UUID, error) {
	tenants := make([]uuid.UUID, len(pins))
	shipments := make([]uuid.UUID, len(pins))
	sources := make([]uuid.UUID, len(pins))
	for i, pin := range pins {
		tenants[i] = pin.ShipmentTenantID
		shipments[i] = pin.ShipmentID
		sources[i] = pin.SourceCompanyID
	}
	var outcome string
	var authID, executionID, revisionID *uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT outcome, authorization_id, execution_id, revision_id
		FROM company.authorize_carrier_relationship_activation(
			$1, $2, $3, $4, $5,
			(
				SELECT pg_catalog.array_agg(
					ROW(subject.shipment_tenant_id, subject.shipment_id, subject.source_company_id)::company.activation_subject_input
					ORDER BY subject.ord
				)
				FROM (
					SELECT tenants.tenant_id AS shipment_tenant_id,
					       shipments.shipment_id,
					       sources.source_company_id,
					       tenants.ord
					FROM pg_catalog.unnest($6::uuid[]) WITH ORDINALITY AS tenants(tenant_id, ord)
					JOIN pg_catalog.unnest($7::uuid[]) WITH ORDINALITY AS shipments(shipment_id, ord) USING (ord)
					JOIN pg_catalog.unnest($8::uuid[]) WITH ORDINALITY AS sources(source_company_id, ord) USING (ord)
				) AS subject
			)
		)
	`, cmd.ActivationID, cmd.OperatingTenantID, cmd.CarrierCompanyID, digest, fingerprint, tenants, shipments, sources).Scan(
		&outcome, &authID, &executionID, &revisionID,
	)
	if err != nil {
		return "", uuid.Nil, uuid.Nil, uuid.Nil, mapActivationError(err)
	}
	return outcome, uuidOrNil(authID), uuidOrNil(executionID), uuidOrNil(revisionID), nil
}

func recheckActivationSubjects(ctx context.Context, tx pgx.Tx, subjects []domain.MaterializedSubject, pins []domain.ActivationPinSubject) error {
	expected := map[string]uuid.UUID{}
	for _, pin := range pins {
		expected[pin.ShipmentTenantID.String()+":"+pin.ShipmentID.String()] = pin.SourceCompanyID
	}
	for _, subject := range subjects {
		var version int
		var source *uuid.UUID
		err := tx.QueryRow(ctx, `
			SELECT version, carrier_company_id
			FROM transport.shipments
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
			FOR UPDATE
		`, subject.ShipmentID, subject.ShipmentTenantID).Scan(&version, &source)
		if errors.Is(err, pgx.ErrNoRows) || source == nil {
			return domain.ActivationRefusal(domain.ReasonRelationshipNotFound)
		}
		if err != nil {
			return mapActivationError(err)
		}
		if version != subject.ShipmentVersion {
			return domain.ProjectionConflict(domain.ReasonPlanStale)
		}
		key := subject.ShipmentTenantID.String() + ":" + subject.ShipmentID.String()
		if expected[key] != *source {
			return domain.ActivationRefusal(domain.ReasonRelationshipNotFound)
		}
		var cargoVersion int
		err = tx.QueryRow(ctx, `
			SELECT version
			FROM transport.cargoes
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
			FOR UPDATE
		`, subject.CargoID, subject.ShipmentTenantID).Scan(&cargoVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ProjectionConflict(domain.ReasonExecutionSubjectUnmaterialized)
		}
		if err != nil {
			return mapActivationError(err)
		}
		if cargoVersion != subject.CargoVersion {
			return domain.ProjectionConflict(domain.ReasonPlanStale)
		}
	}
	return nil
}

func lockOperatingAssignment(ctx context.Context, tx pgx.Tx, cmd domain.ProjectionCommand) error {
	if cmd.DriverID != nil {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			SELECT id FROM transport.drivers
			WHERE id = $1
			  AND tenant_id = $2
			  AND carrier_company_id = $3
			  AND status = 'ACTIVE'
			  AND deleted_at IS NULL
			FOR UPDATE
		`, *cmd.DriverID, cmd.OperatingTenantID, cmd.CarrierCompanyID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ActivationRefusal(domain.ReasonOperatingDriverNotValid)
		}
		if err != nil {
			return mapActivationError(err)
		}
	}
	if cmd.VehicleID != nil {
		var id uuid.UUID
		err := tx.QueryRow(ctx, `
			SELECT id FROM transport.vehicles
			WHERE id = $1
			  AND tenant_id = $2
			  AND carrier_company_id = $3
			  AND status = 'ACTIVE'
			  AND deleted_at IS NULL
			FOR UPDATE
		`, *cmd.VehicleID, cmd.OperatingTenantID, cmd.CarrierCompanyID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ActivationRefusal(domain.ReasonOperatingVehicleNotValid)
		}
		if err != nil {
			return mapActivationError(err)
		}
	}
	return nil
}

var propagateExecutionRelationshipPins = copySuccessorRelationshipPins

// SetSuccessorPinPropagateForTest replaces the successor pin copy for one test.
func SetSuccessorPinPropagateForTest(fn func(context.Context, pgx.Tx, uuid.UUID) error) func() {
	previous := propagateExecutionRelationshipPins
	if fn == nil {
		propagateExecutionRelationshipPins = copySuccessorRelationshipPins
	} else {
		propagateExecutionRelationshipPins = fn
	}
	return func() { propagateExecutionRelationshipPins = previous }
}

func copySuccessorRelationshipPins(ctx context.Context, tx pgx.Tx, successorRevisionID uuid.UUID) error {
	var present bool
	if err := tx.QueryRow(ctx, `SELECT to_regprocedure('transport.propagate_execution_relationship_pins(uuid)') IS NOT NULL`).Scan(&present); err != nil {
		return mapDBError(err)
	}
	if !present {
		return nil
	}
	var copied int
	if err := tx.QueryRow(ctx, `SELECT transport.propagate_execution_relationship_pins($1)`, successorRevisionID).Scan(&copied); err != nil {
		return mapActivationError(err)
	}
	return nil
}

func uuidOrNil(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func mapActivationError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return err
	}
	message := err.Error()
	for _, reason := range []string{
		domain.ReasonActivationBodyConflict,
		domain.ReasonRelationshipNotFound,
		domain.ReasonRelationshipOperatingCompanyMismatch,
		domain.ReasonRelationshipRevoked,
		domain.ReasonRelationshipAmbiguous,
		domain.ReasonOperatingDriverNotValid,
		domain.ReasonOperatingVehicleNotValid,
	} {
		if strings.Contains(message, reason) {
			return domain.ActivationRefusal(reason)
		}
	}
	if strings.Contains(message, "activation subject set is empty") {
		return domain.ActivationRefusal(domain.ReasonRelationshipNotFound)
	}
	if strings.Contains(message, "42501") || strings.Contains(strings.ToLower(message), "permission denied") {
		return domain.ActivationRefusal(domain.ReasonRelationshipStoreUnavailable)
	}
	return mapDBError(err)
}
