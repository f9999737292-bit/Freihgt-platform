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
})
