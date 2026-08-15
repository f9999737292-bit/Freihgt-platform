export type AutomationRuleStatus = 'draft' | 'active' | 'disabled' | 'retired'
export type AutomationExecutionMode = 'observe' | 'recommend' | 'guarded_auto'
export type GuardedActionStatus =
  | 'pending'
  | 'waiting_approval'
  | 'running'
  | 'waiting_response'
  | 'succeeded'
  | 'failed'
  | 'denied'
  | 'rejected'
  | 'timed_out'
  | 'skipped'
export type GuardDecision = 'allow' | 'require_approval' | 'deny' | 'skip'
export type PlaybookStatus = 'draft' | 'active' | 'retired'
export type RecommendationStatus = 'pending' | 'accepted' | 'dismissed' | 'expired' | 'completed'
export type PlaybookExecutionStatus = 'not_started' | 'in_progress' | 'completed' | 'cancelled'
export type PlaybookStepStatus = 'pending' | 'in_progress' | 'done' | 'skipped'
export type DismissReason = 'not_relevant' | 'already_handled' | 'duplicate' | 'false_positive' | 'other'

export interface ConditionClause {
  field: string
  operator: string
  value?: unknown
}

export interface ConditionGroup {
  logic: 'ALL' | 'ANY'
  conditions: ConditionClause[]
  groups?: ConditionGroup[]
}

export interface AutomationRule {
  id: string
  name: string
  description?: string
  status: AutomationRuleStatus
  triggerType: string
  conditions: ConditionGroup
  conditionSchemaVersion: number
  playbookId?: string
  executionMode: AutomationExecutionMode
  priority: number
  version: number
  createdAt: string
  updatedAt: string
}

export interface PlaybookStep {
  id?: string
  sequence: number
  title: string
  description?: string
  stepType: 'instruction' | 'checklist' | 'operator_action' | 'system_action'
  required: boolean
  actionCode?: string
  estimatedDurationMinutes?: number
}

export interface OperationalPlaybook {
  id: string
  name: string
  description?: string
  status: PlaybookStatus
  currentVersion: number
  stepCount?: number
  steps?: PlaybookStep[]
  createdAt: string
  updatedAt: string
}

export interface MatchedCondition {
  field: string
  operator: string
  expected?: unknown
  actual?: unknown
  matched: boolean
}

export interface AutomationRecommendation {
  id: string
  ruleId: string
  ruleVersion: number
  ruleName?: string
  playbookId: string
  playbookVersion: number
  playbookName?: string
  triggerType: string
  status: RecommendationStatus
  matchedConditions: MatchedCondition[]
  createdAt: string
  shipmentId?: string
  workItemType?: string
  workItemId?: string
  caseId?: string
  dismissReason?: DismissReason
}

export interface PlaybookExecutionStep {
  id: string
  sequence: number
  title: string
  description?: string
  stepType: string
  required: boolean
  actionCode?: string
  status: PlaybookStepStatus
  skipReason?: string
}

export interface PlaybookExecution {
  id: string
  playbookId: string
  playbookVersion: number
  playbookName?: string
  recommendationId?: string
  ownerUserId: string
  status: PlaybookExecutionStatus
  steps: PlaybookExecutionStep[]
  progressDone: number
  progressTotal: number
  createdAt: string
  updatedAt: string
}

export interface GuardedActionApproval {
  id: string
  requiredLevel: string
  status: 'pending' | 'approved' | 'rejected'
  requestedAt: string
  approvedAt?: string
  approvedBy?: string
  rejectedAt?: string
  rejectedBy?: string
  reason?: string
}

export interface GuardedAction {
  id: string
  executionId: string
  executionStepId: string
  actionType: string
  safetyClass: string
  guardDecision: GuardDecision
  guardReason?: string
  status: GuardedActionStatus
  driverId?: string
  shipmentId?: string
  driverTaskId?: string
  correlationId?: string
  sourceEventId?: string
  errorReason?: string
  response?: Record<string, unknown>
  expiresAt?: string
  createdAt?: string
  updatedAt?: string
  approval?: GuardedActionApproval
}

export interface AutomationListPage<T> {
  items: T[]
  page: number
  limit: number
  total: number
  hasNext: boolean
}

export const AUTOMATION_TRIGGER_TYPES = [
  'risk_created',
  'risk_level_changed',
  'exception_created',
  'exception_priority_changed',
  'sla_warning',
  'sla_breached',
  'tracking_stale',
  'tracking_lost',
  'eta_at_risk',
  'eta_projected_late',
  'slot_at_risk',
  'slot_projected_miss',
  'slot_actual_missed',
  'work_item_created',
  'work_item_unassigned',
  'case_created',
  'case_status_changed',
] as const

export const CONDITION_FIELDS = [
  'priority',
  'riskLevel',
  'slaStatus',
  'slotProjectedLateSeconds',
  'etaStatus',
  'trackingStatus',
  'caseSeverity',
  'assigned',
  'hasActiveCase',
] as const

export const OPERATOR_ACTION_CODES = [
  'contact_carrier',
  'contact_driver',
  'check_tracking',
  'review_slot',
  'request_slot_reschedule',
  'create_case',
  'create_action_item',
  'monitor',
  'other',
] as const
