import { describe, expect, it } from 'vitest'
import {
  canCreateResponse,
  canSubmitResponse,
  formatDeadlineRemaining,
  isDeadlineExpired,
} from '~/types/carrierRfx'
import { filterCarrierMemberships, isCarrierMembership } from '~/utils/companyMembership'

describe('carrierRfx helpers', () => {
  it('detects expired deadline', () => {
    const past = new Date(Date.now() - 3600000).toISOString()
    expect(isDeadlineExpired(past)).toBe(true)
    expect(isDeadlineExpired(new Date(Date.now() + 3600000).toISOString())).toBe(false)
  })

  it('formats remaining time', () => {
    const future = new Date(Date.now() + 90 * 60000).toISOString()
    expect(formatDeadlineRemaining(future)).toMatch(/m/)
  })

  it('allows create only when not started and open', () => {
    expect(canCreateResponse('RESPONSES_OPEN', 'NOT_STARTED', new Date(Date.now() + 3600000).toISOString())).toBe(true)
    expect(canCreateResponse('RESPONSES_OPEN', 'DRAFT', new Date(Date.now() + 3600000).toISOString())).toBe(false)
    expect(canCreateResponse('DRAFT', 'NOT_STARTED', new Date(Date.now() + 3600000).toISOString())).toBe(false)
  })

  it('allows submit only from draft', () => {
    expect(canSubmitResponse('DRAFT', 'RESPONSES_OPEN', new Date(Date.now() + 3600000).toISOString())).toBe(true)
    expect(canSubmitResponse('SUBMITTED', 'RESPONSES_OPEN', new Date(Date.now() + 3600000).toISOString())).toBe(false)
  })
})

describe('carrier membership', () => {
  it('filters active carrier memberships', () => {
    const items = filterCarrierMemberships([
      {
        company_id: '1',
        membership_status: 'ACTIVE',
        company_type: 'CARRIER',
        roles: [{ role_id: 'r1', code: 'CARRIER_DISPATCHER', name: 'Dispatcher' }],
      },
      {
        company_id: '2',
        membership_status: 'ACTIVE',
        company_type: 'SHIPPER',
        roles: [{ role_id: 'r2', code: 'SHIPPER_ADMIN', name: 'Admin' }],
      },
    ])
    expect(items).toHaveLength(1)
    expect(isCarrierMembership(items[0]!)).toBe(true)
  })
})
