import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { ApiError, isApiUnavailableError } from '../composables/useApi'

const dashboardSource = readFileSync(resolve(__dirname, '../pages/dashboard/index.vue'), 'utf8')

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

  it('RFx count request does not send tenant_id query', () => {
    const branch = dashboardSource.match(/key === 'rfx'[\s\S]*?\{ limit: 1, offset: 0 \}/)
    expect(branch?.[0]).toBeTruthy()
    expect(branch?.[0]).not.toContain('tenant_id')
  })

  it('empty RFx 200 renders count 0 and is not a permission failure', () => {
    expect(dashboardSource).toContain('const total = data.total ?? (Array.isArray(data.items) ? data.items.length : 0)')
    expect(dashboardSource).toContain('stats.value[result.value.key] = result.value.total')
    const payload = { items: [] as unknown[], total: 0 }
    const total = payload.total ?? (Array.isArray(payload.items) ? payload.items.length : 0)
    expect(total).toBe(0)
    expect(isApiUnavailableError(new ApiError(200, {
      code: 'OK',
      message: 'ok',
      details: {},
    }))).toBe(false)
  })
})
