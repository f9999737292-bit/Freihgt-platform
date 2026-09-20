import type {
  CarrierXlsxCommitRequest,
  CarrierXlsxCommitResponse,
  CarrierXlsxPreviewResponse,
  CarrierXlsxPreviewResult,
} from '~/types/carrierXlsx'
import {
  assertCarrierXlsxUploadSize,
  carrierXlsxCommitPath,
  carrierXlsxExportPath,
  carrierXlsxPreviewPath,
} from '~/utils/carrierXlsxApiRoutes'
import { isCarrierXlsxPreviewEnvelope } from '~/utils/carrierXlsxErrors'
import { ApiError } from '~/utils/apiClient'

export function useCarrierXlsxApi() {
  const { apiGetBlob, apiPost, apiPostForm } = useApi()

  async function exportCarrierDraft(eventId: string, responseId: string) {
    return apiGetBlob(carrierXlsxExportPath(eventId, responseId), {
      headers: { Accept: '*/*' },
    })
  }

  async function previewCarrierDraft(
    eventId: string,
    responseId: string,
    file: File,
  ): Promise<CarrierXlsxPreviewResult> {
    assertCarrierXlsxUploadSize(file.size)
    const form = new FormData()
    form.append('file', file, file.name)
    const preview = await apiPostForm<CarrierXlsxPreviewResponse>(
      carrierXlsxPreviewPath(eventId, responseId),
      form,
      { acceptStatuses: [422] },
    )
    if (!isCarrierXlsxPreviewEnvelope(preview)) {
      throw new ApiError(500, {
        code: 'INTERNAL_ERROR',
        message: 'Unexpected preview response',
        details: {},
      })
    }
    return {
      status: preview.ready_to_commit ? 200 : 422,
      preview,
    }
  }

  async function commitCarrierDraft(
    eventId: string,
    responseId: string,
    body: CarrierXlsxCommitRequest,
    idempotencyKey: string,
  ) {
    return apiPost<CarrierXlsxCommitResponse>(carrierXlsxCommitPath(eventId, responseId), body, {
      headers: { 'Idempotency-Key': idempotencyKey },
    })
  }

  return {
    exportCarrierDraft,
    previewCarrierDraft,
    commitCarrierDraft,
  }
}
