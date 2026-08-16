package controltower

import (
	"sort"
	"time"
)

const (
	PriorityP1 = "p1"
	PriorityP2 = "p2"
	PriorityP3 = "p3"
	PriorityP4 = "p4"

	SLAStatusWithinSLA = "within_sla"
	SLAStatusWarning   = "warning"
	SLAStatusBreached  = "breached"
	SLAStatusCompleted = "completed"
)

func priorityRank(priority string) int {
	switch priority {
	case PriorityP1:
		return 1
	case PriorityP2:
		return 2
	case PriorityP3:
		return 3
	case PriorityP4:
		return 4
	default:
		return 5
	}
}

func slaStatusRank(status string) int {
	switch status {
	case SLAStatusBreached:
		return 1
	case SLAStatusWarning:
		return 2
	case SLAStatusWithinSLA:
		return 3
	default:
		return 4
	}
}

func isUnresolvedEvent(status string) bool {
	return status != WorkflowStatusResolved
}

// SortCriticalEvents orders events by operational importance:
// 1 unresolved before resolved
// 2 P1..P4
// 3 SLA breached, warning, within_sla
// 4 nearest deadline (remaining seconds ascending)
// 5 occurredAt desc
func SortCriticalEvents(events []ControlTowerEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]
		aUnresolved := isUnresolvedEvent(a.Status)
		bUnresolved := isUnresolvedEvent(b.Status)
		if aUnresolved != bUnresolved {
			return aUnresolved
		}
		if pr := priorityRank(a.Priority) - priorityRank(b.Priority); pr != 0 {
			return pr < 0
		}
		aSLA := SLAStatusWithinSLA
		bSLA := SLAStatusWithinSLA
		if a.SLA != nil {
			aSLA = a.SLA.Status
		}
		if b.SLA != nil {
			bSLA = b.SLA.Status
		}
		if sr := slaStatusRank(aSLA) - slaStatusRank(bSLA); sr != 0 {
			return sr < 0
		}
		aRemain := int64(1 << 60)
		bRemain := int64(1 << 60)
		if a.SLA != nil && a.SLA.RemainingSeconds != nil {
			aRemain = *a.SLA.RemainingSeconds
		}
		if b.SLA != nil && b.SLA.RemainingSeconds != nil {
			bRemain = *b.SLA.RemainingSeconds
		}
		if aRemain != bRemain {
			return aRemain < bRemain
		}
		return a.OccurredAt.After(b.OccurredAt)
	})
}

func FilterCriticalEvents(events []ControlTowerEvent, query ListQuery) []ControlTowerEvent {
	if query.EventStatus == "" && query.Priority == "" && query.ExceptionCategory == "" &&
		query.BusinessImpact == "" && query.EventSLAStatus == "" && query.EscalationLevel == "" && !query.UnassignedOnly {
		return events
	}

	filtered := make([]ControlTowerEvent, 0, len(events))
	for _, event := range events {
		if query.EventStatus != "" && event.Status != query.EventStatus {
			continue
		}
		if query.Priority != "" && event.Priority != query.Priority {
			continue
		}
		if query.ExceptionCategory != "" && event.ExceptionCategory != query.ExceptionCategory {
			continue
		}
		if query.BusinessImpact != "" && event.BusinessImpact != query.BusinessImpact {
			continue
		}
		if query.EventSLAStatus != "" {
			status := SLAStatusWithinSLA
			if event.SLA != nil {
				status = event.SLA.Status
			}
			if status != query.EventSLAStatus {
				continue
			}
		}
		if query.EscalationLevel != "" {
			level := "none"
			if event.Escalation != nil {
				level = event.Escalation.Level
			}
			if level != query.EscalationLevel {
				continue
			}
		}
		if query.UnassignedOnly {
			if event.Assignment != nil {
				continue
			}
			if event.Status != WorkflowStatusOpen && event.Status != WorkflowStatusAcknowledged {
				continue
			}
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func CalculateExceptionKPI(events []ControlTowerEvent) ExceptionKPI {
	kpi := ExceptionKPI{}
	for _, event := range events {
		if event.Status == WorkflowStatusResolved {
			slaStatus := SLAStatusCompleted
			if event.SLA != nil {
				slaStatus = event.SLA.Status
			}
			if slaStatus == SLAStatusCompleted || slaStatus == SLAStatusWithinSLA {
				kpi.ResolvedWithinSLA++
			} else {
				kpi.ResolvedOutsideSLA++
			}
			continue
		}
		kpi.TotalOpenExceptions++
		switch event.Priority {
		case PriorityP1:
			kpi.P1Open++
		case PriorityP2:
			kpi.P2Open++
		}
		if event.SLA != nil {
			switch event.SLA.Status {
			case SLAStatusWarning:
				kpi.SLAWarning++
			case SLAStatusBreached:
				kpi.SLABreached++
			}
		}
		if event.Status == WorkflowStatusAcknowledged || event.Status == WorkflowStatusOpen {
			kpi.UnassignedExceptions++
		}
	}
	return kpi
}

func defaultPriorityForSeverity(severity string) string {
	switch severity {
	case EventSeverityCritical:
		return PriorityP1
	case EventSeverityWarning:
		return PriorityP2
	case EventSeverityInfo:
		return PriorityP4
	default:
		return PriorityP3
	}
}

func defaultCategoryForEventType(eventType string) string {
	switch eventType {
	case EventTypePickupDelay:
		return "pickup_issue"
	case EventTypeDeliveryDelay:
		return "delivery_issue"
	case EventTypeStaleUpdates:
		return "delay"
	case EventTypeMissingDocuments:
		return "document_issue"
	case EventTypeShipmentCancelled:
		return "delivery_issue"
	case EventTypeTechnicalProblem:
		return "integration_issue"
	default:
		return "other"
	}
}

func applyDefaultException(event *ControlTowerEvent, now time.Time) {
	if event.Priority == "" {
		event.Priority = defaultPriorityForSeverity(event.Severity)
	}
	if event.ExceptionCategory == "" {
		event.ExceptionCategory = defaultCategoryForEventType(event.Type)
	}
	if event.BusinessImpact == "" {
		event.BusinessImpact = "none"
	}
	if event.Escalation == nil {
		event.Escalation = &ControlTowerEventEscalation{Level: "none"}
	}
	if event.SLA == nil {
		policy := slaPolicyForPriority(event.Priority)
		activatedAt := event.OccurredAt.UTC()
		event.SLA = &ControlTowerEventSLA{
			Phase:            slaPhaseForStatus(event.Status),
			Status:           SLAStatusWithinSLA,
			AcknowledgeDueAt: activatedAt.Add(policy.Acknowledge),
			AssignmentDueAt:  activatedAt.Add(policy.Assign),
			ResolutionDueAt:  activatedAt.Add(policy.Resolve),
			RemainingSeconds: int64Ptr(int64(policy.Resolve / time.Second)),
		}
	}
	_ = now
}

type slaPolicy struct {
	Acknowledge time.Duration
	Assign      time.Duration
	Resolve     time.Duration
}

func slaPolicyForPriority(priority string) slaPolicy {
	switch priority {
	case PriorityP1:
		return slaPolicy{5 * time.Minute, 10 * time.Minute, 60 * time.Minute}
	case PriorityP2:
		return slaPolicy{15 * time.Minute, 30 * time.Minute, 4 * time.Hour}
	case PriorityP4:
		return slaPolicy{240 * time.Minute, 480 * time.Minute, 72 * time.Hour}
	default:
		return slaPolicy{60 * time.Minute, 120 * time.Minute, 24 * time.Hour}
	}
}

func slaPhaseForStatus(status string) string {
	switch status {
	case WorkflowStatusOpen:
		return "acknowledgement"
	case WorkflowStatusAcknowledged:
		return "assignment"
	default:
		return "resolution"
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
