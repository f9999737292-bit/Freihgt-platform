import type { PaginatedResponse } from '~/types/api'
import type {
  AddDocumentFilePayload,
  CancelDocumentPayload,
  CreateDocumentPayload,
  CreateDocumentVersionPayload,
  CreateSigningSessionPayload,
  DocumentDetail,
  DocumentFile,
  DocumentRecord,
  DocumentVersion,
  ListDocumentsFilters,
  SigningSession,
} from '~/types/document'
import { ApiError } from '~/composables/useApi'

export function useDocumentsApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  function tenantQuery(extra: Record<string, string | number | undefined> = {}) {
    return { tenant_id: tenantId(), ...extra }
  }

  async function listDocuments(params: ListDocumentsFilters = {}) {
    const query: Record<string, string | number | undefined> = {
      ...tenantQuery(),
      limit: params.limit ?? 20,
      offset: params.offset ?? 0,
    }
    if (params.document_type) query.document_type = params.document_type
    if (params.document_status) query.document_status = params.document_status
    if (params.related_entity_type) query.related_entity_type = params.related_entity_type
    if (params.related_entity_id) query.related_entity_id = params.related_entity_id
    // TODO: backend search/document_number filter not implemented yet
    if (params.search?.trim()) query.search = params.search.trim()

    const data = await apiGet<PaginatedResponse<DocumentRecord>>('/api/v1/documents', { query })
    return { ...data, items: data.items ?? [] }
  }

  async function getDocument(id: string) {
    return apiGet<DocumentDetail>(`/api/v1/documents/${id}`, { query: tenantQuery() })
  }

  async function createDocument(payload: Omit<CreateDocumentPayload, 'tenant_id'>) {
    return apiPost<DocumentRecord>('/api/v1/documents', {
      ...payload,
      tenant_id: tenantId(),
    })
  }

  async function createDocumentVersion(
    id: string,
    payload: Omit<CreateDocumentVersionPayload, 'tenant_id'>,
  ) {
    return apiPost<DocumentVersion>(`/api/v1/documents/${id}/versions`, {
      ...payload,
      tenant_id: tenantId(),
      payload_xml_path: payload.payload_xml_path?.trim() || null,
      pdf_file_path: payload.pdf_file_path?.trim() || null,
    })
  }

  async function addDocumentFile(id: string, payload: Omit<AddDocumentFilePayload, 'tenant_id'>) {
    return apiPost<DocumentFile>(`/api/v1/documents/${id}/files`, {
      ...payload,
      tenant_id: tenantId(),
      bucket_name: payload.bucket_name?.trim() || undefined,
      checksum_sha256: payload.checksum_sha256?.trim() || undefined,
    })
  }

  async function markReadyForSigning(id: string) {
    return apiPost<DocumentRecord>(`/api/v1/documents/${id}/ready-for-signing`, {
      tenant_id: tenantId(),
    })
  }

  async function createSigningSession(
    id: string,
    payload: Omit<CreateSigningSessionPayload, 'tenant_id'>,
  ) {
    return apiPost<SigningSession>(`/api/v1/documents/${id}/signing-sessions`, {
      ...payload,
      tenant_id: tenantId(),
      expires_at: payload.expires_at || undefined,
    })
  }

  async function cancelDocument(id: string, payload: Omit<CancelDocumentPayload, 'tenant_id'>) {
    return apiPost<DocumentRecord>(`/api/v1/documents/${id}/cancel`, {
      ...payload,
      tenant_id: tenantId(),
    })
  }

  async function archiveDocument(id: string) {
    return apiPost<DocumentRecord>(`/api/v1/documents/${id}/archive`, {
      tenant_id: tenantId(),
    })
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  return {
    listDocuments,
    getDocument,
    createDocument,
    createDocumentVersion,
    addDocumentFile,
    markReadyForSigning,
    createSigningSession,
    cancelDocument,
    archiveDocument,
    isApiUnavailableError,
  }
}
