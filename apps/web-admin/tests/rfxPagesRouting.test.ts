import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

const RFX_ID = '6aa74939-c406-480b-a38f-d5e349d57899'
const WEB_ADMIN_ROOT = fileURLToPath(new URL('..', import.meta.url))
const PAGES_ROOT = join(WEB_ADMIN_ROOT, 'pages', 'rfx')

const RfxDetailStub = { __page: 'RfxDetailPage' as const }
const RfxStudioStub = { __page: 'RfxStudioPage' as const }

function createFixedRfxRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/rfx/:id',
        name: 'rfx-id',
        component: RfxDetailStub,
      },
      {
        path: '/rfx/:id/studio',
        name: 'rfx-id-studio',
        component: RfxStudioStub,
      },
    ],
  })
}

describe('rfx pages route tree (F102-001)', () => {
  it('uses index.vue for detail and keeps studio as a sibling route file', () => {
    expect(existsSync(join(PAGES_ROOT, '[id]', 'index.vue'))).toBe(true)
    expect(existsSync(join(PAGES_ROOT, '[id]', 'studio', 'index.vue'))).toBe(true)
    expect(existsSync(join(PAGES_ROOT, '[id].vue'))).toBe(false)
  })

  it('maps detail and studio pages to distinct route components', async () => {
    const router = createFixedRfxRouter()

    await router.push(`/rfx/${RFX_ID}`)
    expect(router.currentRoute.value.matched.at(-1)?.components?.default).toBe(RfxDetailStub)

    await router.push(`/rfx/${RFX_ID}/studio`)
    expect(router.currentRoute.value.path).toBe(`/rfx/${RFX_ID}/studio`)
    expect(router.currentRoute.value.matched.at(-1)?.components?.default).toBe(RfxStudioStub)
    expect(router.currentRoute.value.matched.at(-1)?.components?.default).not.toBe(RfxDetailStub)
  })

  it('studio page loads questionnaire API without nested parent swallowing', () => {
    const studioSource = readFileSync(join(PAGES_ROOT, '[id]', 'studio', 'index.vue'), 'utf8')
    expect(studioSource).toContain('useRfxQuestionnaireApi')
    expect(studioSource).toContain('loadStudio')
    expect(studioSource).not.toContain('RfxDetailsCard')
  })
})
