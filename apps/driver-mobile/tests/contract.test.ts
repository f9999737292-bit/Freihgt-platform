import { describe, expect, it } from 'vitest'
import {
  buildDelayRequestPayload,
  buildExceptionRequestPayload,
  validateDelayRequest,
  validateExceptionRequest,
} from '@/utils/contractSchemas'

describe('MOBILE_DELAY_CONTRACT_TEST', () => {
  it('matches shipment-service delay contract', () => {
    const payload = buildDelayRequestPayload({
      reasonCode: 'TRAFFIC',
      reasonText: 'Heavy traffic on M11',
      idempotencyKey: 'driver-mobile-op:delay:shipment:1',
    })

    expect(validateDelayRequest(payload)).toEqual([])
    expect(payload).toEqual({
      reasonCode: 'TRAFFIC',
      reasonText: 'Heavy traffic on M11',
      idempotencyKey: 'driver-mobile-op:delay:shipment:1',
    })
  })

  it('rejects unsupported delay reason codes', () => {
    const errors = validateDelayRequest({
      reasonCode: 'WEATHER' as 'TRAFFIC',
      idempotencyKey: 'k',
    })
    expect(errors.some((item) => item.includes('reasonCode'))).toBe(true)
  })
})

describe('MOBILE_PROBLEM_CONTRACT_TEST', () => {
  it('matches OpenAPI DriverExceptionRequest enum', () => {
    const payload = buildExceptionRequestPayload({
      category: 'VEHICLE_BREAKDOWN',
      comment: 'Engine warning light',
      idempotencyKey: 'driver-mobile-op:problem:shipment:1',
    })

    expect(validateExceptionRequest(payload)).toEqual([])
    expect(payload.category).toBe('VEHICLE_BREAKDOWN')
  })

  it('rejects unsupported exception categories', () => {
    const errors = validateExceptionRequest({
      category: 'WEATHER' as 'OTHER',
      idempotencyKey: 'k',
    })
    expect(errors.some((item) => item.includes('category'))).toBe(true)
  })
})

describe('IDEMPOTENCY contract', () => {
  it('requires idempotencyKey in both delay and problem payloads', () => {
    expect(validateDelayRequest({ reasonCode: 'TRAFFIC', idempotencyKey: '' })).toContain('missing idempotencyKey')
    expect(validateExceptionRequest({ category: 'OTHER', idempotencyKey: '' })).toContain('missing idempotencyKey')
  })
})
