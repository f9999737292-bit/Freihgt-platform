import { describe, expect, it } from 'vitest'
import { ApiError } from '~/utils/apiClient'
import { isNotFoundError, shouldShowNotFound } from '~/utils/apiError'

describe('apiError helpers', () => {
  it('detects not found errors', () => {
    expect(isNotFoundError(new ApiError(404, { code: 'NOT_FOUND', message: 'missing', details: {} }))).toBe(true)
    expect(isNotFoundError(new ApiError(403, { code: 'FORBIDDEN', message: 'denied', details: {} }))).toBe(false)
  })

  it('maps unauthorized cross-company access to not-found UI', () => {
    expect(shouldShowNotFound(new ApiError(404, { code: 'NOT_FOUND', message: 'missing', details: {} }))).toBe(true)
    expect(shouldShowNotFound(new ApiError(403, { code: 'FORBIDDEN', message: 'denied', details: {} }))).toBe(true)
    expect(shouldShowNotFound(new ApiError(400, { code: 'VALIDATION', message: 'bad', details: {} }))).toBe(false)
    expect(shouldShowNotFound(new Error('network'))).toBe(false)
  })

  it('classifies duck-typed 403/404 payloads without relying on instanceof', () => {
    expect(shouldShowNotFound({ status: 403, code: 'FORBIDDEN', message: 'denied' })).toBe(true)
    expect(shouldShowNotFound({ status: 404, code: 'NOT_FOUND', message: 'missing' })).toBe(true)
    expect(isNotFoundError({ status: 404, code: 'NOT_FOUND' })).toBe(true)
  })
})
