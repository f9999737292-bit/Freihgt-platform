import { describe, expect, it } from 'vitest'

import type { CarrierCompanyMembership } from '@freight-platform/shared-ts/types'

import {
  applyFreshShipperMemberships,
  canEnterCarrierPortal,
  canEnterShipperPortal,
  carrierCompanies,
  mapServerMemberships,
  selectCarrierCompany,
  selectShipperCompany,
  shipperCompanies,
} from './access'
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

const shipper = (
  companyId: string,
  roles: string[],
  companyType = 'SHIPPER',
  status = 'ACTIVE',
): CarrierCompanyMembership => ({
  ...carrier(companyId, roles, status),
  companyType,
})

describe('shipper portal entry', () => {
  it('allows an active SHIPPER company with SHIPPER_ADMIN or SHIPPER_LOGIST', () => {
    expect(canEnterShipperPortal(['SHIPPER_ADMIN'], [shipper('co-a', ['SHIPPER_ADMIN'])])).toBe(true)
    expect(canEnterShipperPortal(['SHIPPER_LOGIST'], [shipper('co-a', ['SHIPPER_LOGIST'])])).toBe(true)
  })

  it('does not grant entry from a JWT role without an eligible shipper membership', () => {
    expect(canEnterShipperPortal(['SHIPPER_ADMIN'], [])).toBe(false)
    expect(canEnterShipperPortal(['SHIPPER_LOGIST'], [])).toBe(false)
  })

  it('denies forwarder, lsp, carrier, procurement, and driver companies', () => {
    expect(canEnterShipperPortal(['FORWARDER_MANAGER'], [shipper('co-f', ['FORWARDER_MANAGER'], 'FORWARDER')])).toBe(false)
    expect(canEnterShipperPortal(['SHIPPER_ADMIN'], [shipper('co-lsp', ['SHIPPER_ADMIN'], 'LSP')])).toBe(false)
    expect(canEnterShipperPortal(['CARRIER_ADMIN'], [shipper('co-c', ['CARRIER_ADMIN'], 'CARRIER')])).toBe(false)
    expect(canEnterShipperPortal(['PROCUREMENT_MANAGER'], [shipper('co-p', ['PROCUREMENT_MANAGER'], 'SHIPPER')])).toBe(false)
    expect(canEnterShipperPortal(['DRIVER'], [shipper('co-d', ['DRIVER'], 'CARRIER')])).toBe(false)
    expect(canEnterShipperPortal(['CONSIGNEE_OPERATOR'], [shipper('co-g', ['CONSIGNEE_OPERATOR'], 'CONSIGNEE')])).toBe(false)
  })

  it('keeps a shipper role that exists only on another company out of the selectable list', () => {
    const memberships = [
      shipper('co-carrier', ['SHIPPER_ADMIN'], 'CARRIER'),
      shipper('co-shipper', ['SHIPPER_LOGIST'], 'SHIPPER'),
    ]
    expect(shipperCompanies(memberships).map((item) => item.companyId)).toEqual(['co-shipper'])
    expect(() => selectShipperCompany(memberships, 'co-carrier')).toThrow(PortalClientError)
    expect(selectShipperCompany(memberships, 'co-shipper')).toBe('co-shipper')
  })

  it('clears a selected company that the fresh shipper list does not include', () => {
    const fresh = applyFreshShipperMemberships({
      accessToken: 'token',
      tenantId: 'tenant-1',
      selectedCompanyId: 'co-other',
      memberships: [],
      user: {
        id: 'user-1',
        tenantId: 'tenant-1',
        email: 'shipper@example.com',
        fullName: 'Shipper',
        roles: ['SHIPPER_ADMIN'],
      },
    }, [shipper('co-shipper', ['SHIPPER_ADMIN'])])
    expect(fresh.selectedCompanyId).toBeNull()
    expect(fresh.memberships.map((item) => item.companyId)).toEqual(['co-shipper'])
  })

  it('reads company_type from the server membership payload', () => {
    const [mapped] = mapServerMemberships([{
      membership_id: 'm-1',
      company_id: 'co-shipper',
      legal_name: 'Shipper Co',
      company_type: 'shipper',
      membership_status: 'ACTIVE',
      roles: [{ code: 'shipper_logist' }],
    }])
    expect(mapped?.companyType).toBe('SHIPPER')
    expect(shipperCompanies(mapped ? [mapped] : []).map((item) => item.companyId)).toEqual(['co-shipper'])
  })
})
