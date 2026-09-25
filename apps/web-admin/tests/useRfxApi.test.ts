import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const sourcePath = resolve(__dirname, '../composables/useRfxApi.ts')
const source = readFileSync(sourcePath, 'utf8')

function rfxQueryBlocks(sourceText: string): string[] {
  const blocks: string[] = []
  const patterns = [
    /listRfxEvents[\s\S]*?apiGet<PaginatedResponse<RfxEvent>>\('\/api\/v1\/rfx-events', \{ query \}\)/,
    /getRfxEvent[\s\S]*?apiGet<RfxEvent>\(`\/api\/v1\/rfx-events\/\$\{id\}`\)/,
    /updateRfxEvent[\s\S]*?apiPatch<RfxEvent>\(`\/api\/v1\/rfx-events\/\$\{id\}`, payload\)/,
    /publishRfxEvent[\s\S]*?apiPost<\{ id: string; status: string \}>\(\s*`\/api\/v1\/rfx-events\/\$\{id\}\/publish`,\s*\)/,
    /cancelRfxEvent[\s\S]*?apiPost<\{ id: string; status: string \}>\(\s*`\/api\/v1\/rfx-events\/\$\{id\}\/cancel`,\s*\)/,
    /listRfxParticipants[\s\S]*?apiGet<\{ items: RfxParticipant\[\] \}>\(\s*`\/api\/v1\/rfx-events\/\$\{rfxEventId\}\/participants`,\s*\{ query \},\s*\)/,
  ]
  for (const pattern of patterns) {
    const match = sourceText.match(pattern)
    if (match) blocks.push(match[0])
  }
  return blocks
}

describe('useRfxApi trusted tenant query contract (R3.1A.1)', () => {
  it('does not define tenantQuery helper', () => {
    expect(source).not.toMatch(/function tenantQuery\(/)
  })

  it('keeps body tenant_id for createRfxEvent', () => {
    expect(source).toMatch(/createRfxEvent[\s\S]*tenant_id: tenantId\(\)/)
  })

  it('keeps body tenant_id for addRfxParticipant', () => {
    expect(source).toMatch(/addRfxParticipant[\s\S]*tenant_id: tenantId\(\)/)
  })

  it('LIST url has no tenant_id and keeps legitimate filters', () => {
    const block = rfxQueryBlocks(source)[0]
    expect(block).toBeTruthy()
    expect(block).not.toContain('tenant_id')
    expect(block).toContain('limit: params.limit ?? 20')
    expect(block).toContain('offset: params.offset ?? 0')
    expect(block).toContain('rfx_type')
    expect(block).toContain('category')
    expect(block).toContain('status')
    expect(block).toContain('owner_company_id')
    expect(block).toContain('search')
  })

  it('DETAIL url has no tenant_id query', () => {
    const block = rfxQueryBlocks(source)[1]
    expect(block).toBeTruthy()
    expect(block).not.toMatch(/\{ query:/)
    expect(block).not.toContain('tenant_id')
  })

  it('UPDATE url has no tenant_id query', () => {
    const block = rfxQueryBlocks(source)[2]
    expect(block).toBeTruthy()
    expect(block).not.toMatch(/\{ query:/)
    expect(block).not.toContain('tenant_id')
  })

  it('PUBLISH url has no tenant_id query', () => {
    const block = rfxQueryBlocks(source)[3]
    expect(block).toBeTruthy()
    expect(block).not.toMatch(/\{ query:/)
    expect(block).not.toContain('tenant_id')
  })

  it('CANCEL url has no tenant_id query', () => {
    const block = rfxQueryBlocks(source)[4]
    expect(block).toBeTruthy()
    expect(block).not.toMatch(/\{ query:/)
    expect(block).not.toContain('tenant_id')
  })

  it('PARTICIPANTS url has no tenant_id and keeps status filter', () => {
    const block = rfxQueryBlocks(source)[5]
    expect(block).toBeTruthy()
    expect(block).not.toContain('tenant_id')
    expect(block).toContain('if (params.status) query.status = params.status')
  })
})
