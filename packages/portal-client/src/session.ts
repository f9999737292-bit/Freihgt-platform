import type { CarrierTabSession } from '@freight-platform/shared-ts/types'

export const TAB_SESSION_STORAGE_KEY = 'freight_carrier_tab_session'

type SessionStore = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export function readTabSession(storage: SessionStore): CarrierTabSession | null {
  const raw = storage.getItem(TAB_SESSION_STORAGE_KEY)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as CarrierTabSession
    if (!parsed?.accessToken || !parsed.user?.id || !parsed.user.tenantId) return null
    return parsed
  } catch {
    return null
  }
}

export function writeTabSession(storage: SessionStore, session: CarrierTabSession): void {
  storage.setItem(TAB_SESSION_STORAGE_KEY, JSON.stringify({
    accessToken: session.accessToken,
    user: session.user,
    tenantId: session.user.tenantId,
    selectedCompanyId: session.selectedCompanyId,
    memberships: session.memberships,
  }))
}

export function clearTabSession(storage: SessionStore): void {
  storage.removeItem(TAB_SESSION_STORAGE_KEY)
}
