import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const sourcePath = resolve(__dirname, '../composables/useRfxQuestionnaireApi.ts')
const source = readFileSync(sourcePath, 'utf8')

describe('useRfxQuestionnaireApi tenant query contract', () => {
  it('does not define tenantQuery helper', () => {
    expect(source).not.toMatch(/function tenantQuery\(/)
  })

  it('getStudio URL has no tenant_id query param', () => {
    expect(source).toMatch(/getStudio[\s\S]*apiGet<RfxStudioResponse>\(basePath\('\/studio'\)\)/)
    expect(source).not.toMatch(/getStudio[\s\S]*tenant_id/)
  })

  it('validatePublish URL has no tenant_id query param', () => {
    expect(source).toMatch(/validatePublish[\s\S]*apiPost<RfxPublishReadinessResult>\(basePath\('\/validate-publish'\)\)/)
    expect(source).not.toMatch(/validatePublish[\s\S]*tenant_id/)
  })

  it('section mutations have no tenant_id query param', () => {
    expect(source).not.toMatch(/createSection[\s\S]*tenant_id/)
    expect(source).not.toMatch(/updateSection[\s\S]*tenant_id/)
    expect(source).not.toMatch(/scheduleSectionUpdate[\s\S]*tenant_id/)
  })

  it('delete mutations do not append tenant_id to URL search params', () => {
    expect(source).not.toMatch(/searchParams\.set\(key, String\(value\)\)/)
  })

  it('merges debounced question patch payloads before autosave flush', () => {
    expect(source).toMatch(/function scheduleDebouncedMutation/)
    expect(source).toMatch(/scheduleQuestionUpdate[\s\S]*scheduleDebouncedMutation\(`question:\$\{questionId\}`/)
    expect(source).toMatch(/pendingDebouncedPayloads\.set\(key, merged\)/)
  })

  it('createOption waits for pending autosave patches before POST', () => {
    const fnStart = source.indexOf('async function createOption')
    expect(fnStart).toBeGreaterThan(-1)
    const fnBody = source.slice(fnStart, fnStart + 400)
    const flushIdx = fnBody.indexOf('await flushPendingPatches()')
    const postIdx = fnBody.indexOf('apiPost<RfxQuestionOption>')
    expect(flushIdx).toBeGreaterThan(-1)
    expect(postIdx).toBeGreaterThan(flushIdx)
  })

  it('flushPendingPatches executes queued debounced runners', () => {
    expect(source).toMatch(/pendingPatchRunners/)
    expect(source).toMatch(/await runDebouncedPatch\(key, runner\)/)
    expect(source).toMatch(/createOptionInFlight/)
  })

  it('createOption flushes again after loadStudio', () => {
    const fnStart = source.indexOf('async function createOption')
    const fnBody = source.slice(fnStart, fnStart + 900)
    const firstFlush = fnBody.indexOf('await flushPendingPatches()')
    const loadStudio = fnBody.indexOf('await loadStudio()')
    const secondFlush = fnBody.indexOf('await flushPendingPatches()', firstFlush + 1)
    expect(firstFlush).toBeGreaterThan(-1)
    expect(loadStudio).toBeGreaterThan(firstFlush)
    expect(secondFlush).toBeGreaterThan(loadStudio)
  })
})
