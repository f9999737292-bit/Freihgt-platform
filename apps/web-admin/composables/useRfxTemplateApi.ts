import type { PaginatedResponse } from '~/types/api'
import type { RfxQuestionnaireDefinition } from '~/types/rfx-questionnaire'
import type {
  ListRfxTemplatesFilters,
  RfxCreateTemplateRequest,
  RfxPublishTemplateVersionRequest,
  RfxTemplateDetailResponse,
  RfxTemplateRecord,
  RfxTemplateVersionRecord,
  RfxUpdateTemplateRequest,
} from '~/types/rfx-template'
import { createIdempotencyKey } from '~/utils/idempotencyKey'
import { rfxTemplateApiPath } from '~/utils/rfxTemplateApiRoutes'

export function useRfxTemplateApi() {
  const { apiGet, apiPost, apiPatch } = useApi()

  function basePath(templateId: string, suffix = '') {
    return rfxTemplateApiPath(templateId, suffix)
  }

  async function listTemplates(params: ListRfxTemplatesFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    }
    if (params.status) query.status = params.status
    if (params.owner_company_id) query.owner_company_id = params.owner_company_id
    if (params.rfx_type) query.rfx_type = params.rfx_type
    if (params.search?.trim()) query.search = params.search.trim()

    const data = await apiGet<PaginatedResponse<RfxTemplateRecord>>('/api/v1/rfx-templates', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getTemplate(templateId: string) {
    return apiGet<RfxTemplateDetailResponse>(basePath(templateId))
  }

  async function createTemplate(payload: RfxCreateTemplateRequest) {
    return apiPost<RfxTemplateDetailResponse>('/api/v1/rfx-templates', payload)
  }

  async function updateTemplate(templateId: string, payload: RfxUpdateTemplateRequest) {
    return apiPatch<RfxTemplateDetailResponse>(basePath(templateId), payload)
  }

  async function archiveTemplate(templateId: string) {
    return apiPost<RfxTemplateRecord>(basePath(templateId, '/archive'))
  }

  async function publishTemplateVersion(
    templateId: string,
    payload: RfxPublishTemplateVersionRequest,
    idempotencyKey?: string,
  ) {
    return apiPost<RfxTemplateVersionRecord>(basePath(templateId, '/versions/publish'), payload, {
      headers: { 'Idempotency-Key': idempotencyKey ?? createIdempotencyKey('tpl-pub') },
    })
  }

  async function forkTemplateDraft(templateId: string, idempotencyKey?: string) {
    return apiPost<RfxTemplateVersionRecord>(basePath(templateId, '/versions/fork-draft'), {}, {
      headers: { 'Idempotency-Key': idempotencyKey ?? createIdempotencyKey('tpl-fork') },
    })
  }

  async function getTemplateVersionQuestionnaire(templateId: string, versionId: string) {
    return apiGet<RfxQuestionnaireDefinition>(basePath(templateId, `/versions/${versionId}/questionnaire`))
  }

  return {
    listTemplates,
    getTemplate,
    createTemplate,
    updateTemplate,
    archiveTemplate,
    publishTemplateVersion,
    forkTemplateDraft,
    getTemplateVersionQuestionnaire,
  }
}
