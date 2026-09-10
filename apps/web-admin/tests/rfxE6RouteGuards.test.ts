/**
 * E6 route guard acceptance — buyer template/versioning pages fail closed.
 */
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const PAGES_ROOT = resolve(import.meta.dirname, '../pages/rfx')

const TEMPLATE_VERSIONING_PAGES = [
  'templates/index.vue',
  'templates/[id]/index.vue',
  'templates/[id]/versions/index.vue',
  'templates/[id]/versions/[versionId].vue',
]

const EVENT_VERSIONING_PAGES = [
  '[id]/versions/index.vue',
  '[id]/versions/[versionId].vue',
  '[id]/versions/compare.vue',
]

function readPage(relativePath: string): string {
  return readFileSync(resolve(PAGES_ROOT, relativePath), 'utf8')
}

describe('E6 route guards', () => {
  it('E6-RG-01 template library pages require auth and rfx-buyer-manage', () => {
    for (const rel of TEMPLATE_VERSIONING_PAGES) {
      const src = readPage(rel)
      expect(src, rel).toMatch(/middleware:\s*\[\s*'auth',\s*'rfx-buyer-manage'\s*\]/)
    }
  })

  it('E6-RG-02 event version pages require auth and rfx-buyer-manage', () => {
    for (const rel of EVENT_VERSIONING_PAGES) {
      const src = readPage(rel)
      expect(src, rel).toMatch(/middleware:\s*\[\s*'auth',\s*'rfx-buyer-manage'\s*\]/)
    }
  })

  it('E6-RG-03 rfx-buyer-manage middleware blocks carriers and disabled feature flag', () => {
    const middleware = readFileSync(resolve(import.meta.dirname, '../middleware/rfx-buyer-manage.ts'), 'utf8')
    expect(middleware).toMatch(/useRfxVersioningFeature\(\)/)
    expect(middleware).toMatch(/navigateTo\('\/rfx'\)/)
    expect(middleware).toMatch(/isCarrierWithoutBuyerAccess\(\)/)
    expect(middleware).toMatch(/canManageRfxTemplates\(\)/)
  })

  it('E6-RG-04 general rfx list/detail/studio remain auth-only entry points', () => {
    for (const rel of ['index.vue', '[id]/index.vue', '[id]/studio/index.vue']) {
      const src = readPage(rel)
      expect(src, rel).toContain("middleware: 'auth'")
      expect(src, rel).not.toContain('rfx-buyer-manage')
    }
  })
})
