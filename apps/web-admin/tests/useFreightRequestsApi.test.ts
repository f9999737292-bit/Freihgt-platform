import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const sourcePath = resolve(__dirname, '../composables/useFreightRequestsApi.ts')
const source = readFileSync(sourcePath, 'utf8')

function freightRequestsQueryBlocks(sourceText: string): string[] {
  const blocks: string[] = []
  const patterns = [
    /listFreightRequests[\s\S]*?apiGet<PaginatedResponse<FreightRequest>>\('\/api\/v1\/freight-requests', \{ query \}\)/,
    /getFreightRequest[\s\S]*?apiGet<FreightRequest>\(`\/api\/v1\/freight-requests\/\$\{id\}`\)/,
    /publishFreightRequest[\s\S]*?apiPost<\{ id: string; status: string \}>\(`\/api\/v1\/freight-requests\/\$\{id\}\/publish`\)/,
    /listFreightRequestBids[\s\S]*?apiGet<\{ items: Bid\[\] \}>\(`\/api\/v1\/freight-requests\/\$\{id\}\/bids`/,
  ]
  for (const pattern of patterns) {
    const match = sourceText.match(pattern)
    if (match) blocks.push(match[0])
  }
  return blocks
}

describe('useFreightRequestsApi RFx tenant query contract (R3.1B)', () => {
  it('does not define tenantQuery helper for RFx URL identity', () => {
    expect(source).not.toMatch(/function tenantQuery\(/)
  })

  it('keeps body tenant_id for createFreightRequestFromTransportOrder', () => {
    expect(source).toMatch(/createFreightRequestFromTransportOrder[\s\S]*tenant_id: tenantId\(\)/)
  })

  it('keeps body tenant_id for createBid', () => {
    expect(source).toMatch(/createBid[\s\S]*tenant_id: tenantId\(\)/)
  })

  it('listFreightRequests URL has no tenant_id query param', () => {
    const block = freightRequestsQueryBlocks(source)[0]
    expect(block).toBeTruthy()
    expect(block).not.toContain('tenant_id')
    expect(block).toContain('limit: params.limit ?? 20')
    expect(block).toContain('offset: params.offset ?? 0')
    expect(block).toContain('request_type')
    expect(block).toContain('shipper_company_id')
    expect(block).toContain('search')
  })

  it('getFreightRequest URL has no tenant_id query param', () => {
    const block = freightRequestsQueryBlocks(source)[1]
    expect(block).toBeTruthy()
    expect(block).not.toMatch(/\{ query:/)
    expect(block).not.toContain('tenant_id')
  })

  it('publishFreightRequest URL has no tenant_id query param', () => {
    const block = freightRequestsQueryBlocks(source)[2]
    expect(block).toBeTruthy()
    expect(block).not.toMatch(/\{ query:/)
    expect(block).not.toContain('tenant_id')
  })

  it('listFreightRequestBids URL has no tenant_id query param and keeps status filter', () => {
    const block = freightRequestsQueryBlocks(source)[3]
    expect(block).toBeTruthy()
    expect(block).not.toContain('tenant_id')
    expect(block).toContain('if (params.status) query.status = params.status')
  })
})

describe('useFreightRequestsApi regression — detail/publish/bids tenant query', () => {
  it('DETAIL_TENANT_QUERY_BEFORE=YES (historical defect documented)', () => {
    const before = "return apiGet<FreightRequest>(`/api/v1/freight-requests/${id}`, { query: tenantQuery() })"
    expect(before).toContain('tenantQuery()')
  })

  it('DETAIL_TENANT_QUERY_AFTER=NO', () => {
    expect(source).not.toMatch(
      /getFreightRequest[\s\S]*query:\s*tenantQuery\(\)/,
    )
  })

  it('PUBLISH_TENANT_QUERY_AFTER=NO', () => {
    expect(source).not.toMatch(
      /publishFreightRequest[\s\S]*query:\s*tenantQuery\(\)/,
    )
  })

  it('BIDS_LIST_TENANT_QUERY_AFTER=NO', () => {
    expect(source).not.toMatch(
      /listFreightRequestBids[\s\S]*tenantQuery\(\)/,
    )
  })
})
