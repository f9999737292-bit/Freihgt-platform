import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { PortalClient } from '@freight-platform/portal-client'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DocumentWorkspace from '../components/documents/DocumentWorkspace.vue'
import {
  documentListSpec,
  documentMatchesShipment,
  documentReadSpec,
  downloadName,
  retainShipmentContext,
} from '../domain/document'
import type { PortalViewKind } from '../domain/viewState'

const documentRow = {
  id: 'doc-1',
  document_number: 'DOC-1',
  document_type: 'POD',
  document_status: 'SIGNED',
  related_entity_id: 'shp-1',
  legal_language: 'ru',
  created_at: '2026-12-01T10:00:00Z',
  updated_at: '2026-12-01T11:00:00Z',
  version: 2,
}

const attachment = {
  id: 'att-1',
  file_name: 'pod.pdf',
  media_type: 'application/pdf',
  size_bytes: 4,
  status: 'FINALIZED',
  created_at: '2026-12-01T10:00:00Z',
  finalized_at: '2026-12-01T10:05:00Z',
}

function workspace(overrides: Record<string, unknown> = {}) {
  return mount(DocumentWorkspace, {
    props: {
      shipmentId: 'shp-1',
      shipments: [{ id: 'shp-1', shipment_number: 'SHP-1', status: 'BOOKED' }],
      shipmentsKind: 'ready' as PortalViewKind,
      shipmentsTitle: 'Loading',
      documentsKind: 'ready' as PortalViewKind,
      documentsTitle: 'Loading',
      documents: [documentRow],
      documentsTotal: 1,
      documentsOffset: 0,
      documentId: null,
      documentKind: 'ready' as PortalViewKind,
      documentTitle: 'Loading',
      document: null,
      attachmentsKind: 'ready' as PortalViewKind,
      attachmentsTitle: 'Loading',
      attachments: [],
      ...overrides,
    },
  })
}

describe('document workspace', () => {
  it('requires a shipment before showing documents', () => {
    const view = workspace({ shipmentId: null, documents: [documentRow] })
    expect(view.get('[data-testid="shipment-picker"]').text()).toContain('shipper.chooseShipment')
    expect(view.find('[data-testid="document-row"]').exists()).toBe(false)
    expect(view.text()).not.toContain('DOC-1')
  })

  it('shows an empty document list without treating it as a failure', () => {
    const view = workspace({ documentsKind: 'empty', documents: [], documentsTitle: 'No documents' })
    expect(view.get('[data-testid="document-list"]').text()).toContain('No documents')
    expect(view.find('[data-testid="document-row"]').exists()).toBe(false)
  })

  it('shows document detail, attachments, and a named download control', async () => {
    const view = workspace({
      documentId: 'doc-1',
      document: documentRow,
      attachments: [attachment],
    })
    expect(view.get('[data-testid="document-number"]').text()).toBe('DOC-1')
    expect(view.get('[data-testid="attachment-row"]').text()).toContain('pod.pdf')
    const button = view.get('[data-testid="download-attachment"]')
    expect(button.attributes('aria-label')).toContain('pod.pdf')
    await button.trigger('click')
    expect(view.emitted('download')?.[0]).toEqual(['att-1'])
  })

  it('does not offer write, signing, or operator actions', () => {
    const view = workspace({ documentId: 'doc-1', document: documentRow, attachments: [attachment] })
    const source = readFileSync(join(process.cwd(), 'components/documents/DocumentWorkspace.vue'), 'utf8')
    const page = readFileSync(join(process.cwd(), 'pages/documents/index.vue'), 'utf8')
    for (const token of ['type="file"', 'signing-session', 'data-testid="sign', 'data-testid="upload', 'data-testid="archive', 'data-testid="finalize']) {
      expect(source).not.toContain(token)
      expect(page).not.toContain(token)
    }
    expect(view.find('[data-testid="download-attachment"]').exists()).toBe(true)
  })

  it('shows forbidden, missing, and unavailable states', () => {
    expect(workspace({ documentsKind: 'forbidden', documents: [], documentsTitle: 'Forbidden' }).text()).toContain('Forbidden')
    expect(workspace({ documentId: 'doc-1', documentKind: 'not_found', document: null, documentTitle: 'Not found' }).text()).toContain('Not found')
    expect(workspace({ documentsKind: 'unavailable', documents: [], documentsTitle: 'Unavailable' }).text()).toContain('Unavailable')
  })
})

describe('shipper document request contract', () => {
  it('scopes the list and reads to the selected company and shipment', async () => {
    const calls: string[] = []
    const client = new PortalClient({
      baseUrl: 'http://gateway.test',
      getAccessToken: () => 'tok',
      getAllowedCompanyIds: () => ['co-shipper'],
      fetchImpl: async (input, init) => {
        const url = new URL(String(input))
        const headers = new Headers(init?.headers)
        calls.push(`${url.pathname}${url.search}`)
        expect(url.pathname.startsWith('/api/v1/shipper/documents')).toBe(true)
        expect(url.pathname.startsWith('/api/v1/documents')).toBe(false)
        expect(url.pathname.startsWith('/v1/')).toBe(false)
        expect(url.searchParams.get('shipper_company_id')).toBe('co-shipper')
        expect(url.searchParams.has('tenant_id')).toBe(false)
        expect(url.searchParams.has('related_entity_type')).toBe(false)
        expect(headers.get('x-company-id')).toBe('co-shipper')
        expect(headers.get('x-tenant-id')).toBeNull()
        return new Response('{}', { status: 200 })
      },
    })
    const list = documentListSpec('shp-1', 'co-shipper', { documentType: 'POD', limit: 80, offset: 0 })
    expect(list.query.shipment_id).toBe('shp-1')
    expect(list.query.limit).toBe('50')
    await client.request(list.path, { companyId: 'co-shipper', query: list.query })
    const detail = documentReadSpec('co-shipper', 'doc-1')
    await client.request(detail.path, { companyId: 'co-shipper', query: detail.query })
    const files = documentReadSpec('co-shipper', 'doc-1', '/attachments')
    await client.request(files.path, { companyId: 'co-shipper', query: files.query })
    const file = documentReadSpec('co-shipper', 'doc-1', '/attachments/att-1')
    await client.request(file.path, { companyId: 'co-shipper', query: file.query })
    expect(calls.some((call) => call.includes('shipment_id=shp-1') && call.includes('document_type=POD'))).toBe(true)
    expect(calls.some((call) => call.includes('/api/v1/shipper/documents/doc-1/attachments'))).toBe(true)
  })

  it('rejects a document that belongs to another shipment before attachments', () => {
    expect(documentMatchesShipment({ related_entity_type: 'SHIPMENT', related_entity_id: 'shp-1' }, 'shp-1')).toBe(true)
    expect(documentMatchesShipment({ related_entity_type: 'SHIPMENT', related_entity_id: 'shp-other' }, 'shp-1')).toBe(false)
    expect(documentMatchesShipment({ related_entity_type: 'ORDER', related_entity_id: 'shp-1' }, 'shp-1')).toBe(false)
    expect(documentMatchesShipment({ related_entity_id: 'shp-1' }, 'shp-1')).toBe(false)
    const page = readFileSync(join(process.cwd(), 'pages/documents/index.vue'), 'utf8')
    const layout = readFileSync(join(process.cwd(), 'layouts/default.vue'), 'utf8')
    expect(page.indexOf('api.getDocument')).toBeLessThan(page.indexOf('api.listDocumentAttachments'))
    expect(page).not.toContain('Promise.all')
    expect(page).toContain('documentMatchesShipment')
    expect(layout).not.toContain('company-switch')
    expect(page).toContain('company-switch')
  })

  it('drops shipment context when the company changes and never sends authority overrides', () => {
    expect(retainShipmentContext('co-a', 'co-b', 'shp-1')).toBeNull()
    expect(retainShipmentContext('co-a', 'co-a', 'shp-1')).toBe('shp-1')
    expect(() => documentListSpec('shp-1', 'co-a', { tenant_id: 'tenant-1' } as never)).toThrow(/authority/)
    expect(() => documentListSpec('shp-1', 'co-a', { related_entity_type: 'SHIPMENT' } as never)).toThrow(/authority/)
    expect(downloadName('attachment; filename="pod.pdf"', 'other.pdf')).toBe('pod.pdf')
    expect(downloadName(null, 'folder/pod.pdf')).toBe('pod.pdf')
    const api = readFileSync(join(process.cwd(), 'composables/useShipperApi.ts'), 'utf8')
    const domain = readFileSync(join(process.cwd(), 'domain/document.ts'), 'utf8')
    expect(domain).toContain('/api/v1/shipper/documents')
    expect(api).toContain('documentListSpec')
    expect(api).toContain('downloadDocumentAttachment')
    expect(api.includes('/api/v1/documents')).toBe(false)
    expect(domain.includes('/api/v1/documents')).toBe(false)
    expect(api.includes("'/v1/shipper/documents")).toBe(false)
    expect(domain.includes("'/v1/shipper/documents")).toBe(false)
    expect(api.includes('tenant_id')).toBe(false)
    expect(api.includes('related_entity_type')).toBe(false)
  })
})
