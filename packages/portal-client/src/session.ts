import type { CarrierTabSession, PersistedCarrierTabSession } from '@freight-platform/shared-ts/types'

export const CARRIER_TAB_SESSION_STORAGE_KEY = 'freight_carrier_tab_session'
export const SHIPPER_TAB_SESSION_STORAGE_KEY = 'freight_shipper_tab_session'
/** Carrier office key. Existing callers keep this default. */
export const TAB_SESSION_STORAGE_KEY = CARRIER_TAB_SESSION_STORAGE_KEY

type SessionStore = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export function readTabSession(storage: SessionStore, storageKey = TAB_SESSION_STORAGE_KEY): CarrierTabSession | null {
  const raw = storage.getItem(storageKey)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as PersistedCarrierTabSession & { memberships?: unknown }
    if (!parsed?.accessToken || !parsed.user?.id || !parsed.user.tenantId) return null
    return {
      accessToken: parsed.accessToken,
      user: parsed.user,
      tenantId: parsed.user.tenantId,
      selectedCompanyId: parsed.selectedCompanyId ?? null,
      memberships: [],
    }
  } catch {
    return null
  }
}

export function writeTabSession(
  storage: SessionStore,
  session: CarrierTabSession,
  storageKey = TAB_SESSION_STORAGE_KEY,
): void {
  const persisted: PersistedCarrierTabSession = {
    accessToken: session.accessToken,
    user: session.user,
    tenantId: session.user.tenantId,
    selectedCompanyId: session.selectedCompanyId,
  }
  storage.setItem(storageKey, JSON.stringify(persisted))
}

export function clearTabSession(storage: SessionStore, storageKey = TAB_SESSION_STORAGE_KEY): void {
  storage.removeItem(storageKey)
}
