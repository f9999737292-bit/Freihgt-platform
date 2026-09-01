import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const sourcePath = resolve(__dirname, '../composables/useBidsApi.ts')
const source = readFileSync(sourcePath, 'utf8')

describe('useBidsApi RFx bid tenant query contract (R3.1B.1)', () => {
  it('does not define tenantQuery helper', () => {
    expect(source).not.toMatch(/function tenantQuery\(/)
  })

  it('submitBid URL has no tenant_id query param', () => {
    expect(source).toMatch(/submitBid[\s\S]*?apiPost<\{ id: string; status: string \}>\(`\/api\/v1\/bids\/\$\{id\}\/submit`\)/)
    expect(source).not.toMatch(/submitBid[\s\S]*?\{ query:/)
    expect(source).not.toMatch(/submitBid[\s\S]*?tenant_id/)
  })

  it('acceptBid URL has no tenant_id query param', () => {
    expect(source).toMatch(/acceptBid[\s\S]*?apiPost<\{ id: string; status: string \}>\(`\/api\/v1\/bids\/\$\{id\}\/accept`\)/)
    expect(source).not.toMatch(/acceptBid[\s\S]*?\{ query:/)
    expect(source).not.toMatch(/acceptBid[\s\S]*?tenant_id/)
  })
})

describe('useBidsApi regression — submit/accept tenant query', () => {
  it('SUBMIT_BID_TENANT_QUERY_BEFORE=YES (historical defect documented)', () => {
    const before = "query: tenantQuery()"
    expect(before).toContain('tenantQuery()')
  })

  it('SUBMIT_BID_TENANT_QUERY_AFTER=NO', () => {
    expect(source).not.toMatch(/submitBid[\s\S]*tenantQuery\(\)/)
  })

  it('ACCEPT_BID_TENANT_QUERY_AFTER=NO', () => {
    expect(source).not.toMatch(/acceptBid[\s\S]*tenantQuery\(\)/)
  })
})
