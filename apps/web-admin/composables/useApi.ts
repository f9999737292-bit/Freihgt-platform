import type { ApiErrorBody } from '~/types/api'
import {
  API_HEADER_AUTHORIZATION,
  API_HEADER_REQUEST_ID,
  API_HEADER_TENANT_ID,
} from '~/constants/apiHeaders'

export class ApiError extends Error {
  code: string
  details: Record<string, unknown>
  preview?: Record<string, unknown>
  status: number

  constructor(status: number, body: ApiErrorBody['error']) {
    super(body.message)
    this.name = 'ApiError'
    this.code = body.code
    this.details = body.details ?? {}
    this.preview = body.preview
    this.status = status
  }
}

export class TenantRequiredError extends Error {
  constructor(message = 'Tenant ID is required') {
    super(message)
    this.name = 'TenantRequiredError'
  }
}

export function isApiUnavailableError(error: unknown): boolean {
  if (error instanceof ApiError) {
    return (
      error.status === 0
      || error.status >= 500
      || error.code === 'SERVICE_UNAVAILABLE'
      || error.code === 'BACKEND_UNAVAILABLE'
    )
  }
  return error instanceof TypeError
}

interface RequestOptions {
  skipAuth?: boolean
  skipTenant?: boolean
  headers?: Record<string, string>
  query?: Record<string, string | number | undefined>
}

function isNetworkFetchError(error: unknown): boolean {
  if (error instanceof TypeError) return true
  if (error instanceof DOMException && error.name === 'AbortError') return true
  return false
}

export function isBackendUnavailableError(error: unknown): boolean {
  if (error instanceof ApiError) {
    return error.code === 'BACKEND_UNAVAILABLE' || error.status === 0
  }
  return isNetworkFetchError(error)
}

function throwBackendUnavailable() {
  const { t } = useI18n()
  throw new ApiError(0, {
    code: 'BACKEND_UNAVAILABLE',
    message: t('backendStatus.gatewayUnavailableMessage'),
    details: {},
  })
}

async function fetchWithNetworkHandling(input: string, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(input, init)
  } catch (error) {
    if (isNetworkFetchError(error)) {
      throwBackendUnavailable()
    }
    throw error
  }
}

function buildUrl(path: string, query?: RequestOptions['query']) {
  const config = useRuntimeConfig()
  const base = config.public.apiBaseUrl.replace(/\/$/, '')
  const url = new URL(path.startsWith('http') ? path : `${base}${path}`)
  if (query) {
    for (const [key, value] of Object.entries(query)) {
      if (value !== undefined && value !== '') {
        url.searchParams.set(key, String(value))
      }
    }
  }
  return url.toString()
}

function ensureTenant(options: RequestOptions) {
  if (options.skipTenant) return

  const tenantStore = useTenantStore()
  if (!tenantStore.tenantId?.trim()) {
    const { t } = useI18n()
    throw new TenantRequiredError(t('tenant.required'))
  }
}

function buildHeaders(options: RequestOptions = {}) {
  const authStore = useAuthStore()
  const tenantStore = useTenantStore()
  const { locale } = useI18n()
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    [API_HEADER_REQUEST_ID]: crypto.randomUUID(),
    'X-Locale': locale.value,
    ...options.headers,
  }

  if (!options.skipAuth && authStore.token) {
    headers[API_HEADER_AUTHORIZATION] = `Bearer ${authStore.token}`
  }
  if (!options.skipTenant && tenantStore.tenantId) {
    headers[API_HEADER_TENANT_ID] = tenantStore.tenantId
  }
  if (tenantStore.currentCompanyId) {
    headers['X-Company-ID'] = tenantStore.currentCompanyId
  }
  return headers
}

async function handleResponse<T>(response: Response): Promise<T> {
  if (response.ok) {
    if (response.status === 204) {
      return undefined as T
    }
    const text = await response.text()
    return text ? (JSON.parse(text) as T) : (undefined as T)
  }

  let body: ApiErrorBody | null = null
  try {
    body = (await response.json()) as ApiErrorBody
  } catch {
    throw new ApiError(response.status, {
      code: 'INTERNAL_ERROR',
      message: response.statusText || 'Request failed',
      details: {},
    })
  }

  throw new ApiError(response.status, body!.error)
}

export function useApi() {
  async function apiGet<T>(path: string, options: RequestOptions = {}) {
    ensureTenant(options)
    const response = await fetchWithNetworkHandling(buildUrl(path, options.query), {
      method: 'GET',
      headers: buildHeaders(options),
    })
    return handleResponse<T>(response)
  }

  async function apiPost<T>(path: string, body?: unknown, options: RequestOptions = {}) {
    ensureTenant(options)
    const response = await fetchWithNetworkHandling(buildUrl(path, options.query), {
      method: 'POST',
      headers: buildHeaders(options),
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
    return handleResponse<T>(response)
  }

  async function apiPut<T>(path: string, body?: unknown, options: RequestOptions = {}) {
    ensureTenant(options)
    const response = await fetchWithNetworkHandling(buildUrl(path, options.query), {
      method: 'PUT',
      headers: buildHeaders(options),
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
    return handleResponse<T>(response)
  }

  async function apiPatch<T>(path: string, body?: unknown, options: RequestOptions = {}) {
    ensureTenant(options)
    const response = await fetchWithNetworkHandling(buildUrl(path, options.query), {
      method: 'PATCH',
      headers: buildHeaders(options),
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
    return handleResponse<T>(response)
  }

  async function apiDelete<T>(path: string, options: RequestOptions = {}) {
    ensureTenant(options)
    const response = await fetchWithNetworkHandling(buildUrl(path, options.query), {
      method: 'DELETE',
      headers: buildHeaders(options),
    })
    return handleResponse<T>(response)
  }

  async function checkGatewayHealth() {
    const uiStore = useUiStore()
    uiStore.setApiGatewayStatus('checking')
    try {
      const config = useRuntimeConfig()
      const response = await fetchWithNetworkHandling(`${config.public.apiBaseUrl.replace(/\/$/, '')}/health`, {
        method: 'GET',
        headers: { Accept: 'application/json' },
      })
      uiStore.setApiGatewayStatus(response.ok ? 'online' : 'offline')
      return response.ok
    } catch (error) {
      if (error instanceof ApiError && error.code === 'BACKEND_UNAVAILABLE') {
        uiStore.setApiGatewayStatus('offline')
        return false
      }
      uiStore.setApiGatewayStatus('offline')
      return false
    }
  }

  return {
    apiGet,
    apiPost,
    apiPut,
    apiPatch,
    apiDelete,
    checkGatewayHealth,
    ApiError,
    TenantRequiredError,
    isBackendUnavailableError,
  }
}

