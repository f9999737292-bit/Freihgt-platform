/** RFx v3.0E4 template library types aligned with OpenAPI (snake_case). */

export const RFX_TEMPLATE_AGGREGATE_STATUSES = ['ACTIVE', 'ARCHIVED'] as const
export type RfxTemplateAggregateStatus = (typeof RFX_TEMPLATE_AGGREGATE_STATUSES)[number]

export const RFX_TEMPLATE_VERSION_STATUSES = ['DRAFT', 'PUBLISHED', 'SUPERSEDED'] as const
export type RfxTemplateVersionStatus = (typeof RFX_TEMPLATE_VERSION_STATUSES)[number]

export interface RfxTemplateRecord {
  id: string
  tenant_id: string
  template_code: string
  name_i18n: Record<string, string>
  description_i18n?: Record<string, string> | null
  rfx_type?: string | null
  owner_company_id?: string | null
  status: RfxTemplateAggregateStatus
  version: number
  created_by: string
  created_at: string
  updated_at: string
}

export interface RfxTemplateVersionRecord {
  id: string
  tenant_id: string
  template_id: string
  version_number: number
  status: RfxTemplateVersionStatus
  change_summary?: string | null
  is_active_draft: boolean
  is_published: boolean
  published_at?: string | null
  published_by?: string | null
  created_by: string
  created_at: string
  updated_at: string
  version: number
}

export interface RfxTemplateDetailResponse {
  template: RfxTemplateRecord
  draft_version?: RfxTemplateVersionRecord | null
  published_version?: RfxTemplateVersionRecord | null
  versions: RfxTemplateVersionRecord[]
}

export interface RfxCreateTemplateRequest {
  template_code: string
  name_i18n: Record<string, string>
  description_i18n?: Record<string, string>
  rfx_type?: string
  owner_company_id?: string
}

export interface RfxUpdateTemplateRequest {
  name_i18n?: Record<string, string>
  description_i18n?: Record<string, string>
  rfx_type?: string
  expected_version: number
}

export interface RfxPublishTemplateVersionRequest {
  expected_template_version: number
  expected_draft_version: number
  change_summary: string
}

export interface ListRfxTemplatesFilters {
  status?: RfxTemplateAggregateStatus | ''
  owner_company_id?: string
  rfx_type?: string
  search?: string
  limit?: number
  offset?: number
}

export function isTemplateVersionEditable(status: RfxTemplateVersionStatus): boolean {
  return status === 'DRAFT'
}

export function isTemplateVersionReadOnly(status: RfxTemplateVersionStatus): boolean {
  return status === 'PUBLISHED' || status === 'SUPERSEDED'
}

export function canCloneFromTemplateVersion(
  aggregateStatus: RfxTemplateAggregateStatus,
  versionStatus: RfxTemplateVersionStatus,
): boolean {
  if (aggregateStatus === 'ARCHIVED') return false
  return versionStatus === 'PUBLISHED' || versionStatus === 'SUPERSEDED'
}
