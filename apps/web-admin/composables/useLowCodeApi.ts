import type { PaginatedResponse } from '~/types/api'
import type {
  AdminFormTemplateDetail,
  ClonePublishedTemplateToDraftResponse,
  CreateDraftFormTemplateResponse,
  CustomFieldValuesResponse,
  DraftFormTemplatePayload,
  FormTemplateDetail,
  ListAdminFormTemplatesParams,
  ListAuditEventsParams,
  ListAuditEventsResponse,
  ListFormTemplatesResponse,
  ListActiveFormTemplatesParams,
  LowCodeEntityType,
  MigrateToActivePayload,
  MigrateToActiveResponse,
  MigrationPreviewRequest,
  MigrationPreviewResponse,
  BatchMigrationPreviewRequest,
  BatchMigrationPreviewResponse,
  BatchMigrateToActivePayload,
  BatchMigrateToActiveResponse,
  SaveCustomFieldValuesPayload,
  SaveCustomFieldValuesResponse,
  TemplateExportEnvelope,
  ImportPreviewRequest,
  ImportPreviewResponse,
  ImportExecuteRequest,
  ImportExecuteResponse,
} from '~/types/lowCode'
import { ApiError } from '~/composables/useApi'

const ADMIN_FORM_TEMPLATES_PATH = '/api/v1/low-code/admin/form-templates'
const ADMIN_CUSTOM_FIELD_VALUES_PATH = '/api/v1/low-code/admin/custom-field-values'

export function useLowCodeApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPut, apiPost } = useApi()
  const { t } = useI18n()

  function tenantId() {
    return tenantStore.tenantId
  }

  function tenantQuery(extra: Record<string, string | number | undefined> = {}) {
    return { tenant_id: tenantId(), ...extra }
  }

  async function listFormTemplates(entityType?: string) {
    const query: Record<string, string | number | undefined> = tenantQuery()
    if (entityType?.trim()) {
      query.entity_type = entityType.trim()
    }
    const data = await apiGet<ListFormTemplatesResponse>('/api/v1/low-code/form-templates', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function listActiveFormTemplates(params: ListActiveFormTemplatesParams) {
    const query: Record<string, string | number | undefined> = tenantQuery({
      entity_type: params.entity_type.trim(),
    })
    if (params.code?.trim()) {
      query.code = params.code.trim()
    }
    const data = await apiGet<ListFormTemplatesResponse>('/api/v1/low-code/form-templates/active', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function loadActivePublishedTemplateIds(entityTypes: string[]) {
    const uniqueTypes = [...new Set(entityTypes.map((value) => value.trim()).filter(Boolean))]
    const ids = new Set<string>()
    for (const entityType of uniqueTypes) {
      const data = await listActiveFormTemplates({ entity_type: entityType })
      for (const item of data.items) {
        ids.add(item.id)
      }
    }
    return ids
  }

  async function getFormTemplate(id: string) {
    return apiGet<FormTemplateDetail>(`/api/v1/low-code/form-templates/${id}`, {
      query: tenantQuery(),
    })
  }

  async function getCustomFieldValues(entityType: string, entityId: string) {
    const data = await apiGet<CustomFieldValuesResponse>('/api/v1/low-code/custom-field-values', {
      query: tenantQuery({
        entity_type: entityType,
        entity_id: entityId,
      }),
    })
    return { ...data, items: data.items ?? [] }
  }

  async function saveCustomFieldValues(payload: SaveCustomFieldValuesPayload) {
    return apiPut<SaveCustomFieldValuesResponse>('/api/v1/low-code/custom-field-values', payload, {
      query: tenantQuery(),
    })
  }

  async function previewMigrationToActive(payload: MigrationPreviewRequest) {
    return apiPost<MigrationPreviewResponse>(`${ADMIN_CUSTOM_FIELD_VALUES_PATH}/migration-preview`, payload, {
      query: tenantQuery(),
    })
  }

  async function previewBatchMigrationToActive(payload: BatchMigrationPreviewRequest) {
    return apiPost<BatchMigrationPreviewResponse>(
      `${ADMIN_CUSTOM_FIELD_VALUES_PATH}/batch-migration-preview`,
      payload,
      { query: tenantQuery() },
    )
  }

  async function batchMigrateCustomFieldValuesToActive(payload: BatchMigrateToActivePayload) {
    return apiPost<BatchMigrateToActiveResponse>(
      `${ADMIN_CUSTOM_FIELD_VALUES_PATH}/batch-migrate-to-active`,
      payload,
      { query: tenantQuery() },
    )
  }

  async function migrateCustomFieldValuesToActive(payload: MigrateToActivePayload) {
    return apiPost<MigrateToActiveResponse>(`${ADMIN_CUSTOM_FIELD_VALUES_PATH}/migrate-to-active`, payload, {
      query: tenantQuery(),
    })
  }

  async function listAuditEvents(params: ListAuditEventsParams = {}) {
    const query: Record<string, string | number | undefined> = tenantQuery({
      limit: params.limit ?? 50,
    })
    if (params.entity_type?.trim()) query.entity_type = params.entity_type.trim()
    if (params.entity_id?.trim()) query.entity_id = params.entity_id.trim()
    if (params.action?.trim()) query.action = params.action.trim()

    const data = await apiGet<ListAuditEventsResponse>('/api/v1/low-code/audit-events', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function listAdminFormTemplates(params: ListAdminFormTemplatesParams = {}) {
    const query: Record<string, string | number | undefined> = tenantQuery({
      limit: params.limit ?? 50,
    })
    if (params.entity_type?.trim()) query.entity_type = params.entity_type.trim()
    if (params.status?.trim()) query.status = params.status.trim()

    const data = await apiGet<ListFormTemplatesResponse>(ADMIN_FORM_TEMPLATES_PATH, { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getAdminFormTemplate(id: string) {
    return apiGet<AdminFormTemplateDetail>(`${ADMIN_FORM_TEMPLATES_PATH}/${id}`, {
      query: tenantQuery(),
    })
  }

  async function createDraftFormTemplate(payload: DraftFormTemplatePayload) {
    return apiPost<CreateDraftFormTemplateResponse>(ADMIN_FORM_TEMPLATES_PATH, payload, {
      query: tenantQuery(),
    })
  }

  async function updateDraftFormTemplate(id: string, payload: DraftFormTemplatePayload) {
    return apiPut<AdminFormTemplateDetail>(`${ADMIN_FORM_TEMPLATES_PATH}/${id}`, payload, {
      query: tenantQuery(),
    })
  }

  async function publishDraftFormTemplate(id: string) {
    return apiPost<AdminFormTemplateDetail>(`${ADMIN_FORM_TEMPLATES_PATH}/${id}/publish`, undefined, {
      query: tenantQuery(),
    })
  }

  async function clonePublishedTemplateToDraft(id: string) {
    return apiPost<ClonePublishedTemplateToDraftResponse>(
      `${ADMIN_FORM_TEMPLATES_PATH}/${id}/clone-to-draft`,
      undefined,
      { query: tenantQuery() },
    )
  }

  async function exportFormTemplate(templateId: string) {
    return apiGet<TemplateExportEnvelope>(`${ADMIN_FORM_TEMPLATES_PATH}/${templateId}/export`, {
      query: tenantQuery(),
    })
  }

  async function previewImportFormTemplate(payload: ImportPreviewRequest) {
    return apiPost<ImportPreviewResponse>(`${ADMIN_FORM_TEMPLATES_PATH}/import-preview`, payload, {
      query: tenantQuery(),
    })
  }

  async function importFormTemplate(payload: ImportExecuteRequest) {
    return apiPost<ImportExecuteResponse>(`${ADMIN_FORM_TEMPLATES_PATH}/import`, payload, {
      query: tenantQuery(),
    })
  }

  function getAdminFormTemplateErrorMessage(error: unknown): string {
    if (error instanceof ApiError) {
      switch (error.code) {
        case 'TENANT_REQUIRED':
          return t('tenant.required')
        case 'VALIDATION_ERROR':
          if (error.message?.includes('cloned')) {
            return t('lowCode.cloneTemplateFailed')
          }
          return error.message || t('lowCode.validationError')
        case 'FORM_TEMPLATE_NOT_FOUND':
          return t('lowCode.errorFormTemplateNotFound')
        case 'FORM_TEMPLATE_NOT_DRAFT':
          return t('lowCode.publishedTemplatesCannotBeEdited')
        case 'FORM_TEMPLATE_CONFLICT':
          return t('lowCode.templateCodeConflict')
        case 'UNSUPPORTED_SCHEMA_VERSION':
          return t('lowCode.templateImportUnsupportedSchema')
        case 'IMPORT_PAYLOAD_TOO_LARGE':
          return t('lowCode.templateImportPayloadTooLarge')
        case 'FIELD_INVALID_TYPE':
          return t('lowCode.errorFieldInvalidType')
        case 'TEMPLATE_CODE_INVALID':
          return t('lowCode.templateCodeInvalid')
        case 'INTERNAL_ERROR':
          return t('common.error')
        default:
          return error.message || t('common.error')
      }
    }
    if (error instanceof Error) {
      if (error.message === 'INVALID_JSON') return t('lowCode.invalidJson')
      return error.message
    }
    return t('common.error')
  }

  async function resolvePublishedTemplate(entityType: string, code?: string): Promise<FormTemplateDetail | null> {
    const active = await listActiveFormTemplates({
      entity_type: entityType,
      code,
    })
    const summary = code
      ? active.items.find((item) => item.code === code)
      : active.items.find((item) => item.entity_type === entityType) ?? active.items[0]
    if (!summary) return null
    return getFormTemplate(summary.id)
  }

  function getSaveErrorMessage(error: unknown): string {
    if (error instanceof ApiError) {
      switch (error.code) {
        case 'TENANT_REQUIRED':
          return t('tenant.required')
        case 'FORM_TEMPLATE_NOT_FOUND':
          return t('lowCode.errorFormTemplateNotFound')
        case 'FIELD_NOT_FOUND':
          return t('lowCode.errorFieldNotFound')
        case 'FIELD_INVALID_TYPE':
          return t('lowCode.errorFieldInvalidType')
        case 'VALIDATION_RULE_FAILED':
          return t('lowCode.errorValidationRuleFailed')
        case 'SYSTEM_FIELD_PROTECTED':
          return t('lowCode.errorSystemFieldProtected')
        case 'READ_ONLY_FIELD_PROTECTED':
          return t('lowCode.errorReadOnlyFieldProtected')
        default:
          return error.message || t('lowCode.saveFailed')
      }
    }
    if (error instanceof Error) {
      if (error.message === 'INVALID_NUMBER') return t('lowCode.errorFieldInvalidType')
      if (error.message === 'INVALID_JSON') return t('lowCode.invalidJson')
      return error.message
    }
    return t('lowCode.saveFailed')
  }

  function getMigrationErrorMessage(error: unknown): string {
    if (error instanceof ApiError) {
      switch (error.code) {
        case 'TENANT_REQUIRED':
          return t('tenant.required')
        case 'VALIDATION_ERROR':
        case 'ENTITY_TYPE_INVALID':
        case 'ENTITY_ID_INVALID':
          return error.message || t('lowCode.migrationValidationError')
        case 'MIGRATION_BLOCKED':
          return t('lowCode.migrationBlockedMessage')
        case 'MIGRATION_WARNINGS_REQUIRE_CONFIRMATION':
          return t('lowCode.migrationWarningsRequireConfirmation')
        case 'BATCH_MIGRATION_BLOCKED':
          return t('lowCode.batchMigrationBlockedMessage')
        case 'BATCH_MIGRATION_WARNINGS_REQUIRE_CONFIRMATION':
          return t('lowCode.batchMigrationWarningsRequireConfirmation')
        case 'FORM_TEMPLATE_NOT_FOUND':
          return t('lowCode.errorFormTemplateNotFound')
        case 'INTERNAL_ERROR':
          return t('common.error')
        default:
          if (error.status >= 500 || error.status === 0) {
            return t('lowCode.migrationServerError')
          }
          return error.message || t('lowCode.migrationFailed')
      }
    }
    if (error instanceof Error) {
      return error.message
    }
    return t('lowCode.migrationFailed')
  }

  async function resolveActiveTemplateCode(entityType: string): Promise<string | null> {
    const data = await listActiveFormTemplates({ entity_type: entityType })
    const template =
      data.items.find((item) => item.entity_type === entityType) ?? data.items[0]
    return template?.code?.trim() || null
  }

  async function resolveDemoEntityId(entityType: LowCodeEntityType): Promise<string | null> {
    const tenant = tenantId()
    if (!tenant) return null

    if (entityType === 'TRANSPORT_ORDER') {
      const data = await apiGet<PaginatedResponse<{ id: string; order_number?: string }>>(
        '/api/v1/transport-orders',
        { query: { tenant_id: tenant, limit: 100, offset: 0 } },
      )
      return data.items?.find((item) => item.order_number === 'DEMO-TO-001')?.id ?? null
    }

    if (entityType === 'SHIPMENT') {
      const data = await apiGet<PaginatedResponse<{ id: string; shipment_number?: string }>>(
        '/api/v1/shipments',
        { query: { tenant_id: tenant, limit: 100, offset: 0 } },
      )
      return data.items?.find((item) => item.shipment_number === 'DEMO-SH-PLANNED')?.id ?? null
    }

    if (entityType === 'BILLING_REGISTER') {
      const data = await apiGet<PaginatedResponse<{ id: string; register_number?: string }>>(
        '/api/v1/billing-registers',
        { query: { tenant_id: tenant, limit: 100, offset: 0 } },
      )
      return data.items?.find((item) => item.register_number === 'DEMO-BR-001')?.id ?? null
    }

    if (entityType === 'FREIGHT_REQUEST') {
      const data = await apiGet<PaginatedResponse<{ id: string; freight_request_number?: string }>>(
        '/api/v1/freight-requests',
        { query: { tenant_id: tenant, limit: 100, offset: 0 } },
      )
      return data.items?.find((item) => item.freight_request_number === 'DEMO-FR-001')?.id ?? null
    }

    if (entityType === 'DOCUMENT') {
      const data = await apiGet<PaginatedResponse<{ id: string; document_number?: string }>>(
        '/api/v1/documents',
        { query: { tenant_id: tenant, limit: 100, offset: 0 } },
      )
      return data.items?.find((item) => item.document_number === 'DEMO-DOC-001')?.id ?? null
    }

    if (entityType === 'RFX') {
      const data = await apiGet<PaginatedResponse<{ id: string; rfx_number?: string }>>(
        '/api/v1/rfx-events',
        { query: { tenant_id: tenant, limit: 100, offset: 0 } },
      )
      return data.items?.find((item) => item.rfx_number === 'DEMO-RFX-001')?.id ?? null
    }

    return null
  }

  async function resolveDemoEmptyEntityId(entityType: LowCodeEntityType): Promise<string | null> {
    const tenant = tenantId()
    if (!tenant) return null

    if (entityType === 'TRANSPORT_ORDER') {
      const data = await apiGet<PaginatedResponse<{ id: string; order_number?: string }>>(
        '/api/v1/transport-orders',
        { query: { tenant_id: tenant, limit: 100, offset: 0 } },
      )
      return data.items?.find((item) => item.order_number === 'DEMO-TO-002')?.id ?? null
    }

    return null
  }

  async function resolveEntityStatus(entityType: string, entityId: string): Promise<string | null> {
    const tenant = tenantId()
    const id = entityId.trim()
    if (!tenant || !id) return null

    try {
      if (entityType === 'TRANSPORT_ORDER') {
        const data = await apiGet<{ status?: string }>(`/api/v1/transport-orders/${id}`, {
          query: { tenant_id: tenant },
        })
        return data.status?.trim() || null
      }

      if (entityType === 'SHIPMENT') {
        const data = await apiGet<{ status?: string }>(`/api/v1/shipments/${id}`, {
          query: { tenant_id: tenant },
        })
        return data.status?.trim() || null
      }

      if (entityType === 'BILLING_REGISTER') {
        const data = await apiGet<{ status?: string }>(`/api/v1/billing-registers/${id}`, {
          query: { tenant_id: tenant },
        })
        return data.status?.trim() || null
      }

      if (entityType === 'FREIGHT_REQUEST') {
        const data = await apiGet<{ status?: string }>(`/api/v1/freight-requests/${id}`, {
          query: { tenant_id: tenant },
        })
        return data.status?.trim() || null
      }

      if (entityType === 'DOCUMENT') {
        const data = await apiGet<{ document_status?: string }>(`/api/v1/documents/${id}`, {
          query: { tenant_id: tenant },
        })
        return data.document_status?.trim() || null
      }

      if (entityType === 'RFX') {
        const data = await apiGet<{ status?: string }>(`/api/v1/rfx-events/${id}`, {
          query: { tenant_id: tenant },
        })
        return data.status?.trim() || null
      }
    } catch {
      return null
    }

    return null
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  function isLowCodeServiceError(error: unknown): boolean {
    if (!(error instanceof ApiError)) return false
    return (
      error.status === 502
      || error.status === 503
      || error.status === 504
      || error.code === 'SERVICE_UNAVAILABLE'
      || error.code === 'BACKEND_UNAVAILABLE'
    )
  }

  return {
    listFormTemplates,
    listActiveFormTemplates,
    loadActivePublishedTemplateIds,
    getFormTemplate,
    getCustomFieldValues,
    saveCustomFieldValues,
    previewMigrationToActive,
    previewBatchMigrationToActive,
    migrateCustomFieldValuesToActive,
    batchMigrateCustomFieldValuesToActive,
    listAuditEvents,
    listAdminFormTemplates,
    getAdminFormTemplate,
    createDraftFormTemplate,
    updateDraftFormTemplate,
    publishDraftFormTemplate,
    clonePublishedTemplateToDraft,
    exportFormTemplate,
    previewImportFormTemplate,
    importFormTemplate,
    getAdminFormTemplateErrorMessage,
    resolvePublishedTemplate,
    resolveDemoEntityId,
    resolveDemoEmptyEntityId,
    resolveEntityStatus,
    getSaveErrorMessage,
    getMigrationErrorMessage,
    resolveActiveTemplateCode,
    isApiUnavailableError,
    isLowCodeServiceError,
  }
}
