import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

import type { CarrierTabSession } from '@freight-platform/shared-ts/types'

import { applyFreshMemberships } from './access'
import {
  CARRIER_TAB_SESSION_STORAGE_KEY,
  clearTabSession,
  readTabSession,
  SHIPPER_TAB_SESSION_STORAGE_KEY,
  TAB_SESSION_STORAGE_KEY,
  writeTabSession,
} from './session'

function memoryStore(): Storage {
  const data = new Map<string, string>()
  return {
    get length() {
      return data.size
    },
    clear: () => data.clear(),
    getItem: (key) => data.get(key) ?? null,
    key: (index) => [...data.keys()][index] ?? null,
    removeItem: (key) => data.delete(key),
    setItem: (key, value) => data.set(key, value),
  }
}

const session: CarrierTabSession = {
  accessToken: 'token-1',
  tenantId: 'tenant-1',
  selectedCompanyId: null,
  memberships: [],
  user: {
    id: 'user-1',
    tenantId: 'tenant-1',
    email: 'carrier@example.com',
    fullName: 'Carrier User',
    roles: ['CARRIER_DISPATCHER'],
  },
}

describe('tab session', () => {
  it('stores the access token in the provided session store only', () => {
    const sessionStorage = memoryStore()
    const localStorage = memoryStore()
    writeTabSession(sessionStorage, session)
    expect(sessionStorage.getItem(TAB_SESSION_STORAGE_KEY)).toContain('token-1')
    expect(localStorage.getItem(TAB_SESSION_STORAGE_KEY)).toBeNull()
    expect(readTabSession(sessionStorage)?.user.tenantId).toBe('tenant-1')
    clearTabSession(sessionStorage)
    expect(readTabSession(sessionStorage)).toBeNull()
  })

  it('does not persist memberships', () => {
    const sessionStorage = memoryStore()
    writeTabSession(sessionStorage, {
      ...session,
      selectedCompanyId: 'co-1',
      memberships: [{
        membershipId: 'm-spoof',
        companyId: 'co-spoof',
        legalName: 'Spoof Co',
        membershipStatus: 'ACTIVE',
        roleCodes: ['CARRIER_ADMIN'],
      }],
    })
    const stored = JSON.parse(sessionStorage.getItem(TAB_SESSION_STORAGE_KEY) ?? '{}') as Record<string, unknown>
    expect(stored.memberships).toBeUndefined()
    expect(Object.keys(stored).sort()).toEqual(['accessToken', 'selectedCompanyId', 'tenantId', 'user'])
    expect(readTabSession(sessionStorage)?.memberships).toEqual([])
  })

  it('rejects a company manufactured in tampered sessionStorage', () => {
    const sessionStorage = memoryStore()
    sessionStorage.setItem(TAB_SESSION_STORAGE_KEY, JSON.stringify({
      accessToken: 'token-1',
      tenantId: 'tenant-1',
      selectedCompanyId: 'co-spoof',
      memberships: [{
        membershipId: 'm-spoof',
        companyId: 'co-spoof',
        legalName: 'Spoof Co',
        membershipStatus: 'ACTIVE',
        roleCodes: ['CARRIER_ADMIN'],
      }],
      user: session.user,
    }))
    const hydrated = readTabSession(sessionStorage)
    expect(hydrated?.memberships).toEqual([])
    const fresh = applyFreshMemberships(hydrated!, [{
      membershipId: 'm-1',
      companyId: 'co-1',
      legalName: 'Carrier Co',
      membershipStatus: 'ACTIVE',
      roleCodes: ['CARRIER_DISPATCHER'],
    }])
    expect(fresh.memberships.map((item) => item.companyId)).toEqual(['co-1'])
    expect(fresh.selectedCompanyId).toBeNull()
  })

  it('keeps the shipper tab session on a separate key and still drops memberships', () => {
    const sessionStorage = memoryStore()
    writeTabSession(sessionStorage, {
      ...session,
      selectedCompanyId: 'co-shipper',
      memberships: [{
        membershipId: 'm-spoof',
        companyId: 'co-spoof',
        legalName: 'Spoof',
        membershipStatus: 'ACTIVE',
        companyType: 'SHIPPER',
        roleCodes: ['SHIPPER_ADMIN'],
      }],
    }, SHIPPER_TAB_SESSION_STORAGE_KEY)
    expect(sessionStorage.getItem(CARRIER_TAB_SESSION_STORAGE_KEY)).toBeNull()
    expect(sessionStorage.getItem(TAB_SESSION_STORAGE_KEY)).toBeNull()
    const stored = JSON.parse(sessionStorage.getItem(SHIPPER_TAB_SESSION_STORAGE_KEY) ?? '{}') as Record<string, unknown>
    expect(stored.memberships).toBeUndefined()
    expect(readTabSession(sessionStorage, SHIPPER_TAB_SESSION_STORAGE_KEY)?.memberships).toEqual([])
    expect(readTabSession(sessionStorage, SHIPPER_TAB_SESSION_STORAGE_KEY)?.selectedCompanyId).toBe('co-shipper')
  })

  it('does not reference localStorage', () => {
    const source = readFileSync(new URL('./session.ts', import.meta.url), 'utf8')
    expect(source.includes('localStorage')).toBe(false)
  })
})
