import type { BuyerXlsxCreateMetadata } from '~/types/buyerXlsxCreate'
import { toRFC3339 } from '~/types/rfx'
import {
  BUYER_XLSX_CREATE_FORBIDDEN_MULTIPART_FIELDS,
  BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES,
  BUYER_XLSX_CREATE_OPTIONAL_MULTIPART_FIELDS,
  BUYER_XLSX_CREATE_REQUIRED_MULTIPART_FIELDS,
} from '~/utils/buyerXlsxCreateApiRoutes'

export type BuyerXlsxCreateClientErrorKind =
  | 'file_required'
  | 'file_extension'
  | 'payload_too_large'
  | 'metadata_required'

export interface BuyerXlsxCreateClientValidation {
  ok: boolean
  kind?: BuyerXlsxCreateClientErrorKind
  messageKey?: string
}

export function isBuyerXlsxCreateFileName(name: string): boolean {
  return name.trim().toLowerCase().endsWith('.xlsx')
}

export function validateBuyerXlsxCreateFile(file: File | null): BuyerXlsxCreateClientValidation {
  if (!file) {
    return {
      ok: false,
      kind: 'file_required',
      messageKey: 'tenders.buyerXlsxCreate.errors.fileRequired',
    }
  }
  if (!isBuyerXlsxCreateFileName(file.name)) {
    return {
      ok: false,
      kind: 'file_extension',
      messageKey: 'tenders.buyerXlsxCreate.errors.fileExtension',
    }
  }
  if (file.size > BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES) {
    return {
      ok: false,
      kind: 'payload_too_large',
      messageKey: 'tenders.buyerXlsxCreate.errors.tooLarge',
    }
  }
  return { ok: true }
}

export function validateBuyerXlsxCreateMetadata(
  metadata: BuyerXlsxCreateMetadata,
): BuyerXlsxCreateClientValidation {
  const required: Array<keyof BuyerXlsxCreateMetadata> = [
    'owner_company_id',
    'rfx_number',
    'title',
    'rfx_type',
    'category',
  ]
  if (required.some((field) => !String(metadata[field] || '').trim())) {
    return {
      ok: false,
      kind: 'metadata_required',
      messageKey: 'tenders.buyerXlsxCreate.errors.metadataRequired',
    }
  }
  return { ok: true }
}

export function validateBuyerXlsxCreateInput(
  file: File | null,
  metadata: BuyerXlsxCreateMetadata,
): BuyerXlsxCreateClientValidation {
  const meta = validateBuyerXlsxCreateMetadata(metadata)
  if (!meta.ok) return meta
  return validateBuyerXlsxCreateFile(file)
}

export function buyerXlsxCreatePreviewFingerprint(
  file: File | null,
  metadata: BuyerXlsxCreateMetadata,
): string {
  return JSON.stringify({
    fileName: file?.name ?? '',
    fileSize: file?.size ?? 0,
    fileLastModified: file?.lastModified ?? 0,
    owner_company_id: metadata.owner_company_id.trim(),
    rfx_number: metadata.rfx_number.trim(),
    title: metadata.title.trim(),
    rfx_type: metadata.rfx_type.trim(),
    category: metadata.category.trim(),
    description: metadata.description.trim(),
    response_deadline: metadata.response_deadline.trim(),
    currency_code: metadata.currency_code.trim(),
  })
}

export function buyerXlsxCreateCommitBody(analysisId: string): { analysis_id: string } {
  return { analysis_id: analysisId }
}

export function buildBuyerXlsxCreatePreviewForm(
  file: File,
  metadata: BuyerXlsxCreateMetadata,
): FormData {
  const form = new FormData()
  form.append('file', file, file.name)
  form.append('owner_company_id', metadata.owner_company_id.trim())
  form.append('rfx_number', metadata.rfx_number.trim())
  form.append('title', metadata.title.trim())
  form.append('rfx_type', metadata.rfx_type.trim())
  form.append('category', metadata.category.trim())
  if (metadata.description.trim()) {
    form.append('description', metadata.description.trim())
  }
  const deadline = toRFC3339(metadata.response_deadline)
  if (deadline) {
    form.append('response_deadline', deadline)
  }
  if (metadata.currency_code.trim()) {
    form.append('currency_code', metadata.currency_code.trim())
  }
  return form
}

export function buyerXlsxCreateFormFieldNames(form: FormData): string[] {
  return Array.from(form.keys())
}

export function buyerXlsxCreateFormHasForbiddenFields(form: FormData): boolean {
  const names = new Set(buyerXlsxCreateFormFieldNames(form))
  return BUYER_XLSX_CREATE_FORBIDDEN_MULTIPART_FIELDS.some((field) => names.has(field))
}

export function buyerXlsxCreateFormHasRequiredFields(form: FormData): boolean {
  const names = new Set(buyerXlsxCreateFormFieldNames(form))
  return BUYER_XLSX_CREATE_REQUIRED_MULTIPART_FIELDS.every((field) => names.has(field))
}

export function buyerXlsxCreateAllowedFieldNames(): readonly string[] {
  return [
    ...BUYER_XLSX_CREATE_REQUIRED_MULTIPART_FIELDS,
    ...BUYER_XLSX_CREATE_OPTIONAL_MULTIPART_FIELDS,
  ]
}
