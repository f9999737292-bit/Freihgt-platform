import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('nuxt deploy cache policy', () => {
  it('keeps HTML uncached while hashed assets stay immutable', () => {
    const source = readFileSync(join(import.meta.dirname, '..', 'nuxt.config.ts'), 'utf8')
    expect(source).toContain("'Cache-Control': 'no-cache, no-store, must-revalidate'")
    expect(source).toContain("'Cache-Control': 'public, max-age=31536000, immutable'")
  })
})
