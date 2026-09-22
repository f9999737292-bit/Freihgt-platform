import type {
  BuyerXlsxCreateCommitResponse,
  BuyerXlsxCreateMetadata,
  BuyerXlsxCreatePreviewResponse,
  BuyerXlsxCreatePreviewResult,
} from '~/types/buyerXlsxCreate'
import { ApiError } from '~/utils/apiClient'
import {
  BUYER_XLSX_CREATE_COMMIT_PATH,
  BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES,
  BUYER_XLSX_CREATE_PREVIEW_PATH,
} from '~/utils/buyerXlsxCreateApiRoutes'
import { isBuyerXlsxCreatePreviewEnvelope } from '~/utils/buyerXlsxCreateErrors'
import {
  buildBuyerXlsxCreatePreviewForm,
  buyerXlsxCreateCommitBody,
  validateBuyerXlsxCreateInput,
} from '~/utils/buyerXlsxCreateForm'
import { fetchBuyerXlsxCreateTemplate } from '~/utils/buyerXlsxCreateTemplate'

export function useBuyerXlsxCreateApi() {
  const { apiGetBlob, apiPost, apiPostForm } = useApi()

  async function previewBuyerCreate(
    file: File,
    metadata: BuyerXlsxCreateMetadata,
  ): Promise<BuyerXlsxCreatePreviewResult> {
    const validation = validateBuyerXlsxCreateInput(file, metadata)
    if (!validation.ok) {
      throw new ApiError(validation.kind === 'payload_too_large' ? 413 : 400, {
        code: validation.kind === 'payload_too_large' ? 'REQUEST_BODY_TOO_LARGE' : 'VALIDATION',
        message: validation.messageKey || 'Invalid create-from-xlsx input',
        details: {},
      })
    }
    if (file.size > BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES) {
      throw new ApiError(413, {
        code: 'REQUEST_BODY_TOO_LARGE',
        message: 'Uploaded workbook exceeds 5 MiB',
        details: {},
      })
    }
    const form = buildBuyerXlsxCreatePreviewForm(file, metadata)
    let preview: BuyerXlsxCreatePreviewResponse
    try {
      preview = await apiPostForm<BuyerXlsxCreatePreviewResponse>(
        BUYER_XLSX_CREATE_PREVIEW_PATH,
        form,
        { acceptStatuses: [422] },
      )
    } catch (error) {
      if (error instanceof ApiError) throw error
      throw new ApiError(500, {
        code: 'INTERNAL_ERROR',
        message: 'Unexpected preview response',
        details: {},
      })
    }
    if (!isBuyerXlsxCreatePreviewEnvelope(preview)) {
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

  async function commitBuyerCreate(analysisId: string, idempotencyKey: string) {
    return apiPost<BuyerXlsxCreateCommitResponse>(
      BUYER_XLSX_CREATE_COMMIT_PATH,
      buyerXlsxCreateCommitBody(analysisId),
      {
        headers: { 'Idempotency-Key': idempotencyKey },
      },
    )
  }

  async function downloadBuyerCreateTemplate() {
    return fetchBuyerXlsxCreateTemplate(apiGetBlob)
  }

  return {
    previewBuyerCreate,
    commitBuyerCreate,
    downloadBuyerCreateTemplate,
  }
}
