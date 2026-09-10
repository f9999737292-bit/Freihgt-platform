import type {
  RfxTemplateDetailResponse,
  RfxTemplateVersionRecord,
} from '~/types/rfx-template'
import {
  canCloneFromTemplateVersion,
  isTemplateVersionEditable,
} from '~/types/rfx-template'

export type TemplateForkGate = {
  allowed: boolean
  reasonKey?: string
}

/** Fork is allowed only when ACTIVE template has PUBLISHED source and no active DRAFT. */
export function canForkTemplateDraft(detail: RfxTemplateDetailResponse | null): TemplateForkGate {
  if (!detail) return { allowed: false, reasonKey: 'rfx.templates.fork.unavailable' }
  if (detail.template.status === 'ARCHIVED') {
    return { allowed: false, reasonKey: 'rfx.templates.fork.archived' }
  }
  if (detail.draft_version && isTemplateVersionEditable(detail.draft_version.status)) {
    return { allowed: false, reasonKey: 'rfx.templates.fork.draftExists' }
  }
  if (!detail.published_version) {
    return { allowed: false, reasonKey: 'rfx.templates.fork.noPublished' }
  }
  return { allowed: true }
}

export function filterCloneableTemplateVersions(
  detail: RfxTemplateDetailResponse,
): RfxTemplateVersionRecord[] {
  return (detail.versions ?? []).filter((version) =>
    canCloneFromTemplateVersion(detail.template.status, version.status),
  )
}

/** Prefer current PUBLISHED; never auto-select SUPERSEDED when PUBLISHED exists. */
export function selectDefaultCloneVersionId(
  cloneable: RfxTemplateVersionRecord[],
): string | null {
  const published = cloneable.filter((v) => v.status === 'PUBLISHED')
  if (published.length === 0) return null
  const sorted = [...published].sort((a, b) => b.version_number - a.version_number)
  return sorted[0]?.id ?? null
}

export function findTemplateVersionById(
  detail: RfxTemplateDetailResponse,
  versionId: string,
): RfxTemplateVersionRecord | undefined {
  return detail.versions?.find((v) => v.id === versionId)
}
