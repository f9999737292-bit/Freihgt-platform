import type { AddSignaturePayload, DocumentRecord, Signature, SigningSession } from '~/types/document'
import { ApiError } from '~/composables/useApi'

export interface AddSignatureResponse {
  signature: Signature
  signing_session: SigningSession
  document: DocumentRecord
}

export function useSigningApi() {
  const tenantStore = useTenantStore()
  const { apiGet, apiPost } = useApi()

  function tenantId() {
    return tenantStore.tenantId
  }

  function tenantQuery(extra: Record<string, string | number | undefined> = {}) {
    return { tenant_id: tenantId(), ...extra }
  }

  async function getSigningSession(id: string) {
    return apiGet<SigningSession>(`/api/v1/signing-sessions/${id}`, { query: tenantQuery() })
  }

  async function addSignature(
    signingSessionId: string,
    payload: Omit<AddSignaturePayload, 'tenant_id'>,
  ) {
    return apiPost<AddSignatureResponse>(`/api/v1/signing-sessions/${signingSessionId}/signatures`, {
      ...payload,
      tenant_id: tenantId(),
      signature_payload_path: payload.signature_payload_path || 'signatures/test-signature.sig',
      certificate_fingerprint: payload.certificate_fingerprint?.trim() || undefined,
    })
  }

  function isApiUnavailableError(error: unknown): boolean {
    if (error instanceof ApiError) {
      return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
    }
    return error instanceof TypeError
  }

  return {
    getSigningSession,
    addSignature,
    isApiUnavailableError,
  }
}
