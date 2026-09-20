import { isRfxLateSubmissionEnabled } from '~/utils/lateSubmissionFeatureFlag'

const LATE_UI_STORAGE_KEY = 'freight_procurement_rfx_late_submission'

export function useRfxLateSubmissionFeature() {
  const config = useRuntimeConfig()
  const enabled = computed(() => {
    if (isRfxLateSubmissionEnabled(config.public.rfxLateSubmissionEnabled)) return true
    if (import.meta.client) {
      return isRfxLateSubmissionEnabled(localStorage.getItem(LATE_UI_STORAGE_KEY))
    }
    return false
  })

  return { enabled }
}
