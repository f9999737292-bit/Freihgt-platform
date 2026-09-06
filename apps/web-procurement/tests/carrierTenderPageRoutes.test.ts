import { existsSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

/**
 * Regression: Nuxt cannot register sibling + nested dynamic routes when both
 * pages/carrier/tenders/[id].vue and pages/carrier/tenders/[id]/questionnaire.vue exist.
 * The detail page must live at [id]/index.vue so /questionnaire mounts CarrierResponseWorkspace.
 */
describe('carrier tender page routes', () => {
  const pagesRoot = join(process.cwd(), 'pages', 'carrier', 'tenders')

  it('uses [id]/index.vue instead of conflicting [id].vue at parent level', () => {
    expect(existsSync(join(pagesRoot, '[id]', 'index.vue'))).toBe(true)
    expect(existsSync(join(pagesRoot, '[id]', 'questionnaire.vue'))).toBe(true)
    expect(existsSync(join(pagesRoot, '[id].vue'))).toBe(false)
  })
})
