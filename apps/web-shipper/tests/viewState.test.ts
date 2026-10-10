import { PortalClientError } from '@freight-platform/portal-client'
import { describe, expect, it } from 'vitest'

import { displayFact } from '../domain/shipment'
import { viewKindFromError } from '../domain/viewState'

describe('shipper view state', () => {
  it('maps gateway failures without keeping a live list', () => {
    expect(viewKindFromError(new PortalClientError('no', 401, 'UNAUTHORIZED'))).toBe('unauthorized')
    expect(viewKindFromError(new PortalClientError('no', 403, 'FORBIDDEN'))).toBe('forbidden')
    expect(viewKindFromError(new PortalClientError('no', 404, 'NOT_FOUND'))).toBe('not_found')
    expect(viewKindFromError(new PortalClientError('no', 500, 'UNAVAILABLE'))).toBe('unavailable')
    expect(viewKindFromError(new Error('network'))).toBe('unavailable')
  })

  it('does not invent a label for a missing server fact', () => {
    expect(displayFact(null)).toBe('—')
    expect(displayFact('  ')).toBe('—')
    expect(displayFact('ROAD')).toBe('ROAD')
  })
})
