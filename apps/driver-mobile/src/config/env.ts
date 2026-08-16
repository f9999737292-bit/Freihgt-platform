export function getApiBaseUrl(): string {
  const base = import.meta.env.VITE_API_BASE_URL?.trim() || 'http://localhost:8080'
  return base.replace(/\/$/, '')
}

export function getPilotTenantId(): string {
  return import.meta.env.VITE_PILOT_TENANT_ID?.trim() || ''
}

export function getApiTimeoutMs(): number {
  const raw = Number(import.meta.env.VITE_API_TIMEOUT_MS ?? 30000)
  return Number.isFinite(raw) && raw > 0 ? raw : 30000
}

export const SESSION_STORAGE_KEY = 'freight_driver_mobile_session'
