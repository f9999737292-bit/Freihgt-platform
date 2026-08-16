import { computeLocalRemainingSeconds, formatSlaRemainingLabel } from '~/utils/exceptionSlaDisplay'
import type { ControlTowerEventSLA } from '~/types/controlTower'

export function useSlaCountdown(sla: () => ControlTowerEventSLA | null | undefined) {
  const { t } = useI18n()
  const nowMs = ref(Date.now())
  let timer: ReturnType<typeof setInterval> | undefined

  onMounted(() => {
    timer = setInterval(() => {
      nowMs.value = Date.now()
    }, 30_000)
  })

  onBeforeUnmount(() => {
    if (timer) clearInterval(timer)
  })

  const label = computed(() => {
    const current = sla()
    if (!current) return ''
    const phaseDeadline =
      current.phase === 'acknowledgement'
        ? current.acknowledgeDueAt
        : current.phase === 'assignment'
          ? current.assignmentDueAt
          : current.resolutionDueAt
    const remaining = computeLocalRemainingSeconds(phaseDeadline, nowMs.value)
    if (current.status === 'completed') {
      return t('controlTower.exceptions.slaCompleted')
    }
    return formatSlaRemainingLabel(remaining, t)
  })

  return { label }
}
