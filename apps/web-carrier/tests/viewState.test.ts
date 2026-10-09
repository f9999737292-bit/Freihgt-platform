import { PortalClientError } from '@freight-platform/portal-client'
import { describe, expect, it } from 'vitest'

import { canEnterCarrierPortal, selectCarrierCompany } from '@freight-platform/portal-client'
import { viewKindFromError } from '../domain/viewState'
import { fleetAllowsCreate } from '../domain/fleet'

describe('portal view states', () => {
  it('maps required error branches', () => {
    expect(viewKindFromError(new PortalClientError('no', 401))).toBe('unauthorized')
    expect(viewKindFromError(new PortalClientError('no', 403))).toBe('forbidden')
    expect(viewKindFromError(new PortalClientError('spoof', 403, 'COMPANY_SPOOF'))).toBe('forbidden')
    expect(viewKindFromError(new PortalClientError('missing', 404))).toBe('not_found')
    expect(viewKindFromError(new PortalClientError('down', 503))).toBe('unavailable')
    expect(viewKindFromError(new PortalClientError('network', 0, 'UNAVAILABLE'))).toBe('unavailable')
    expect(viewKindFromError(new TypeError('failed to fetch'))).toBe('unavailable')
  })
})

describe('carrier role and company branches', () => {
  it('rejects a non-carrier role and a spoofed company', () => {
    expect(canEnterCarrierPortal(['DRIVER'])).toBe(false)
    expect(canEnterCarrierPortal(['SHIPPER_ADMIN'])).toBe(false)
    expect(canEnterCarrierPortal(['CARRIER_DISPATCHER'])).toBe(true)
    expect(() => selectCarrierCompany([
      {
        membershipId: 'm-1',
        companyId: 'co-1',
        legalName: 'Carrier Co',
        membershipStatus: 'ACTIVE',
        roleCodes: ['CARRIER_ADMIN'],
      },
    ], 'co-other')).toThrow(/not a server-returned carrier membership/)
  })

  it('keeps fleet create closed for both carrier office roles', () => {
    expect(fleetAllowsCreate('CARRIER_ADMIN')).toBe(false)
    expect(fleetAllowsCreate('CARRIER_DISPATCHER')).toBe(false)
  })
})
