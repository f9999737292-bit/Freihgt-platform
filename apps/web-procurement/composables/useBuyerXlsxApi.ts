import type {
  BuyerXlsxCommitRequest,
  BuyerXlsxCommitResponse,
  BuyerXlsxPreviewResponse,
  BuyerXlsxPreviewResult,
} from '~/types/buyerXlsx'
import {
  BUYER_XLSX_CONTENT_TYPE,
  BUYER_XLSX_MAX_UPLOAD_BYTES,
  buyerXlsxCommitPath,
  buyerXlsxExportPath,
  buyerXlsxPreviewPath,
} from '~/utils/buyerXlsxApiRoutes'
import { isBuyerXlsxPreviewEnvelope } from '~/utils/buyerXlsxErrors'
import { ApiError } from '~/utils/apiClient'

export function useBuyerXlsxApi() {
  const { apiGetBlob, apiPost, apiPostForm } = useApi()

  async function exportBuyerDraft(eventId: string) {
    return apiGetBlob(buyerXlsxExportPath(eventId), {
      headers: { Accept: BUYER_XLSX_CONTENT_TYPE },
    })
  }

  async function previewBuyerDraft(eventId: string, file: File): Promise<BuyerXlsxPreviewResult> {
    if (file.size > BUYER_XLSX_MAX_UPLOAD_BYTES) {
      throw new ApiError(413, {
        code: 'REQUEST_BODY_TOO_LARGE',
        message: 'Uploaded workbook exceeds 5 MiB',
        details: {},
      })
    }
    const form = new FormData()
    form.append('file', file, file.name)
    const preview = await apiPostForm<BuyerXlsxPreviewResponse>(buyerXlsxPreviewPath(eventId), form, {
      acceptStatuses: [422],
    })
    if (!isBuyerXlsxPreviewEnvelope(preview)) {
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

  async function commitBuyerDraft(
    eventId: string,
    body: BuyerXlsxCommitRequest,
    idempotencyKey: string,
  ) {
    return apiPost<BuyerXlsxCommitResponse>(buyerXlsxCommitPath(eventId), body, {
      headers: { 'Idempotency-Key': idempotencyKey },
    })
  }

  return {
    exportBuyerDraft,
    previewBuyerDraft,
    commitBuyerDraft,
  }
}
