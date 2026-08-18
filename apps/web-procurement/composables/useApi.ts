import type { ApiErrorBody } from '~/types/api'
import {
  API_HEADER_AUTHORIZATION,
  API_HEADER_REQUEST_ID,
  API_HEADER_TENANT_ID,
  API_HEADER_USER_ID,
} from '~/constants/apiHeaders'
import { ApiError, TenantRequiredError } from '~/utils/apiClient'

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
    throw new TenantRequiredError()
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
  if (!options.skipAuth && authStore.user?.id) {
    headers[API_HEADER_USER_ID] = authStore.user.id
  }
  if (!options.skipTenant && tenantStore.tenantId) {
    headers[API_HEADER_TENANT_ID] = tenantStore.tenantId
  }
  if (tenantStore.currentCompanyId) {
    headers['X-Company-ID'] = tenantStore.currentCompanyId
  }
  return headers
}

async function fetchWithNetworkHandling(input: string, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(input, init)
  } catch (error) {
    if (isNetworkFetchError(error)) {
      throw new ApiError(0, {
        code: 'BACKEND_UNAVAILABLE',
        message: 'Backend unavailable',
        details: {},
      })
    }
    throw error
  }
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

  return {
    apiGet,
    apiPost,
    apiPut,
    apiPatch,
    apiDelete,
  }
}
