import type { PaginatedResponse } from '~/types/api'
import type {
  CustomFieldValuesResponse,
  FormTemplateDetail,
  ListFormTemplatesResponse,
  LowCodeEntityType,
  SaveCustomFieldValuesPayload,
  SaveCustomFieldValuesResponse,
} from '~/types/lowCode'
import { ApiError } from '~/composables/useApi'

export function useLowCodeApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPut } = useApi()
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

  async function resolvePublishedTemplate(entityType: string): Promise<FormTemplateDetail | null> {
    const list = await listFormTemplates(entityType)
    const summary = list.items.find((item) => item.entity_type === entityType) ?? list.items[0]
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
    getFormTemplate,
    getCustomFieldValues,
    saveCustomFieldValues,
    resolvePublishedTemplate,
    resolveDemoEntityId,
    getSaveErrorMessage,
    isApiUnavailableError,
    isLowCodeServiceError,
  }
}
