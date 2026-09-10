import type {
  RfxCompareChangeType,
  RfxCompareItemDiff,
  RfxCompareSummary,
  RfxCompareVersionsResponse,
  RfxImpactClass,
} from '~/types/rfx-version-lifecycle'

export function isCompareEmpty(summary: RfxCompareSummary): boolean {
  return (
    summary.added_count === 0
    && summary.removed_count === 0
    && summary.changed_count === 0
    && summary.reordered_count === 0
  )
}

export function filterVisibleCompareItems(items: RfxCompareItemDiff[]): RfxCompareItemDiff[] {
  return items.filter((item) => item.change !== 'UNCHANGED')
}

export function groupCompareItemsByChange(items: RfxCompareItemDiff[]): Record<RfxCompareChangeType, RfxCompareItemDiff[]> {
  const groups: Record<RfxCompareChangeType, RfxCompareItemDiff[]> = {
    ADDED: [],
    REMOVED: [],
    CHANGED: [],
    REORDERED: [],
    UNCHANGED: [],
  }
  for (const item of items) {
    groups[item.change].push(item)
  }
  return groups
}

export function compareItemLabel(item: RfxCompareItemDiff): string {
  const parts: string[] = []
  if (item.section_code) parts.push(item.section_code)
  if (item.question_code) parts.push(item.question_code)
  if (item.option_code) parts.push(item.option_code)
  if (item.rule_code) parts.push(item.rule_code)
  if (item.criterion_code) parts.push(item.criterion_code)
  if (parts.length === 0) return item.entity_type
  return parts.join(' / ')
}

export function swapCompareDirection(request: {
  source_version_id: string
  target_version_id: string
}): { source_version_id: string; target_version_id: string } {
  return {
    source_version_id: request.target_version_id,
    target_version_id: request.source_version_id,
  }
}

export function mergeCompareSections(response: RfxCompareVersionsResponse): RfxCompareItemDiff[] {
  return [
    ...response.sections,
    ...response.questions,
    ...response.options,
    ...response.rules,
    ...(response.differences ?? []),
  ]
}

export function impactClassSeverity(className: RfxImpactClass): number {
  const order: RfxImpactClass[] = [
    'NON_MATERIAL',
    'MATERIAL_NO_RESPONSES',
    'MATERIAL_WITH_DRAFT_RESPONSES',
    'MATERIAL_WITH_SUBMITTED_RESPONSES',
    'SCORING_AFFECTING',
    'KNOCKOUT_AFFECTING',
  ]
  return order.indexOf(className)
}

export function sortImpactClasses(classes: RfxImpactClass[]): RfxImpactClass[] {
  return [...classes].sort((a, b) => impactClassSeverity(b) - impactClassSeverity(a))
}
