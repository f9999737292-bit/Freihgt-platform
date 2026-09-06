import type {
  CarrierAnswerPatchItem,
  CarrierPatchAnswersRequest,
  CarrierResponseSaveResult,
  CarrierResponseSubmitResult,
  CarrierResponseValidationResult,
  CarrierResponseWorkspace,
} from '~/types/carrierResponse'
import { carrierResponseApiPath } from '~/utils/carrierResponseApiRoutes'
import {
  CarrierConflictError,
  CarrierValidationError,
} from '~/utils/carrierResponseErrors'
import { parseCarrierValidation422Body } from '~/utils/carrierResponseValidation'
import { ApiError } from '~/utils/apiClient'
import {
  API_HEADER_AUTHORIZATION,
  API_HEADER_REQUEST_ID,
  API_HEADER_TENANT_ID,
  API_HEADER_USER_ID,
} from '~/constants/apiHeaders'

interface CarrierApiOptions {
  carrierCompanyId?: string
}

async function carrierFetch<T>(
  method: string,
  path: string,
  body?: unknown,
  options: CarrierApiOptions = {},
): Promise<T> {
  const config = useRuntimeConfig()
  const authStore = useAuthStore()
  const tenantStore = useTenantStore()
  const nuxtApp = useNuxtApp()
  const i18n = nuxtApp.$i18n as { locale?: { value?: string } } | undefined

  if (!tenantStore.tenantId?.trim()) {
    throw new ApiError(400, { code: 'TENANT_REQUIRED', message: 'Tenant ID is required', details: {} })
  }

  const base = config.public.apiBaseUrl.replace(/\/$/, '')
  const url = new URL(`${base}${path}`)
  if (options.carrierCompanyId) {
    url.searchParams.set('carrier_company_id', options.carrierCompanyId)
  }

  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    [API_HEADER_REQUEST_ID]: crypto.randomUUID(),
    'X-Locale': i18n?.locale?.value ?? 'ru-RU',
  }
  if (authStore.token) headers[API_HEADER_AUTHORIZATION] = `Bearer ${authStore.token}`
  if (authStore.user?.id) headers[API_HEADER_USER_ID] = authStore.user.id
  if (tenantStore.tenantId) headers[API_HEADER_TENANT_ID] = tenantStore.tenantId
  if (tenantStore.currentCompanyId) headers['X-Company-ID'] = tenantStore.currentCompanyId

  let response: Response
  try {
    response = await fetch(url.toString(), {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
  } catch {
    throw new ApiError(0, { code: 'BACKEND_UNAVAILABLE', message: 'Backend unavailable', details: {} })
  }

  if (response.ok) {
    if (response.status === 204) return undefined as T
    const text = await response.text()
    return text ? (JSON.parse(text) as T) : (undefined as T)
  }

  let payload: unknown = null
  try {
    payload = await response.json()
  } catch {
    throw new ApiError(response.status, {
      code: 'INTERNAL_ERROR',
      message: response.statusText || 'Request failed',
      details: {},
    })
  }

  if (response.status === 422) {
    throw new CarrierValidationError(parseCarrierValidation422Body(payload))
  }

  const errorBody = payload as { error?: { code: string; message: string; details?: Record<string, unknown> } }
  if (response.status === 409) {
    const err = errorBody.error
    throw new CarrierConflictError(
      err?.code ?? 'CONFLICT',
      err?.message ?? 'Conflict',
      err?.details ?? {},
    )
  }

  if (errorBody.error) {
    throw new ApiError(response.status, {
      code: errorBody.error.code,
      message: errorBody.error.message,
      details: errorBody.error.details ?? {},
    })
  }

  throw new ApiError(response.status, {
    code: 'INTERNAL_ERROR',
    message: 'Request failed',
    details: {},
  })
}

export function useCarrierResponseApi() {
  function getCarrierResponse(eventId: string, carrierCompanyId?: string) {
    return carrierFetch<CarrierResponseWorkspace>(
      'GET',
      carrierResponseApiPath(eventId),
      undefined,
      { carrierCompanyId },
    )
  }

  function startCarrierResponse(eventId: string, carrierCompanyId?: string) {
    return carrierFetch<CarrierResponseWorkspace>(
      'POST',
      carrierResponseApiPath(eventId, 'start'),
      {},
      { carrierCompanyId },
    )
  }

  function patchCarrierAnswers(
    eventId: string,
    body: CarrierPatchAnswersRequest,
    carrierCompanyId?: string,
  ) {
    return carrierFetch<CarrierResponseSaveResult>(
      'PATCH',
      carrierResponseApiPath(eventId, 'answers'),
      body,
      { carrierCompanyId },
    )
  }

  function validateCarrierResponse(eventId: string, carrierCompanyId?: string) {
    return carrierFetch<CarrierResponseValidationResult>(
      'POST',
      carrierResponseApiPath(eventId, 'validate'),
      {},
      { carrierCompanyId },
    )
  }

  function submitCarrierResponse(
    eventId: string,
    saveVersion: number,
    carrierCompanyId?: string,
  ) {
    return carrierFetch<CarrierResponseSubmitResult>(
      'POST',
      carrierResponseApiPath(eventId, 'submit'),
      { save_version: saveVersion },
      { carrierCompanyId },
    )
  }

  function getCarrierResponseSummary(eventId: string, carrierCompanyId?: string) {
    return carrierFetch<CarrierResponseValidationResult>(
      'GET',
      carrierResponseApiPath(eventId, 'summary'),
      undefined,
      { carrierCompanyId },
    )
  }

  return {
    getCarrierResponse,
    startCarrierResponse,
    patchCarrierAnswers,
    validateCarrierResponse,
    submitCarrierResponse,
    getCarrierResponseSummary,
  }
}

export type { CarrierAnswerPatchItem, CarrierPatchAnswersRequest }
