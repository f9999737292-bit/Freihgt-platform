export const BUYER_XLSX_UPDATE_MODE = 'UPDATE_DRAFT' as const

export const BUYER_XLSX_MAX_UPLOAD_BYTES = 5 * 1024 * 1024

export function buyerXlsxExportPath(eventId: string) {
  return `/api/v1/rfx-events/${eventId}/xlsx-export`
}

export function buyerXlsxPreviewPath(eventId: string) {
  return `/api/v1/rfx-events/${eventId}/xlsx-import/preview`
}

export function buyerXlsxCommitPath(eventId: string) {
  return `/api/v1/rfx-events/${eventId}/xlsx-import/commit`
}

export const BUYER_XLSX_CONTENT_TYPE =
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
