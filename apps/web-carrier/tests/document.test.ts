import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DocumentWorkspace from '../components/documents/DocumentWorkspace.vue'
import {
  FORBIDDEN_DOCUMENT_QUERY_KEYS,
  companyMayReadDocuments,
  documentListSpec,
  documentMatchesShipment,
  documentReadSpec,
  downloadName,
} from '../domain/document'

const LEAKS = ['X-Storage-Bucket', 'X-Object-Key', 'X-Storage-Key', 'secret-bucket', 'secret-object-key']

describe('carrier document workspace', () => {
  it('shows an empty shipment document list without a shipment id field', () => {
    const view = mount(DocumentWorkspace, {
      props: {
        shipmentId: 'shipment-A',
        documentsKind: 'empty',
        documentsTitle: 'Документов нет',
        documents: [],
        documentKind: 'ready',
        documentTitle: '',
        document: null,
        attachmentsKind: 'ready',
        attachmentsTitle: '',
        attachments: [],
      },
    })
    expect(view.text()).toContain('Документов нет')
    expect(view.find('[data-testid="shipment-id-input"]').exists()).toBe(false)
    expect(view.find('[data-testid="upload-attachment"]').exists()).toBe(false)
    expect(view.find('input[type="file"]').exists()).toBe(false)
  })

  it('renders document detail and a download control without storage metadata', async () => {
    const view = mount(DocumentWorkspace, {
      props: {
        shipmentId: 'shipment-A',
        documentsKind: 'ready',
        documentsTitle: '',
        documents: [{
          id: 'doc-1',
          document_number: 'DOC-1',
          document_type: 'POD',
          document_status: 'SIGNED',
          related_entity_type: 'SHIPMENT',
          related_entity_id: 'shipment-A',
          created_at: '2026-12-01T10:00:00Z',
        }],
        documentKind: 'ready',
        documentTitle: '',
        document: {
          id: 'doc-1',
          document_number: 'DOC-1',
          document_type: 'POD',
          document_status: 'SIGNED',
          related_entity_type: 'SHIPMENT',
          related_entity_id: 'shipment-A',
        },
        attachmentsKind: 'ready',
        attachmentsTitle: '',
        attachments: [{
          id: 'att-1',
          file_name: 'pod.pdf',
          media_type: 'application/pdf',
          size_bytes: 4,
          status: 'FINALIZED',
          finalized_at: '2026-12-01T10:05:00Z',
        }],
      },
    })
    expect(view.get('[data-testid="document-number"]').text()).toBe('DOC-1')
    expect(view.get('[data-testid="related-shipment"]').text()).toBe('shipment-A')
    expect(view.find('[data-testid="download-attachment"]').exists()).toBe(true)
    for (const leak of LEAKS) expect(view.text()).not.toContain(leak)
    await view.get('[data-testid="open-document"]').trigger('click')
    expect(view.emitted('open')?.[0]).toEqual(['doc-1'])
  })

  it('keeps forbidden and unavailable states distinct from an empty list', () => {
    const forbidden = mount(DocumentWorkspace, {
      props: {
        shipmentId: 'shipment-A',
        documentsKind: 'forbidden',
        documentsTitle: 'Доступ запрещён',
        documents: [],
        documentKind: 'ready',
        documentTitle: '',
        document: null,
        attachmentsKind: 'ready',
        attachmentsTitle: '',
        attachments: [],
      },
    })
    expect(forbidden.text()).toContain('Доступ запрещён')
    expect(forbidden.find('[data-testid="documents-empty"]').exists()).toBe(false)
  })
})

describe('carrier document request contract', () => {
  it('scopes the list to the selected carrier company and shipment', () => {
    const spec = documentListSpec('shipment-A', 'co-1', { documentType: 'POD', limit: 80 })
    expect(spec.path).toBe('/api/v1/carrier/documents')
    expect(spec.query.carrier_company_id).toBe('co-1')
    expect(spec.query.shipment_id).toBe('shipment-A')
    expect(spec.query.limit).toBe('50')
    expect(spec.query.document_type).toBe('POD')
    expect(spec.path.includes('/api/v1/documents')).toBe(false)
    const read = documentReadSpec('co-1', 'doc-1', '/attachments/att-1/content')
    expect(read.path).toBe('/api/v1/carrier/documents/doc-1/attachments/att-1/content')
    expect(read.query).toEqual({ carrier_company_id: 'co-1' })
  })

  it('rejects authority query fields and a document from another shipment', () => {
    for (const key of FORBIDDEN_DOCUMENT_QUERY_KEYS) {
      expect(() => documentListSpec('shipment-A', 'co-1', { [key]: 'secret' } as never)).toThrow(/authority/)
    }
    expect(documentMatchesShipment({ related_entity_type: 'SHIPMENT', related_entity_id: 'shipment-A' }, 'shipment-A')).toBe(true)
    expect(documentMatchesShipment({ related_entity_type: 'SHIPMENT', related_entity_id: 'shipment-B' }, 'shipment-A')).toBe(false)
    expect(downloadName('attachment; filename="pod.pdf"', 'other.pdf')).toBe('pod.pdf')
    expect(companyMayReadDocuments(['CARRIER_ADMIN'])).toBe(true)
    expect(companyMayReadDocuments(['CARRIER_DISPATCHER'])).toBe(true)
    expect(companyMayReadDocuments(['DRIVER'])).toBe(false)
    expect(companyMayReadDocuments(['CARRIER_ACCOUNTANT'])).toBe(false)
  })

  it('does not call the legacy document API or render storage metadata', () => {
    const files = [
      'domain/document.ts',
      'components/documents/DocumentWorkspace.vue',
      'pages/shipments/[id]/documents.vue',
      'pages/transport-orders/index.vue',
      'pages/transport-orders/[id].vue',
      'layouts/default.vue',
    ]
    for (const file of files) {
      const source = readFileSync(join(process.cwd(), file), 'utf8')
      expect(source.includes('/api/v1/documents')).toBe(false)
      for (const leak of ['X-Storage-Bucket', 'X-Object-Key', 'X-Storage-Key']) expect(source).not.toContain(leak)
    }
    const page = readFileSync(join(process.cwd(), 'pages/shipments/[id]/documents.vue'), 'utf8')
    expect(page.indexOf('api.getDocument')).toBeLessThan(page.indexOf('api.listDocumentAttachments'))
    expect(page).not.toContain('Promise.all')
    expect(page).toContain('company-switch')
    const layout = readFileSync(join(process.cwd(), 'layouts/default.vue'), 'utf8')
    expect(layout).not.toContain('openDocuments')
    const list = readFileSync(join(process.cwd(), 'pages/transport-orders/index.vue'), 'utf8')
    expect(list).toContain('item.shipment_id')
    expect(list).not.toContain('shipment-id-input')
  })
})
