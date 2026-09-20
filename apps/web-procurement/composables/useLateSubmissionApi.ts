import type {
  ApproveLateSubmissionRequestBody,
  CreateLateSubmissionRequestBody,
  LateSubmissionRequest,
  LateSubmissionRequestList,
  RejectLateSubmissionRequestBody,
} from '~/types/lateSubmission'

function latePath(eventId: string, suffix = '') {
  return `/api/v1/rfx-events/${encodeURIComponent(eventId)}/late-submission-requests${suffix}`
}

export function useLateSubmissionApi() {
  const { apiGet, apiPost } = useApi()

  function createRequest(
    eventId: string,
    body: CreateLateSubmissionRequestBody,
    idempotencyKey: string,
    carrierCompanyId?: string,
  ) {
    return apiPost<LateSubmissionRequest>(latePath(eventId), body, {
      headers: { 'Idempotency-Key': idempotencyKey },
      query: carrierCompanyId ? { carrier_company_id: carrierCompanyId } : undefined,
    })
  }

  function listOwnRequests(eventId: string, carrierCompanyId?: string) {
    return apiGet<LateSubmissionRequestList>(latePath(eventId, '/mine'), {
      query: carrierCompanyId ? { carrier_company_id: carrierCompanyId } : undefined,
    })
  }

  function listBuyerQueue(eventId: string) {
    return apiGet<LateSubmissionRequestList>(latePath(eventId))
  }

  function approveRequest(
    eventId: string,
    requestId: string,
    body: ApproveLateSubmissionRequestBody,
    idempotencyKey: string,
  ) {
    return apiPost<LateSubmissionRequest>(
      latePath(eventId, `/${encodeURIComponent(requestId)}/approve`),
      body,
      { headers: { 'Idempotency-Key': idempotencyKey } },
    )
  }

  function rejectRequest(
    eventId: string,
    requestId: string,
    body: RejectLateSubmissionRequestBody,
    idempotencyKey: string,
  ) {
    return apiPost<LateSubmissionRequest>(
      latePath(eventId, `/${encodeURIComponent(requestId)}/reject`),
      body,
      { headers: { 'Idempotency-Key': idempotencyKey } },
    )
  }

  return {
    createRequest,
    listOwnRequests,
    listBuyerQueue,
    approveRequest,
    rejectRequest,
  }
}
