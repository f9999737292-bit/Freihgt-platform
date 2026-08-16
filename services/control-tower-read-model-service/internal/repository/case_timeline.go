package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

func (r *CaseRepository) ListTimeline(ctx context.Context, tenantID, caseID uuid.UUID, page, limit int) ([]domain.CaseEvent, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	offset := (page - 1) * limit

	const timelineSQL = `
WITH unified AS (
  SELECT e.id::text AS sort_id, e.occurred_at, e.source, e.action_type, e.actor_user_id, e.metadata, 'case' AS origin
  FROM control_tower.operational_case_event e
  WHERE e.tenant_id = $1 AND e.case_id = $2
  UNION ALL
  SELECT ('wf-' || a.id::text) AS sort_id, a.occurred_at, 'WORKFLOW' AS source, a.action_type, a.actor_user_id,
         jsonb_build_object('eventId', a.event_id) AS metadata, 'workflow' AS origin
  FROM control_tower.critical_event_action a
  JOIN control_tower.operational_case_link l
    ON l.tenant_id = a.tenant_id AND l.entity_type = 'exception' AND l.entity_id = a.event_id
  WHERE l.case_id = $2 AND a.tenant_id = $1
  UNION ALL
  SELECT ('rk-' || a.id::text) AS sort_id, a.occurred_at, 'RISK' AS source, a.action_type, a.actor_user_id,
         jsonb_build_object('riskKey', a.risk_key) AS metadata, 'risk' AS origin
  FROM control_tower.shipment_risk_action a
  JOIN control_tower.operational_case_link l
    ON l.tenant_id = a.tenant_id AND l.entity_type = 'risk' AND l.entity_id = a.risk_key
  WHERE l.case_id = $2 AND a.tenant_id = $1
  UNION ALL
  SELECT ('ho-' || hi.id::text) AS sort_id, hi.created_at AS occurred_at, 'HANDOFF' AS source,
         'handoff_transferred' AS action_type, h.from_user_id AS actor_user_id,
         jsonb_build_object('itemType', hi.item_type, 'sourceId', hi.source_id, 'handoffId', h.id::text) AS metadata,
         'handoff' AS origin
  FROM control_tower.shift_handoff_item hi
  JOIN control_tower.shift_handoff h ON h.id = hi.handoff_id
  JOIN control_tower.operational_case_link l
    ON l.tenant_id = hi.tenant_id AND l.entity_type = hi.item_type AND l.entity_id = hi.source_id
  WHERE l.case_id = $2 AND hi.tenant_id = $1
)
SELECT sort_id, occurred_at, source, action_type, actor_user_id, metadata
FROM unified
ORDER BY occurred_at DESC, sort_id DESC
LIMIT $3 OFFSET $4`

	countSQL := `
WITH unified AS (
  SELECT e.id FROM control_tower.operational_case_event e WHERE e.tenant_id = $1 AND e.case_id = $2
  UNION ALL
  SELECT a.id FROM control_tower.critical_event_action a
  JOIN control_tower.operational_case_link l ON l.tenant_id = a.tenant_id AND l.entity_type = 'exception' AND l.entity_id = a.event_id
  WHERE l.case_id = $2 AND a.tenant_id = $1
  UNION ALL
  SELECT a.id FROM control_tower.shipment_risk_action a
  JOIN control_tower.operational_case_link l ON l.tenant_id = a.tenant_id AND l.entity_type = 'risk' AND l.entity_id = a.risk_key
  WHERE l.case_id = $2 AND a.tenant_id = $1
  UNION ALL
  SELECT hi.id FROM control_tower.shift_handoff_item hi
  JOIN control_tower.operational_case_link l ON l.tenant_id = hi.tenant_id AND l.entity_type = hi.item_type AND l.entity_id = hi.source_id
  WHERE l.case_id = $2 AND hi.tenant_id = $1
)
SELECT COUNT(*) FROM unified`

	var total int
	if err := r.pool.QueryRow(ctx, countSQL, tenantID, caseID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, timelineSQL, tenantID, caseID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]domain.CaseEvent, 0)
	for rows.Next() {
		var sortID string
		var occurredAt time.Time
		var source, actionType string
		var actorID *uuid.UUID
		var metadataRaw []byte
		if err := rows.Scan(&sortID, &occurredAt, &source, &actionType, &actorID, &metadataRaw); err != nil {
			return nil, 0, err
		}
		meta := map[string]any{}
		if len(metadataRaw) > 0 {
			_ = json.Unmarshal(metadataRaw, &meta)
		}
		out = append(out, domain.CaseEvent{
			Source: source, ActionType: actionType, ActorUserID: actorID,
			OccurredAt: occurredAt.UTC(), Metadata: meta,
		})
	}
	return out, total, rows.Err()
}
