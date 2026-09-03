import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('appshell SSR policy', () => {
  it('disables SSR for authenticated SPA routes to avoid localStorage hydration mismatch', () => {
    const source = readFileSync(join(import.meta.dirname, '..', 'nuxt.config.ts'), 'utf8')
    expect(source).toMatch(/'\/\*\*'\s*:\s*\{[\s\S]*ssr:\s*false/)
  })
})
