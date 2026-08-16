export function formatSlaRemainingLabel(
  remainingSeconds: number | null | undefined,
  t: (key: string, params?: Record<string, unknown>) => string,
): string {
  if (remainingSeconds == null) {
    return ''
  }
  if (remainingSeconds < 0) {
    const overdueMinutes = Math.ceil(Math.abs(remainingSeconds) / 60)
    if (overdueMinutes < 60) {
      return t('controlTower.exceptions.overdueMinutes', { count: overdueMinutes })
    }
    const hours = Math.floor(overdueMinutes / 60)
    const minutes = overdueMinutes % 60
    if (minutes === 0) {
      return t('controlTower.exceptions.overdueHours', { count: hours })
    }
    return t('controlTower.exceptions.overdueHoursMinutes', { hours, minutes })
  }

  const totalMinutes = Math.ceil(remainingSeconds / 60)
  if (totalMinutes < 60) {
    return t('controlTower.exceptions.remainingMinutes', { count: totalMinutes })
  }
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  if (minutes === 0) {
    return t('controlTower.exceptions.remainingHours', { count: hours })
  }
  return t('controlTower.exceptions.remainingHoursMinutes', { hours, minutes })
}

export function computeLocalRemainingSeconds(deadlineIso: string | undefined, nowMs: number): number | null {
  if (!deadlineIso) return null
  const deadlineMs = new Date(deadlineIso).getTime()
  if (Number.isNaN(deadlineMs)) return null
  return Math.floor((deadlineMs - nowMs) / 1000)
}
