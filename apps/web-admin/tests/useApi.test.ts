import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { ApiError, isApiUnavailableError } from '../composables/useApi'

describe('useApi isApiUnavailableError export', () => {
  it('keeps isApiUnavailableError on the composable return object', () => {
    const source = readFileSync(resolve(__dirname, '../composables/useApi.ts'), 'utf8')
    expect(source).toContain('isApiUnavailableError,')
    expect(source).toMatch(/return\s*\{[\s\S]*isApiUnavailableError,[\s\S]*isBackendUnavailableError,[\s\S]*\}/)
  })

  it('classifies service and network failures', () => {
    expect(isApiUnavailableError(new ApiError(503, {
      code: 'SERVICE_UNAVAILABLE',
      message: 'Unavailable',
      details: {},
    }))).toBe(true)
    expect(isApiUnavailableError(new TypeError('Failed to fetch'))).toBe(true)
    expect(isApiUnavailableError(new ApiError(403, {
      code: 'FORBIDDEN',
      message: 'Forbidden',
      details: {},
    }))).toBe(false)
  })
})
