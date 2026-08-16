import type { ControlTowerCaseHealth, ControlTowerCaseSeverity } from '~/types/controlTower'

const SEVERITY_RANK: Record<ControlTowerCaseSeverity, number> = {
  critical: 4,
  high: 3,
  medium: 2,
  low: 1,
}

export type CaseSlaDisplayState = 'breached' | 'warning' | 'normal'

export function caseSlaDisplayState(health?: ControlTowerCaseHealth | null): CaseSlaDisplayState {
  if (!health) return 'normal'
  if (health.hasSlaBreach) return 'breached'
  if (health.hasSlaWarning) return 'warning'
  return 'normal'
}

export function isSeverityDecrease(
  from: ControlTowerCaseSeverity,
  to: ControlTowerCaseSeverity,
): boolean {
  return SEVERITY_RANK[to] < SEVERITY_RANK[from]
}

export function caseTimelineCategory(source: string, actionType: string): string {
  const src = source.toUpperCase()
  const action = actionType.toLowerCase()
  if (src === 'CASE' || action.startsWith('case_') || action.startsWith('participant_') || action.includes('severity')) {
    return 'CASE'
  }
  if (src === 'NOTE') return 'NOTE'
  if (src === 'ACTION_ITEM') return 'ACTION_ITEM'
  if (src === 'DECISION') return 'DECISION'
  if (src === 'SYSTEM' || action.includes('sla')) return 'SYSTEM'
  if (action.includes('handoff')) return 'HANDOFF'
  if (action.startsWith('risk_')) return 'RISK'
  if (['acknowledged', 'assigned', 'resolved', 'claimed'].includes(action)) return 'WORKFLOW'
  return 'SYSTEM'
}

export function formatRelativeTime(iso: string | undefined, now = Date.now()): string | null {
  if (!iso) return null
  const target = new Date(iso).getTime()
  if (Number.isNaN(target)) return null
  const diffMs = target - now
  const absMinutes = Math.round(Math.abs(diffMs) / 60000)
  const absHours = Math.round(absMinutes / 60)
  const absDays = Math.round(absHours / 24)
  if (diffMs >= 0) {
    if (absMinutes < 60) return `in_${absMinutes}m`
    if (absHours < 24) return `in_${absHours}h`
    if (absDays === 1) return 'tomorrow'
    return `in_${absDays}d`
  }
  if (absMinutes < 60) return `overdue_${absMinutes}m`
  if (absHours < 24) return `overdue_${absHours}h`
  return `overdue_${absDays}d`
}

export function formatCaseDateTime(iso?: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}
