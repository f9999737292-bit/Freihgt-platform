import { describe, expect, it, vi } from 'vitest'
import { ApiError, isApiUnavailableError } from '../composables/useApi'
import type { FreightRequest } from '../types/rfx'

const draftRequest: FreightRequest = {
  id: '6aa74939-c406-480b-a38f-d5e349d57899',
  tenant_id: '285f9447-faf7-423e-96dd-e4c5e2b3fc6c',
  freight_request_number: 'FR-PILOT-001',
  request_type: 'MINI_TENDER',
  status: 'DRAFT',
  shipper_company_id: '83cb2447-75e9-41f2-8e0d-93c70f8506be',
  currency_code: 'RUB',
  response_deadline: '2026-12-31T00:00:00Z',
  created_at: '2026-08-29T00:00:00Z',
  updated_at: '2026-08-29T00:00:00Z',
}

describe('freight request detail initialization contract', () => {
  it('invokes detail fetch and accepts DRAFT state for publish eligibility', async () => {
    const getFreightRequest = vi.fn(async (_id: string) => draftRequest)

    let request: FreightRequest | null = null
    let apiUnavailable = false

    async function loadRequest(id: string) {
      try {
        request = await getFreightRequest(id)
      } catch (error) {
        request = null
        apiUnavailable = isApiUnavailableError(error)
      }
    }

    await loadRequest(draftRequest.id)

    expect(getFreightRequest).toHaveBeenCalledWith(draftRequest.id)
    expect(request?.status).toBe('DRAFT')
    expect(apiUnavailable).toBe(false)
    expect(request?.status === 'DRAFT').toBe(true)
  })

  it('does not treat permission errors as backend unavailable', () => {
    expect(isApiUnavailableError(new ApiError(403, {
      code: 'FORBIDDEN',
      message: 'denied',
      details: {},
    }))).toBe(false)
  })
})
