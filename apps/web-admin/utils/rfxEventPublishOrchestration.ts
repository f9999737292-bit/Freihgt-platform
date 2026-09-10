import type { RfxStudioResponse } from '~/types/rfx-questionnaire'
import type { RfxPublishQuestionnaireRequest } from '~/types/rfx-version-lifecycle'
import type { RfxVersionRecord } from '~/types/rfx-version-lifecycle'
import type { RfxChangeImpactAnalysisResponse } from '~/types/rfx-version-lifecycle'

export interface ImpactPreviewBaseline {
  eventVersion: number
  draftVersionId: string
  draftVersion: number
}

export function extractPublishBaselineFromStudio(studio: RfxStudioResponse): ImpactPreviewBaseline | null {
  if (!studio.draft_version || studio.event?.version == null) return null
  return {
    eventVersion: studio.event.version,
    draftVersionId: studio.draft_version.id,
    draftVersion: studio.draft_version.version,
  }
}

export function isImpactPreviewBaselineStale(
  baseline: ImpactPreviewBaseline,
  current: ImpactPreviewBaseline,
  analysis?: Pick<RfxChangeImpactAnalysisResponse, 'candidate_version_id'> | null,
): boolean {
  if (baseline.eventVersion !== current.eventVersion) return true
  if (baseline.draftVersionId !== current.draftVersionId) return true
  if (baseline.draftVersion !== current.draftVersion) return true
  if (analysis?.candidate_version_id && analysis.candidate_version_id !== current.draftVersionId) return true
  return false
}

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
