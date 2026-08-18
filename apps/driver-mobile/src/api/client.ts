import { Preferences } from '@capacitor/preferences'
import { getApiBaseUrl, getApiTimeoutMs, SESSION_STORAGE_KEY } from '@/config/env'
import { DriverApiError, type ApiErrorBody, type RequestResult, type SubmissionOutcome } from '@/types/api'
import { newRequestId } from '@/utils/uuid'

export interface HttpClientOptions {
  getToken: () => string | null
  isOnline: () => boolean
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

function isNetworkError(error: unknown): boolean {
  return error instanceof TypeError
}

async function parseErrorResponse(response: Response): Promise<DriverApiError> {
  try {
    const body = (await response.json()) as ApiErrorBody
    if (body?.error?.message) {
      return new DriverApiError(response.status, body.error)
    }
  } catch {
    // fall through
  }
  return new DriverApiError(response.status, {
    code: 'HTTP_ERROR',
    message: response.statusText || `HTTP ${response.status}`,
  })
}

function toBodyInit(raw: Blob | ArrayBuffer | Uint8Array): BodyInit {
  if (raw instanceof Blob) return raw
  if (raw instanceof ArrayBuffer) return raw
  return new Blob([Uint8Array.from(raw)])
}

export class HttpClient {
  constructor(private readonly options: HttpClientOptions) {}

  async request<T>(
    method: string,
    path: string,
    init?: {
      body?: unknown
      rawBody?: Blob | ArrayBuffer | Uint8Array
      idempotencyKey?: string
      skipAuth?: boolean
      query?: Record<string, string | number | undefined>
      extraHeaders?: Record<string, string>
    },
  ): Promise<RequestResult<T>> {
    if (!this.options.isOnline()) {
      return { outcome: 'REQUEST_NOT_SENT' }
    }

    let sent = false
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), getApiTimeoutMs())

    try {
      const url = new URL(path.startsWith('http') ? path : `${getApiBaseUrl()}${path}`)
      if (init?.query) {
        for (const [key, value] of Object.entries(init.query)) {
          if (value !== undefined && value !== '') {
            url.searchParams.set(key, String(value))
          }
        }
      }

      const headers: Record<string, string> = {
        Accept: 'application/json',
        'X-Request-ID': newRequestId(),
        ...init?.extraHeaders,
      }

      if (init?.body !== undefined) {
        headers['Content-Type'] = 'application/json'
      }

      if (init?.idempotencyKey) {
        headers['Idempotency-Key'] = init.idempotencyKey
      }

      if (!init?.skipAuth) {
        const token = this.options.getToken()
        if (!token) {
          return {
            outcome: 'SERVER_REJECTED',
            error: new DriverApiError(401, {
              code: 'UNAUTHORIZED',
              message: 'Authentication required',
            }),
          }
        }
        headers.Authorization = `Bearer ${token}`
      }

      sent = true
      const requestBody: BodyInit | undefined =
        init?.rawBody !== undefined
          ? toBodyInit(init.rawBody)
          : init?.body !== undefined
            ? JSON.stringify(init.body)
            : undefined
      const response = await fetch(url.toString(), {
        method,
        headers,
        body: requestBody,
        signal: controller.signal,
      })

      if (!response.ok) {
        return {
          outcome: 'SERVER_REJECTED',
          error: await parseErrorResponse(response),
        }
      }

      if (response.status === 204) {
        return { outcome: 'SUCCESS', data: undefined as T }
      }

      const data = (await response.json()) as T
      return { outcome: 'SUCCESS', data }
    } catch (error) {
      if (sent && (isAbortError(error) || isNetworkError(error))) {
        return { outcome: 'REQUEST_SENT_RESPONSE_UNKNOWN' }
      }
      if (!sent) {
        return { outcome: 'REQUEST_NOT_SENT' }
      }
      return { outcome: 'REQUEST_SENT_RESPONSE_UNKNOWN' }
    } finally {
      clearTimeout(timeout)
    }
  }
}

export async function persistSession(raw: string): Promise<void> {
  try {
    await Preferences.set({ key: SESSION_STORAGE_KEY, value: raw })
  } catch {
    sessionStorage.setItem(SESSION_STORAGE_KEY, raw)
  }
}

export async function loadPersistedSession(): Promise<string | null> {
  try {
    const { value } = await Preferences.get({ key: SESSION_STORAGE_KEY })
    if (value) return value
  } catch {
    // fallback below
  }
  return sessionStorage.getItem(SESSION_STORAGE_KEY)
}

export async function clearPersistedSession(): Promise<void> {
  try {
    await Preferences.remove({ key: SESSION_STORAGE_KEY })
  } catch {
    sessionStorage.removeItem(SESSION_STORAGE_KEY)
  }
}

export function mapOutcomeLabel(outcome: SubmissionOutcome): string {
  switch (outcome) {
    case 'REQUEST_NOT_SENT':
      return 'offline'
    case 'REQUEST_SENT_RESPONSE_UNKNOWN':
      return 'unknown'
    case 'SERVER_REJECTED':
      return 'rejected'
    case 'SUCCESS':
      return 'success'
  }
}
