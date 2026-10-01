package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func durationPolicyImmutable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "SERVICE_DURATION_POLICY_IMMUTABLE")
}

func (t *pgTx) ActiveServiceDurationPolicy(ctx context.Context, tenant uuid.UUID) (ServiceDurationPolicy, error) {
	var row ServiceDurationPolicy
	err := t.tx.QueryRow(ctx, `
		SELECT policy.id, policy.tenant_id, policy.version, policy.status, policy.created_at, policy.published_at, policy.retired_at,
		       pickup.duration_seconds, delivery.duration_seconds
		FROM network_optimizer.service_duration_policies AS policy
		JOIN network_optimizer.service_duration_policy_entries AS pickup
		  ON pickup.policy_id = policy.id AND pickup.action_type = 'PICKUP'
		JOIN network_optimizer.service_duration_policy_entries AS delivery
		  ON delivery.policy_id = policy.id AND delivery.action_type = 'DELIVERY'
		WHERE policy.tenant_id = $1 AND policy.status = 'ACTIVE'
	`, tenant).Scan(
		&row.ID, &row.TenantID, &row.Version, &row.Status, &row.CreatedAt, &row.PublishedAt, &row.RetiredAt,
		&row.PickupSeconds, &row.DeliverySeconds,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ServiceDurationPolicy{}, ErrNotFound
		}
		return ServiceDurationPolicy{}, err
	}
	return row, nil
}

func (t *pgTx) InsertServiceDurationDraft(ctx context.Context, row ServiceDurationPolicy) error {
	if row.PickupSeconds < 0 || row.DeliverySeconds < 0 || row.Version < 1 || row.Status != DurationPolicyDraft {
		return ErrConflict
	}
	if _, err := t.tx.Exec(ctx, `
		INSERT INTO network_optimizer.service_duration_policies (
			id, tenant_id, version, status, created_at, published_at, retired_at
		) VALUES ($1,$2,$3,'DRAFT',$4,NULL,NULL)
	`, row.ID, row.TenantID, row.Version, row.CreatedAt); err != nil {
		if isConstraint(err, "service_duration_policies_tenant_id_version_key") {
			return ErrConflict
		}
		return err
	}
	if _, err := t.tx.Exec(ctx, `
		INSERT INTO network_optimizer.service_duration_policy_entries (policy_id, action_type, duration_seconds)
		VALUES ($1,'PICKUP',$2), ($1,'DELIVERY',$3)
	`, row.ID, row.PickupSeconds, row.DeliverySeconds); err != nil {
		return err
	}
	return nil
}

func (t *pgTx) UpdateServiceDurationDraft(ctx context.Context, tenant, id uuid.UUID, pickup, delivery int) error {
	if pickup < 0 || delivery < 0 {
		return ErrConflict
	}
	var status string
	err := t.tx.QueryRow(ctx, `
		SELECT status FROM network_optimizer.service_duration_policies
		WHERE id = $1 AND tenant_id = $2
	`, id, tenant).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status != DurationPolicyDraft {
		return ErrConflict
	}
	tag, err := t.tx.Exec(ctx, `
		UPDATE network_optimizer.service_duration_policy_entries AS entry
		SET duration_seconds = CASE entry.action_type
			WHEN 'PICKUP' THEN $2
			WHEN 'DELIVERY' THEN $3
			ELSE entry.duration_seconds
		END
		WHERE entry.policy_id = $1
		  AND entry.action_type IN ('PICKUP', 'DELIVERY')
	`, id, pickup, delivery)
	if err != nil {
		if durationPolicyImmutable(err) {
			return ErrConflict
		}
		return err
	}
	if tag.RowsAffected() != 2 {
		return ErrConflict
	}
	return nil
}

func (t *pgTx) PublishServiceDurationPolicy(ctx context.Context, tenant, id uuid.UUID, at time.Time) (ServiceDurationPolicy, error) {
	stamp := at.UTC()
	if _, err := t.tx.Exec(ctx, `
		UPDATE network_optimizer.service_duration_policies
		SET status = 'RETIRED', retired_at = $2
		WHERE tenant_id = $1 AND status = 'ACTIVE'
	`, tenant, stamp); err != nil {
		return ServiceDurationPolicy{}, err
	}
	tag, err := t.tx.Exec(ctx, `
		UPDATE network_optimizer.service_duration_policies
		SET status = 'ACTIVE', published_at = $3
		WHERE id = $1 AND tenant_id = $2 AND status = 'DRAFT'
	`, id, tenant, stamp)
	if err != nil {
		if isConstraint(err, "service_duration_policies_one_active_idx") || durationPolicyImmutable(err) {
			return ServiceDurationPolicy{}, ErrConflict
		}
		return ServiceDurationPolicy{}, err
	}
	if tag.RowsAffected() != 1 {
		return ServiceDurationPolicy{}, ErrConflict
	}
	return t.ActiveServiceDurationPolicy(ctx, tenant)
}

func (t *pgTx) NextServiceDurationVersion(ctx context.Context, tenant uuid.UUID) (int, error) {
	var version int
	err := t.tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM network_optimizer.service_duration_policies
		WHERE tenant_id = $1
	`, tenant).Scan(&version)
	return version, err
}
