import { describe, expect, it } from 'vitest'

import type { CarrierCompanyMembership } from '@freight-platform/shared-ts/types'

import { canEnterCarrierPortal, carrierCompanies, selectCarrierCompany } from './access'
import { PortalClientError } from './errors'

const carrier = (companyId: string, roles: string[], status = 'ACTIVE'): CarrierCompanyMembership => ({
  membershipId: `m-${companyId}`,
  companyId,
  legalName: companyId,
  membershipStatus: status,
  roleCodes: roles,
})

describe('carrier portal entry', () => {
  it('allows carrier office roles', () => {
    expect(canEnterCarrierPortal(['CARRIER_ADMIN'])).toBe(true)
    expect(canEnterCarrierPortal(['CARRIER_DISPATCHER'])).toBe(true)
  })

  it('denies driver, shipper, forwarder, consignee, and roles without carrier membership', () => {
    expect(canEnterCarrierPortal(['DRIVER'])).toBe(false)
    expect(canEnterCarrierPortal(['SHIPPER_ADMIN'])).toBe(false)
    expect(canEnterCarrierPortal(['SHIPPER_LOGIST'])).toBe(false)
    expect(canEnterCarrierPortal(['FORWARDER_MANAGER'])).toBe(false)
    expect(canEnterCarrierPortal(['CONSIGNEE_OPERATOR'])).toBe(false)
    expect(canEnterCarrierPortal(['PLATFORM_ADMIN'])).toBe(false)
    expect(canEnterCarrierPortal([])).toBe(false)
  })

  it('allows entry when the carrier role is on a server membership', () => {
    expect(canEnterCarrierPortal([], [carrier('co-1', ['CARRIER_DISPATCHER'])])).toBe(true)
    expect(canEnterCarrierPortal(['DRIVER'], [carrier('co-1', ['DRIVER'])])).toBe(false)
  })
})

describe('company selection', () => {
  const memberships = [
    carrier('co-carrier', ['CARRIER_ADMIN']),
    carrier('co-shipper', ['SHIPPER_ADMIN']),
    carrier('co-suspended', ['CARRIER_DISPATCHER'], 'SUSPENDED'),
  ]

  it('keeps only active carrier-role memberships', () => {
    expect(carrierCompanies(memberships).map((item) => item.companyId)).toEqual(['co-carrier'])
  })

  it('rejects a company that was not returned as a carrier membership', () => {
    expect(() => selectCarrierCompany(memberships, 'co-other')).toThrow(PortalClientError)
    expect(() => selectCarrierCompany(memberships, 'co-shipper')).toThrow(PortalClientError)
    expect(selectCarrierCompany(memberships, 'co-carrier')).toBe('co-carrier')
  })
})
