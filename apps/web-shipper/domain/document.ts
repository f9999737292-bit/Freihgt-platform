/** Customer-safe shipper document and attachment facts. Storage locations are not part of this contract. */

export const DOCUMENT_PAGE_SIZE = 20

export const FORBIDDEN_DOCUMENT_QUERY_KEYS = ['tenant_id', 'related_entity_type'] as const

export interface ShipperDocument {
  id: string
  document_number?: string
  document_type?: string
  document_status?: string
  owner_company_id?: string
  related_entity_type?: string
  related_entity_id?: string | null
  legal_language?: string
  created_at?: string
  updated_at?: string
  version?: number
}

export interface ShipperAttachment {
  id: string
  document_id?: string
  document_version_id?: string | null
  file_name?: string
  media_type?: string
  size_bytes?: number
  sha256?: string
  status?: string
  created_at?: string
  finalized_at?: string | null
}

export interface DocumentPage {
  items?: ShipperDocument[]
  total?: number
  limit?: number
  offset?: number
}

export interface DocumentListFilters {
  documentType?: string
  documentStatus?: string
  limit?: number
  offset?: number
}

export function documentListSpec(
  shipmentId: string,
  companyId: string,
  filters: DocumentListFilters = {},
): { path: string; query: Record<string, string> } {
  rejectForbiddenDocumentQuery(filters)
  const shipment = shipmentId.trim()
  const company = companyId.trim()
  if (!shipment || !company) throw new Error('shipment company scope required')
  const limit = filters.limit && filters.limit > 0 ? Math.min(filters.limit, 50) : DOCUMENT_PAGE_SIZE
  const offset = filters.offset && filters.offset > 0 ? filters.offset : 0
  const query: Record<string, string> = {
    shipper_company_id: company,
    shipment_id: shipment,
    limit: String(limit),
    offset: String(offset),
  }
  const documentType = filters.documentType?.trim()
  const documentStatus = filters.documentStatus?.trim()
  if (documentType) query.document_type = documentType
  if (documentStatus) query.document_status = documentStatus
  rejectForbiddenDocumentQuery(query)
  return { path: '/api/v1/shipper/documents', query }
}

export function documentReadSpec(
  companyId: string,
  documentId: string,
  suffix = '',
): { path: string; query: Record<string, string> } {
  const company = companyId.trim()
  const id = documentId.trim()
  if (!company || !id) throw new Error('document company scope required')
  if (suffix !== '' && !suffix.startsWith('/')) throw new Error('document suffix rejected')
  const path = `/api/v1/shipper/documents/${encodeURIComponent(id)}${suffix}`
  return { path, query: { shipper_company_id: company } }
}

export function retainShipmentContext(
  previousCompanyId: string | null,
  companyId: string,
  shipmentId: string | null,
): string | null {
  if (previousCompanyId && previousCompanyId !== companyId) return null
  const value = shipmentId?.trim() ?? ''
  return value === '' ? null : value
}

export function fileNameFromDisposition(header: string | null | undefined): string | null {
  if (!header) return null
  const encoded = /filename\*=UTF-8''([^;]+)/i.exec(header)
  if (encoded?.[1]) return sanitizeFileName(decodeURIComponent(encoded[1]))
  const quoted = /filename="([^"]+)"/i.exec(header)
  if (quoted?.[1]) return sanitizeFileName(quoted[1])
  const plain = /filename=([^;]+)/i.exec(header)
  return sanitizeFileName(plain?.[1] ?? '')
}

export function sanitizeFileName(name: string | null | undefined): string | null {
  const base = (name ?? '').replace(/\\/g, '/').split('/').pop()?.trim() ?? ''
  if (!base || base === '.' || base === '..') return null
  if (/["\r\n]/.test(base)) return null
  return base
}

export function downloadName(disposition: string | null | undefined, attachmentName: string | null | undefined): string {
  return fileNameFromDisposition(disposition) ?? sanitizeFileName(attachmentName) ?? 'document'
}

function rejectForbiddenDocumentQuery(value: object) {
  for (const key of FORBIDDEN_DOCUMENT_QUERY_KEYS) {
    if (Object.prototype.hasOwnProperty.call(value, key)) {
      throw new Error('document authority query rejected')
    }
  }
}
