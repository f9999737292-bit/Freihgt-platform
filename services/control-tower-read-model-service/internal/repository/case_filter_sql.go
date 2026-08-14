package repository

import "fmt"

func overdueActionsSQL(caseRef string) string {
	return fmt.Sprintf(`EXISTS (
		SELECT 1 FROM control_tower.operational_case_action_item ai
		WHERE ai.case_id = %s AND ai.status IN ('open','in_progress')
		  AND ai.due_at IS NOT NULL AND ai.due_at < NOW()
	)`, caseRef)
}

func slaBreachSQL(caseRef string) string {
	return fmt.Sprintf(`EXISTS (
		SELECT 1 FROM control_tower.operational_case_link l
		JOIN control_tower.critical_event_workflow wf
		  ON wf.tenant_id = l.tenant_id AND wf.event_id = l.entity_id
		WHERE l.case_id = %s AND l.entity_type = 'exception'
		  AND wf.status <> 'resolved'
		  AND (
		    wf.ack_sla_breached_at IS NOT NULL
		    OR wf.assign_sla_breached_at IS NOT NULL
		    OR wf.resolve_sla_breached_at IS NOT NULL
		    OR (wf.status = 'open' AND wf.acknowledged_at IS NULL AND wf.acknowledge_due_at < NOW())
		    OR (wf.status = 'acknowledged' AND wf.assigned_at IS NULL AND wf.assignment_due_at < NOW())
		    OR (wf.status IN ('assigned','in_progress') AND wf.resolved_at IS NULL AND wf.resolution_due_at < NOW())
		  )
	)`, caseRef)
}

func slaWarningSQL(caseRef string) string {
	return fmt.Sprintf(`EXISTS (
		SELECT 1 FROM control_tower.operational_case_link l
		JOIN control_tower.critical_event_workflow wf
		  ON wf.tenant_id = l.tenant_id AND wf.event_id = l.entity_id
		WHERE l.case_id = %s AND l.entity_type = 'exception'
		  AND wf.status <> 'resolved'
		  AND wf.ack_sla_breached_at IS NULL
		  AND wf.assign_sla_breached_at IS NULL
		  AND wf.resolve_sla_breached_at IS NULL
		  AND (
		    (wf.status = 'open' AND wf.acknowledged_at IS NULL AND wf.acknowledge_due_at > NOW()
		      AND wf.acknowledge_due_at <= NOW() + (wf.acknowledge_due_at - wf.exception_activated_at) * 0.25)
		    OR (wf.status = 'acknowledged' AND wf.assigned_at IS NULL AND wf.assignment_due_at > NOW()
		      AND wf.assignment_due_at <= NOW() + (wf.assignment_due_at - COALESCE(wf.acknowledged_at, wf.exception_activated_at)) * 0.25)
		    OR (wf.status IN ('assigned','in_progress') AND wf.resolved_at IS NULL AND wf.resolution_due_at > NOW()
		      AND wf.resolution_due_at <= NOW() + (wf.resolution_due_at - COALESCE(wf.assigned_at, wf.exception_activated_at)) * 0.25)
		  )
	)`, caseRef)
}

func slaAtRiskSQL(caseRef string) string {
	return fmt.Sprintf("(%s OR %s)", slaBreachSQL(caseRef), slaWarningSQL(caseRef))
}
