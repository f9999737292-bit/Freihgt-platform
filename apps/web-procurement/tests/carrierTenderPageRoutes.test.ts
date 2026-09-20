import { existsSync, readFileSync } from 'node:fs'
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

  it('mounts carrier XLSX only on the tender detail page', () => {
    const detail = readFileSync(join(pagesRoot, '[id]', 'index.vue'), 'utf8')
    const questionnaire = readFileSync(join(pagesRoot, '[id]', 'questionnaire.vue'), 'utf8')
    expect(detail).toContain('CarrierXlsxExchangePanel')
    expect(questionnaire).not.toContain('CarrierXlsxExchangePanel')
    expect(questionnaire).not.toContain('carrier-xlsx')
  })

  it('mounts late submission request on carrier tender and questionnaire pages', () => {
    const detail = readFileSync(join(pagesRoot, '[id]', 'index.vue'), 'utf8')
    const questionnaire = readFileSync(join(pagesRoot, '[id]', 'questionnaire.vue'), 'utf8')
    expect(detail).toContain('LateSubmissionRequestPanel')
    expect(questionnaire).toContain('LateSubmissionRequestPanel')
    expect(detail).toContain('carrier-late-submission-slot')
  })

  it('loads own-response after buyer-only lots and missing own-participant failures', () => {
    const detail = readFileSync(join(pagesRoot, '[id]', 'index.vue'), 'utf8')
    const loadWorkspace = detail.slice(detail.indexOf('async function loadWorkspace()'))
    expect(loadWorkspace).toMatch(/try \{\s*participant\.value = await getOwnParticipant/)
    expect(loadWorkspace).toMatch(/try \{\s*lots\.value = await listLots/)
    expect(loadWorkspace).toMatch(/try \{\s*await loadResponse\(\)/)
    const tenderFail = loadWorkspace.slice(
      loadWorkspace.indexOf('event.value = await getTender'),
      loadWorkspace.indexOf('participant.value = await getOwnParticipant'),
    )
    expect(tenderFail).toContain('return')
    expect(tenderFail).not.toContain('await loadResponse()')
  })

  it('loads tender metadata from the carrier-scoped invited event route', () => {
    const api = readFileSync(join(process.cwd(), 'composables', 'useCarrierRfxApi.ts'), 'utf8')
    expect(api).toContain('/api/v1/carrier/rfx-events/${encodeURIComponent(id)}')
    expect(api).not.toMatch(/async function getTender[\s\S]{0,240}\/api\/v1\/rfx-events\//)
  })
})
