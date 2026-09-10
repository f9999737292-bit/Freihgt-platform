import type {
  RfxChangeImpactAnalysisResponse,
  RfxChangeImpactPreviewRequest,
  RfxCloneEventFromTemplateRequest,
  RfxCloneEventFromTemplateResponse,
  RfxCompareVersionsRequest,
  RfxCompareVersionsResponse,
  RfxPublishQuestionnaireRequest,
  RfxRestoreVersionAsDraftRequest,
  RfxVersionDetailResponse,
  RfxVersionListResponse,
  RfxVersionRecord,
} from '~/types/rfx-version-lifecycle'
import type { RfxPublishReadinessResult } from '~/types/rfx-questionnaire'
import { createIdempotencyKey } from '~/utils/idempotencyKey'
import { rfxEventVersionApiPath } from '~/utils/rfxVersionApiRoutes'

export function useRfxVersionLifecycleApi(eventId: Ref<string> | string) {
  const { apiGet, apiPost } = useApi()

  const eventIdRef = computed(() => (typeof eventId === 'string' ? eventId : eventId.value))

  function basePath(suffix = '') {
    return rfxEventVersionApiPath(eventIdRef.value, suffix)
  }

  async function listVersions() {
    return apiGet<RfxVersionListResponse>(basePath('/versions'))
  }

  async function getVersionDetail(versionId: string) {
    return apiGet<RfxVersionDetailResponse>(basePath(`/versions/${versionId}`))
  }

  async function validatePublish() {
    return apiPost<RfxPublishReadinessResult>(basePath('/validate-publish'))
  }

  async function publishQuestionnaire(
    payload: RfxPublishQuestionnaireRequest,
    idempotencyKey?: string,
  ) {
    return apiPost<RfxVersionRecord>(basePath('/questionnaire/publish'), payload, {
      headers: { 'Idempotency-Key': idempotencyKey ?? createIdempotencyKey('evt-pub') },
    })
  }

  async function forkDraft(idempotencyKey?: string) {
    return apiPost<RfxVersionRecord>(basePath('/versions/fork-draft'), {}, {
      headers: { 'Idempotency-Key': idempotencyKey ?? createIdempotencyKey('evt-fork') },
    })
  }

  async function compareVersions(payload: RfxCompareVersionsRequest) {
    return apiPost<RfxCompareVersionsResponse>(basePath('/versions/compare'), payload)
  }

  async function restoreAsDraft(
    versionId: string,
    payload: RfxRestoreVersionAsDraftRequest,
    idempotencyKey?: string,
  ) {
    return apiPost<RfxVersionRecord>(basePath(`/versions/${versionId}/restore-draft`), payload, {
      headers: { 'Idempotency-Key': idempotencyKey ?? createIdempotencyKey('evt-restore') },
    })
  }

  async function previewChangeImpact(payload: RfxChangeImpactPreviewRequest) {
    return apiPost<RfxChangeImpactAnalysisResponse>(basePath('/change-impact/preview'), payload)
  }

  async function cloneFromTemplate(
    payload: RfxCloneEventFromTemplateRequest,
    idempotencyKey?: string,
  ) {
    return apiPost<RfxCloneEventFromTemplateResponse>('/api/v1/rfx-events/from-template', payload, {
      headers: { 'Idempotency-Key': idempotencyKey ?? createIdempotencyKey('evt-clone') },
    })
  }

  return {
    listVersions,
    getVersionDetail,
    validatePublish,
    publishQuestionnaire,
    forkDraft,
    compareVersions,
    restoreAsDraft,
    previewChangeImpact,
    cloneFromTemplate,
  }
}
