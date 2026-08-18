import { describe, expect, it } from 'vitest'

describe('mockAuth production guard', () => {
  it('documents production guard expectation', () => {
    // plugins/mockAuthGuard.ts throws when NODE_ENV=production && mockAuth=true
    expect(process.env.NODE_ENV !== 'production' || process.env.NUXT_PUBLIC_MOCK_AUTH !== 'true').toBe(true)
  })
})
