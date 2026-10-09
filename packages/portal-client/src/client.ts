import type { CarrierCompanyMembership, ServerUserSnapshot } from '@freight-platform/shared-ts/types'

import { assertSessionIdentity, mapServerMemberships } from './access'
import { PortalClientError } from './errors'
import { assertScopedCompanyFields, buildPortalHeaders, COMPANY_SCOPE_FIELDS } from './headers'

export interface PortalRequest {
  method?: string
  query?: Record<string, string | number | undefined | null>
  body?: unknown
  companyId?: string | null
  anonymous?: boolean
}

export interface PortalClientOptions {
  baseUrl: string
  fetchImpl?: typeof fetch
  getAccessToken: () => string | null
  getAllowedCompanyIds: () => readonly string[]
  getLocale?: () => string | null
  onUnauthorized?: () => void
}

interface LoginResponse {
  access_token: string
  token_type: string
  expires_in: number
  user: {
    id: string
    tenant_id: string
    email: string
    full_name: string
    roles?: string[]
  }
}

export class PortalClient {
  constructor(private readonly options: PortalClientOptions) {}

  async request<T>(path: string, init: PortalRequest = {}): Promise<T> {
    const url = resolveGatewayUrl(this.options.baseUrl, path)
    const allowed = this.options.getAllowedCompanyIds()
    if (init.companyId) {
      assertScopedCompanyFields({ company_id: init.companyId }, allowed)
    }
    if (init.query) {
      for (const key of COMPANY_SCOPE_FIELDS) {
        const value = init.query[key]
        if (typeof value === 'string' && value !== '') {
          assertScopedCompanyFields({ [key]: value }, allowed)
        }
      }
    }
    if (init.body !== undefined) {
      assertScopedCompanyFields(init.body, allowed)
    }

    const headers = buildPortalHeaders({
      accessToken: init.anonymous ? null : this.options.getAccessToken(),
      companyId: init.companyId,
      allowedCompanyIds: allowed,
      locale: this.options.getLocale?.() ?? null,
    })
    if (init.body !== undefined) {
      headers['Content-Type'] = 'application/json'
    }

    const method = init.method ?? (init.body === undefined ? 'GET' : 'POST')
    appendQuery(url, init.query)

    let response: Response
    try {
      response = await (this.options.fetchImpl ?? fetch)(url.toString(), {
        method,
        headers,
        body: init.body === undefined ? undefined : JSON.stringify(init.body),
      })
    } catch (error) {
      throw new PortalClientError(
        error instanceof Error ? error.message : 'network error',
        0,
        'UNAVAILABLE',
      )
    }

    const payload = await readBody(response)
    if (response.status === 401) {
      this.options.onUnauthorized?.()
      throw new PortalClientError(messageFrom(payload, 'unauthorized'), 401, 'UNAUTHORIZED', payload)
    }
    if (!response.ok) {
      throw new PortalClientError(
        messageFrom(payload, response.statusText || 'request failed'),
        response.status,
        codeForStatus(response.status),
        payload,
      )
    }
    return payload as T
  }

  login(input: { tenantId: string; email: string; password: string }): Promise<LoginResponse> {
    return this.request<LoginResponse>('/api/v1/auth/login', {
      method: 'POST',
      anonymous: true,
      body: {
        tenant_id: input.tenantId,
        email: input.email,
        password: input.password,
      },
    })
  }

  async listMemberships(
    user: ServerUserSnapshot,
    attempted?: { tenantId?: string; userId?: string },
  ): Promise<CarrierCompanyMembership[]> {
    assertSessionIdentity(user, attempted)
    const data = await this.request<{ items?: Parameters<typeof mapServerMemberships>[0] }>(
      `/api/v1/users/${encodeURIComponent(user.id)}/companies`,
      {
        query: {
          tenant_id: user.tenantId,
          status: 'ACTIVE',
        },
      },
    )
    return mapServerMemberships(data.items ?? [])
  }
}

export function userFromLogin(payload: LoginResponse): ServerUserSnapshot {
  return {
    id: payload.user.id,
    tenantId: payload.user.tenant_id,
    email: payload.user.email,
    fullName: payload.user.full_name,
    roles: payload.user.roles ?? [],
  }
}

function resolveGatewayUrl(baseUrl: string, path: string): URL {
  if (path.startsWith('http://') || path.startsWith('https://')) {
    throw new PortalClientError('domain service URL rejected', 0, 'GATEWAY_ONLY')
  }
  if (!path.startsWith('/api/')) {
    throw new PortalClientError('path must be an API gateway route', 0, 'GATEWAY_ONLY')
  }
  const base = baseUrl.endsWith('/') ? baseUrl : `${baseUrl}/`
  return new URL(path.slice(1), base)
}

function appendQuery(url: URL, query: PortalRequest['query']): void {
  if (!query) return
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === '') continue
    url.searchParams.set(key, String(value))
  }
}

async function readBody(response: Response): Promise<unknown> {
  const text = await response.text()
  if (!text) return null
  try {
    return JSON.parse(text) as unknown
  } catch {
    return text
  }
}

function messageFrom(payload: unknown, fallback: string): string {
  if (payload && typeof payload === 'object' && 'message' in payload) {
    const message = (payload as { message?: unknown }).message
    if (typeof message === 'string' && message.trim() !== '') return message
  }
  return fallback
}

function codeForStatus(status: number): string {
  if (status === 403) return 'FORBIDDEN'
  if (status === 404) return 'NOT_FOUND'
  if (status >= 500) return 'UNAVAILABLE'
  return 'REJECTED'
}
