import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const sourcePath = resolve(__dirname, '../composables/useRfxScoreModelApi.ts')
const source = readFileSync(sourcePath, 'utf8')

describe('useRfxScoreModelApi contract', () => {
  it('uses score-model routes without tenant_id query', () => {
    expect(source).toMatch(/basePath\('\/score-model'\)/)
    expect(source).not.toMatch(/tenant_id/)
  })

  it('calls validate and publish endpoints', () => {
    expect(source).toMatch(/\/score-model\/validate/)
    expect(source).toMatch(/\/score-model\/publish/)
  })

  it('tracks editor state via deriveEditorState', () => {
    expect(source).toContain('deriveEditorState')
    expect(source).toContain('editorState')
  })
})
