export interface SelectOption {
  label: string
  value: string
}

export function buildStatusFilterOptions(
  statuses: readonly string[],
  allLabel: string,
): SelectOption[] {
  return [{ label: allLabel, value: '' }, ...statuses.map((status) => ({ label: status, value: status }))]
}

export function matchesStatusFilter(itemStatus: string, filterStatus: string): boolean {
  if (!filterStatus.trim()) return true
  return itemStatus === filterStatus
}

export function filterByStatus<T extends { status: string }>(items: T[], filterStatus: string): T[] {
  if (!filterStatus.trim()) return items
  return items.filter((item) => matchesStatusFilter(item.status, filterStatus))
}
