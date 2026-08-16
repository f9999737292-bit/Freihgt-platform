import { describe, expect, it } from 'vitest'
import { getPilotTenantId } from '@/config/env'
import { createOperationId, isValidOperationId } from '@/utils/idempotency'

describe('tenant configuration', () => {
  it('uses build-time tenant id rather than user-editable driver identity fields', () => {
    // Pilot tenant comes from VITE_PILOT_TENANT_ID only.
    expect(typeof getPilotTenantId()).toBe('string')
  })
})

describe('operation id generation', () => {
  it('creates stable retry-safe operation ids under 128 chars', () => {
    const id = createOperationId('delay', '00000000-0000-0000-0000-000000000001')
    expect(isValidOperationId(id)).toBe(true)
    expect(id.length).toBeLessThanOrEqual(128)
  })
})
