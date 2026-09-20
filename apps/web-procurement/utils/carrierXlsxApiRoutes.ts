import { ApiError } from '~/utils/apiClient'

export const CARRIER_XLSX_UPDATE_MODE = 'UPDATE_CARRIER_DRAFT' as const

export const CARRIER_XLSX_MAX_UPLOAD_BYTES = 5 * 1024 * 1024

export const CARRIER_XLSX_IDEMPOTENCY_KEY_MAX_LENGTH = 128

export function carrierXlsxExportPath(eventId: string, responseId: string) {
  return `/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-export`
}

export function carrierXlsxPreviewPath(eventId: string, responseId: string) {
  return `/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/preview`
}

export function carrierXlsxCommitPath(eventId: string, responseId: string) {
  return `/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/commit`
}

export const CARRIER_XLSX_CONTENT_TYPE =
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'

export function assertCarrierXlsxUploadSize(size: number): void {
  if (size > CARRIER_XLSX_MAX_UPLOAD_BYTES) {
    throw new ApiError(413, {
      code: 'REQUEST_BODY_TOO_LARGE',
      message: 'Uploaded workbook exceeds 5 MiB',
      details: {},
    })
  }
}

export function carrierXlsxHumanJwtOperations() {
  return {
    export: 'GET /api/v1/rfx-events/{id}/carrier-responses/{response_id}/xlsx-export',
    preview: 'POST /api/v1/rfx-events/{id}/carrier-responses/{response_id}/xlsx-import/preview',
    commit: 'POST /api/v1/rfx-events/{id}/carrier-responses/{response_id}/xlsx-import/commit',
  } as const
}
