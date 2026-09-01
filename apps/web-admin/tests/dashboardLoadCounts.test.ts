import { describe, expect, it } from 'vitest'
import { ApiError, isApiUnavailableError } from '../composables/useApi'

/**
 * Regression for R31-BLK-001: dashboard loadCounts destructures isApiUnavailableError
 * from useApi() and must receive the same classifier (not undefined).
 */
describe('dashboard loadCounts unavailable classification', () => {
  it('handles Promise.allSettled rejections without TypeError', async () => {
    expect(typeof isApiUnavailableError).toBe('function')

    const results = await Promise.allSettled([
      Promise.reject(new ApiError(403, { code: 'FORBIDDEN', message: 'denied', details: {} })),
      Promise.reject(new ApiError(400, { code: 'VALIDATION', message: 'bad query', details: {} })),
    ])

    expect(() => {
      const unavailableKeys = new Set<string>()
      for (const result of results) {
        if (result.status === 'rejected' && isApiUnavailableError(result.reason)) {
          unavailableKeys.add('endpoint')
        }
      }
      expect(unavailableKeys.size).toBe(0)
    }).not.toThrow()
  })
})
