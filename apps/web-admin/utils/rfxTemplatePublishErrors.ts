import type { RfxPublishReadinessResult } from '~/types/rfx-questionnaire'
import { ApiError } from '~/composables/useApi'

export function localTemplatePublishPrecheck(sectionCount: number): RfxPublishReadinessResult {
  const hasSections = sectionCount > 0
  return {
    ready: false,
    blocking_fail_count: hasSections ? 0 : 1,
    warning_count: hasSections ? 1 : 0,
    items: hasSections
      ? [{
          code: 'LOCAL_PRECHECK',
          status: 'WARN',
          message: 'rfx.templates.readiness.serverAuthoritative',
        }]
      : [{
          code: 'NO_SECTIONS',
          status: 'FAIL',
          message: 'rfx.templates.readiness.noSections',
        }],
  }
}

export function parseTemplatePublish422(error: unknown): RfxPublishReadinessResult | null {
  if (!(error instanceof ApiError) || error.status !== 422) return null
  const details = error.details as {
    ready?: boolean
    blocking_fail_count?: number
    items?: RfxPublishReadinessResult['items']
  } | undefined
  if (!details?.items) return null
  return {
    ready: details.ready ?? false,
    blocking_fail_count: details.blocking_fail_count ?? details.items.filter((i) => i.status === 'FAIL').length,
    warning_count: details.items.filter((i) => i.status === 'WARN').length,
    items: details.items,
  }
}
