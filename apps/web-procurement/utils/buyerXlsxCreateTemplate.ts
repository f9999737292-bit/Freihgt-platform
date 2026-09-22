import type {
  BuyerXlsxCreateMetadata,
  BuyerXlsxCreatePreviewResponse,
} from '~/types/buyerXlsxCreate'
import { ApiError } from '~/utils/apiClient'
import {
  BUYER_XLSX_CREATE_CONTENT_TYPE,
  BUYER_XLSX_CREATE_TEMPLATE_FILENAME,
  BUYER_XLSX_CREATE_TEMPLATE_PATH,
} from '~/utils/buyerXlsxCreateApiRoutes'

export const BUYER_XLSX_CREATE_TEMPLATE_MIME = BUYER_XLSX_CREATE_CONTENT_TYPE

const SAFE_XLSX_BASENAME = /^[A-Za-z0-9][A-Za-z0-9._-]{0,120}\.xlsx$/

export type BuyerXlsxCreateTemplateErrorKind =
  | 'unauthorized'
  | 'forbidden'
  | 'notFound'
  | 'rateLimited'
  | 'unavailable'
  | 'invalid_binary'

export interface BuyerXlsxCreateTemplatePayload {
  blob: Blob
  filename: string | null
  contentType: string | null
}

export interface BuyerXlsxCreateTemplateFile {
  blob: Blob
  filename: string
  contentType: string
}

export interface BuyerXlsxCreateTemplateFlowSnapshot {
  file: File | null
  metadata: BuyerXlsxCreateMetadata
  preview: BuyerXlsxCreatePreviewResponse | null
  analysisId: string | null
  idempotencyKey: string | null
}

export interface BuyerXlsxCreateTemplateLabels {
  download: string
  downloading: string
  success: string
  hint: string
  unauthorized: string
  forbidden: string
  notFound: string
  rateLimited: string
  unavailable: string
  invalidBinary: string
}

export interface BuyerXlsxCreateTemplateDownloadOutcome {
  phase: 'success' | 'error'
  kind: BuyerXlsxCreateTemplateErrorKind | null
  messageKey: string
  flow: BuyerXlsxCreateTemplateFlowSnapshot
}

const TEMPLATE_ERROR_KEYS: Record<BuyerXlsxCreateTemplateErrorKind, string> = {
  unauthorized: 'tenders.buyerXlsxCreate.template.errors.unauthorized',
  forbidden: 'tenders.buyerXlsxCreate.template.errors.forbidden',
  notFound: 'tenders.buyerXlsxCreate.template.errors.notFound',
  rateLimited: 'tenders.buyerXlsxCreate.template.errors.rateLimited',
  unavailable: 'tenders.buyerXlsxCreate.template.errors.unavailable',
  invalid_binary: 'tenders.buyerXlsxCreate.template.errors.invalidBinary',
}

export const BUYER_XLSX_CREATE_TEMPLATE_SUCCESS_KEY = 'tenders.buyerXlsxCreate.template.status.success'

export class BuyerXlsxCreateTemplateBinaryError extends Error {
  readonly kind = 'invalid_binary' as const

  constructor() {
    super('invalid template binary')
    this.name = 'BuyerXlsxCreateTemplateBinaryError'
  }
}

export class BuyerXlsxCreateTemplateBrowserError extends Error {
  readonly kind = 'browser_unavailable' as const

  constructor() {
    super('browser download unavailable')
    this.name = 'BuyerXlsxCreateTemplateBrowserError'
  }
}

export function buyerXlsxCreateTemplateRequest(): {
  method: 'GET'
  path: typeof BUYER_XLSX_CREATE_TEMPLATE_PATH
  headers: { Accept: typeof BUYER_XLSX_CREATE_TEMPLATE_MIME }
} {
  return {
    method: 'GET',
    path: BUYER_XLSX_CREATE_TEMPLATE_PATH,
    headers: { Accept: BUYER_XLSX_CREATE_TEMPLATE_MIME },
  }
}

export function resolveBuyerXlsxCreateTemplateFilename(candidate: string | null | undefined): string {
  if (typeof candidate !== 'string') return BUYER_XLSX_CREATE_TEMPLATE_FILENAME
  if (/[\u0000-\u001f\u007f]/.test(candidate)) return BUYER_XLSX_CREATE_TEMPLATE_FILENAME
  const value = candidate.trim()
  if (!SAFE_XLSX_BASENAME.test(value) || value.includes('..')) {
    return BUYER_XLSX_CREATE_TEMPLATE_FILENAME
  }
  return value
}

function mimeType(value: string | null | undefined): string {
  if (!value) return ''
  return value.split(';', 1)[0]?.trim().toLowerCase() ?? ''
}

export function classifyBuyerXlsxCreateTemplateError(error: unknown): {
  kind: BuyerXlsxCreateTemplateErrorKind
  messageKey: string
} {
  if (error instanceof BuyerXlsxCreateTemplateBinaryError) {
    return { kind: 'invalid_binary', messageKey: TEMPLATE_ERROR_KEYS.invalid_binary }
  }
  if (error instanceof ApiError) {
    if (error.status === 401) {
      return { kind: 'unauthorized', messageKey: TEMPLATE_ERROR_KEYS.unauthorized }
    }
    if (error.status === 403) {
      return { kind: 'forbidden', messageKey: TEMPLATE_ERROR_KEYS.forbidden }
    }
    if (error.status === 404) {
      return { kind: 'notFound', messageKey: TEMPLATE_ERROR_KEYS.notFound }
    }
    if (error.status === 429) {
      return { kind: 'rateLimited', messageKey: TEMPLATE_ERROR_KEYS.rateLimited }
    }
    return { kind: 'unavailable', messageKey: TEMPLATE_ERROR_KEYS.unavailable }
  }
  if (error instanceof BuyerXlsxCreateTemplateBrowserError) {
    return { kind: 'unavailable', messageKey: TEMPLATE_ERROR_KEYS.unavailable }
  }
  return { kind: 'unavailable', messageKey: TEMPLATE_ERROR_KEYS.unavailable }
}

function isBinaryBlob(value: unknown): value is Blob {
  if (!value || typeof value !== 'object') return false
  const blob = value as Blob
  return typeof blob.size === 'number'
    && typeof blob.slice === 'function'
    && typeof blob.arrayBuffer === 'function'
}

export async function validateBuyerXlsxCreateTemplatePayload(
  input: BuyerXlsxCreateTemplatePayload,
): Promise<BuyerXlsxCreateTemplateFile> {
  if (!isBinaryBlob(input.blob) || input.blob.size < 4) {
    throw new BuyerXlsxCreateTemplateBinaryError()
  }
  const headerMime = mimeType(input.contentType)
  const blobMime = mimeType(input.blob.type)
  const mimeOk = headerMime
    ? headerMime === BUYER_XLSX_CREATE_TEMPLATE_MIME
    : blobMime === BUYER_XLSX_CREATE_TEMPLATE_MIME
  if (!mimeOk) throw new BuyerXlsxCreateTemplateBinaryError()
  const header = new Uint8Array(await input.blob.slice(0, 4).arrayBuffer())
  const zipLocalHeader = header.length >= 4
    && header[0] === 0x50
    && header[1] === 0x4b
    && header[2] === 0x03
    && header[3] === 0x04
  if (!zipLocalHeader) throw new BuyerXlsxCreateTemplateBinaryError()
  return {
    blob: input.blob,
    filename: resolveBuyerXlsxCreateTemplateFilename(input.filename),
    contentType: BUYER_XLSX_CREATE_TEMPLATE_MIME,
  }
}

export async function fetchBuyerXlsxCreateTemplate(
  apiGetBlob: (
    path: string,
    options?: {
      headers?: Record<string, string>
      query?: Record<string, string | number | undefined>
    },
  ) => Promise<BuyerXlsxCreateTemplatePayload>,
): Promise<BuyerXlsxCreateTemplateFile> {
  const request = buyerXlsxCreateTemplateRequest()
  const result = await apiGetBlob(request.path, {
    headers: { Accept: request.headers.Accept },
  })
  return validateBuyerXlsxCreateTemplatePayload(result)
}

export function browserTemplateDownloadAvailable(): boolean {
  return typeof window !== 'undefined'
    && typeof document !== 'undefined'
    && typeof URL !== 'undefined'
    && typeof URL.createObjectURL === 'function'
    && typeof URL.revokeObjectURL === 'function'
    && typeof Blob !== 'undefined'
}

export function triggerBuyerXlsxCreateTemplateDownload(blob: Blob, filename: string): void {
  if (!browserTemplateDownloadAvailable()) {
    throw new BuyerXlsxCreateTemplateBrowserError()
  }
  const safeName = resolveBuyerXlsxCreateTemplateFilename(filename)
  const url = URL.createObjectURL(blob)
  try {
    const link = document.createElement('a')
    link.href = url
    link.download = safeName
    link.rel = 'noopener'
    link.style.display = 'none'
    document.body.appendChild(link)
    try {
      link.click()
    } finally {
      link.remove()
    }
  } finally {
    URL.revokeObjectURL(url)
  }
}

export function buyerXlsxCreateTemplateFlowSnapshot(input: {
  file: File | null
  metadata: BuyerXlsxCreateMetadata
  preview: BuyerXlsxCreatePreviewResponse | null
  idempotencyKey: string | null
}): BuyerXlsxCreateTemplateFlowSnapshot {
  return {
    file: input.file,
    metadata: input.metadata,
    preview: input.preview,
    analysisId: input.preview?.analysis_id ?? null,
    idempotencyKey: input.idempotencyKey,
  }
}

export async function performBuyerXlsxCreateTemplateDownload(input: {
  fetchTemplate: () => Promise<BuyerXlsxCreateTemplatePayload>
  saveFile: (file: BuyerXlsxCreateTemplateFile) => void
  flow: BuyerXlsxCreateTemplateFlowSnapshot
}): Promise<BuyerXlsxCreateTemplateDownloadOutcome> {
  const flow = input.flow
  try {
    const raw = await input.fetchTemplate()
    const file = await validateBuyerXlsxCreateTemplatePayload(raw)
    input.saveFile(file)
    return {
      phase: 'success',
      kind: null,
      messageKey: BUYER_XLSX_CREATE_TEMPLATE_SUCCESS_KEY,
      flow,
    }
  } catch (error) {
    const classified = classifyBuyerXlsxCreateTemplateError(error)
    return {
      phase: 'error',
      kind: classified.kind,
      messageKey: classified.messageKey,
      flow,
    }
  }
}
