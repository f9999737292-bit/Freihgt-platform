import type { RfxPublishQuestionnaireRequest } from '~/types/rfx-version-lifecycle'
import type { RfxVersionRecord } from '~/types/rfx-version-lifecycle'
import type { RfxChangeImpactAnalysisResponse } from '~/types/rfx-version-lifecycle'

export function hasPublishedEventVersion(versions: RfxVersionRecord[]): boolean {
  return versions.some((v) => v.status === 'PUBLISHED' || v.is_current_published)
}

export function isRepublishRequired(versions: RfxVersionRecord[]): boolean {
  return hasPublishedEventVersion(versions)
}

export function buildEventPublishPayload(input: {
  expectedEventVersion: number
  expectedDraftVersion: number
  changeSummary: string
  impact?: RfxChangeImpactAnalysisResponse | null
}): RfxPublishQuestionnaireRequest {
  const payload: RfxPublishQuestionnaireRequest = {
    expected_event_version: input.expectedEventVersion,
    expected_draft_version: input.expectedDraftVersion,
    change_summary: input.changeSummary.trim(),
  }
  if (input.impact) {
    payload.impact_analysis_id = input.impact.impact_analysis_id
    payload.canonical_diff_hash = input.impact.canonical_diff_hash
  }
  return payload
}

export function shouldRequireImpactPreview(versions: RfxVersionRecord[]): boolean {
  return isRepublishRequired(versions)
}
