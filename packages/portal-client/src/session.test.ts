import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

import type { CarrierTabSession } from '@freight-platform/shared-ts/types'

import { clearTabSession, readTabSession, TAB_SESSION_STORAGE_KEY, writeTabSession } from './session'

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

  it('does not reference localStorage', () => {
    const source = readFileSync(new URL('./session.ts', import.meta.url), 'utf8')
    expect(source.includes('localStorage')).toBe(false)
  })
})
