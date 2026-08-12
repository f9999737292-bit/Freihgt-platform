import type { ApiErrorBody } from '~/types/api'

export class ApiError extends Error {
  code: string
  details: Record<string, unknown>
  status: number

  constructor(status: number, body: ApiErrorBody['error']) {
    super(body.message)
    this.name = 'ApiError'
    this.code = body.code
    this.details = body.details ?? {}
    this.status = status
  }
}

interface RequestOptions {
  skipAuth?: boolean
  headers?: Record<string, string>
  query?: Record<string, string | number | undefined>
}

function isNetworkFetchError(error: unknown): boolean {
  return error instanceof TypeError
}

function buildUrl(path: string, query?: RequestOptions['query']) {
  const config = useRuntimeConfig()
  const base = config.public.apiBaseUrl.replace(/\/$/, '')
  const url = new URL(path.startsWith('http') ? path : `${base}${path}`)
  if (query) {
    for (const [key, value] of Object.entries(query)) {
      if (value !== undefined) {
        url.searchParams.set(key, String(value))
      }
    }
  }
  return url.toString()
}

function buildHeaders(options: RequestOptions = {}) {
  const { token } = useSession()
  const { locale } = useI18n()
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    'X-Locale': locale.value,
    ...options.headers,
  }

  if (!options.skipAuth && token.value) {
    headers.Authorization = `Bearer ${token.value}`
  }

  return headers
}

async function fetchWithNetworkHandling(input: string, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(input, init)
  } catch (error) {
    if (isNetworkFetchError(error)) {
      throw new ApiError(0, {
        code: 'NETWORK_ERROR',
        message: 'Network error',
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
    const response = await fetchWithNetworkHandling(buildUrl(path, options.query), {
      method: 'GET',
      headers: buildHeaders(options),
    })
    return handleResponse<T>(response)
  }

  async function apiPost<T>(path: string, body?: unknown, options: RequestOptions = {}) {
    const response = await fetchWithNetworkHandling(buildUrl(path, options.query), {
      method: 'POST',
      headers: buildHeaders(options),
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
    return handleResponse<T>(response)
  }

  return {
    apiGet,
    apiPost,
    ApiError,
  }
}
