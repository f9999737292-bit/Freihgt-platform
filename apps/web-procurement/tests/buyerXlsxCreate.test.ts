import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { canShowBuyerXlsxCreateEntry, hasBuyerXlsxManageRole } from '~/utils/buyerXlsxAccess'
import { isRfxExcelExchangeEnabled } from '~/utils/buyerXlsxFeatureFlag'
import {
  BUYER_XLSX_CREATE_COMMIT_PATH,
  BUYER_XLSX_CREATE_FORBIDDEN_MULTIPART_FIELDS,
  BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES,
  BUYER_XLSX_CREATE_PREVIEW_PATH,
  BUYER_XLSX_CREATE_REQUIRED_MULTIPART_FIELDS,
} from '~/utils/buyerXlsxCreateApiRoutes'
import {
  canCommitBuyerXlsxCreatePreview,
  classifyBuyerXlsxCreateHttpError,
  isBuyerXlsxCreatePreviewEnvelope,
  shouldInvalidateBuyerXlsxCreateAnalysis,
} from '~/utils/buyerXlsxCreateErrors'
import { createBuyerXlsxCreateIdempotencyStore } from '~/utils/buyerXlsxCreateIdempotency'
import {
  buildBuyerXlsxCreatePreviewForm,
  buyerXlsxCreateCommitBody,
  buyerXlsxCreateFormFieldNames,
  buyerXlsxCreateFormHasForbiddenFields,
  buyerXlsxCreateFormHasRequiredFields,
  buyerXlsxCreatePreviewFingerprint,
  validateBuyerXlsxCreateFile,
  validateBuyerXlsxCreateInput,
} from '~/utils/buyerXlsxCreateForm'
import { resolveBuyerXlsxIssueCopy } from '~/utils/buyerXlsxIssueText'
import { ApiError } from '~/utils/apiClient'
import type { BuyerXlsxCreateMetadata, BuyerXlsxCreatePreviewResponse } from '~/types/buyerXlsxCreate'

const metadata = (overrides: Partial<BuyerXlsxCreateMetadata> = {}): BuyerXlsxCreateMetadata => ({
  owner_company_id: '11111111-1111-4111-8111-111111111111',
  rfx_number: 'RFX-F5-1',
  title: 'Create from Excel',
  rfx_type: 'SPOT_RFQ',
  category: 'FREIGHT',
  description: 'Optional note',
  response_deadline: '2026-10-01T18:00',
  currency_code: 'RUB',
  ...overrides,
})

const xlsxFile = (name = 'create.xlsx', size = 128) =>
  new File([new Uint8Array(size)], name, {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  })

const readyPreview = (
  overrides: Partial<BuyerXlsxCreatePreviewResponse> = {},
): BuyerXlsxCreatePreviewResponse => ({
  mode: 'CREATE_NEW_DRAFT',
  schema_name: 'BINTRANS_RFX_BUYER_XLSX_V1',
  schema_version: '1',
  owner_company_id: '11111111-1111-4111-8111-111111111111',
  analysis_id: '33333333-3333-4333-8333-333333333333',
  expires_at: '2026-09-23T00:00:00Z',
  ready_to_commit: true,
  normalized_draft_summary: {
    rfx_number: 'RFX-F5-1',
    title: 'Create from Excel',
    rfx_type: 'SPOT_RFQ',
    category: 'FREIGHT',
    lot_count: 1,
    section_count: 1,
    question_count: 1,
  },
  change_counts: {},
  errors: [],
  warnings: [{
    severity: 'warning',
    machine_code: 'hidden_content_warning',
    message_key: 'rfx.buyer_xlsx.hidden_content_warning',
    sheet: 'Lots',
    row: 2,
  }],
  ...overrides,
})

function flatten(value: Record<string, unknown>, prefix = ''): Record<string, unknown> {
  return Object.entries(value).reduce<Record<string, unknown>>((acc, [key, nested]) => {
    const next = prefix ? `${prefix}.${key}` : key
    if (nested && typeof nested === 'object' && !Array.isArray(nested)) {
      Object.assign(acc, flatten(nested as Record<string, unknown>, next))
    } else {
      acc[next] = nested
    }
    return acc
  }, {})
}

function i18nKeys(locale: string): string[] {
  const raw = JSON.parse(
    readFileSync(resolve(__dirname, `../i18n/${locale}/tenders.json`), 'utf8'),
  ) as { tenders: { createFromExcel: string; buyerXlsxCreate: Record<string, unknown> } }
  return ['createFromExcel', ...Object.keys(flatten(raw.tenders.buyerXlsxCreate))]
}

function readSource(relativePath: string): string {
  return readFileSync(resolve(__dirname, '..', relativePath), 'utf8')
}

describe('buyer XLSX create access and flag matrix', () => {
  it('shows the entry only for BuyerManage roles when the Excel flag is on', () => {
    const allowed = ['PLATFORM_ADMIN', 'PROCUREMENT_MANAGER', 'SHIPPER_ADMIN', 'FORWARDER_MANAGER']
    for (const role of allowed) {
      expect(hasBuyerXlsxManageRole([role])).toBe(true)
      expect(canShowBuyerXlsxCreateEntry({ excelExchangeEnabled: true, roles: [role] })).toBe(true)
    }
    expect(canShowBuyerXlsxCreateEntry({
      excelExchangeEnabled: true,
      roles: ['SHIPPER_LOGIST'],
    })).toBe(false)
    expect(canShowBuyerXlsxCreateEntry({
      excelExchangeEnabled: true,
      roles: ['CARRIER_ADMIN'],
    })).toBe(false)
    expect(canShowBuyerXlsxCreateEntry({
      excelExchangeEnabled: true,
      roles: ['CARRIER_DISPATCHER'],
    })).toBe(false)
    expect(canShowBuyerXlsxCreateEntry({
      excelExchangeEnabled: true,
      roles: ['FINANCE_MANAGER'],
    })).toBe(false)
  })

  it('hides the entry when the Excel feature flag is off', () => {
    expect(isRfxExcelExchangeEnabled(false)).toBe(false)
    expect(canShowBuyerXlsxCreateEntry({
      excelExchangeEnabled: false,
      roles: ['PROCUREMENT_MANAGER'],
    })).toBe(false)
    expect(canShowBuyerXlsxCreateEntry({
      excelExchangeEnabled: true,
      roles: ['PROCUREMENT_MANAGER'],
    })).toBe(true)
  })
})

describe('buyer XLSX create preview multipart', () => {
  it('sends only required metadata plus optional presented fields and never tenant_id', () => {
    const form = buildBuyerXlsxCreatePreviewForm(xlsxFile(), metadata())
    const names = buyerXlsxCreateFormFieldNames(form)
    expect(BUYER_XLSX_CREATE_PREVIEW_PATH).toBe('/api/v1/rfx-events/xlsx-create/preview')
    expect(buyerXlsxCreateFormHasRequiredFields(form)).toBe(true)
    expect(BUYER_XLSX_CREATE_REQUIRED_MULTIPART_FIELDS.every((field) => names.includes(field))).toBe(true)
    expect(names).toContain('description')
    expect(names).toContain('response_deadline')
    expect(names).toContain('currency_code')
    expect(names).not.toContain('tenant_id')
    expect(buyerXlsxCreateFormHasForbiddenFields(form)).toBe(false)
    for (const forbidden of BUYER_XLSX_CREATE_FORBIDDEN_MULTIPART_FIELDS) {
      expect(names).not.toContain(forbidden)
    }
  })

  it('omits empty optional fields', () => {
    const form = buildBuyerXlsxCreatePreviewForm(xlsxFile(), metadata({
      description: '  ',
      response_deadline: '',
      currency_code: '',
    }))
    const names = buyerXlsxCreateFormFieldNames(form)
    expect(names).not.toContain('description')
    expect(names).not.toContain('response_deadline')
    expect(names).not.toContain('currency_code')
    expect(names).not.toContain('tenant_id')
  })

  it('enforces the 5 MiB client cap and .xlsx extension', () => {
    expect(BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES).toBe(5 * 1024 * 1024)
    expect(validateBuyerXlsxCreateFile(null).kind).toBe('file_required')
    expect(validateBuyerXlsxCreateFile(xlsxFile('notes.csv')).kind).toBe('file_extension')
    expect(validateBuyerXlsxCreateFile(
      xlsxFile('huge.xlsx', BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES + 1),
    ).kind).toBe('payload_too_large')
    expect(validateBuyerXlsxCreateInput(xlsxFile(), metadata({ title: '' })).kind).toBe('metadata_required')
    expect(validateBuyerXlsxCreateInput(xlsxFile(), metadata()).ok).toBe(true)
  })
})

describe('buyer XLSX create preview and findings', () => {
  it('accepts a successful CREATE_NEW_DRAFT preview envelope', () => {
    const preview = readyPreview()
    expect(isBuyerXlsxCreatePreviewEnvelope(preview)).toBe(true)
    expect(canCommitBuyerXlsxCreatePreview(preview)).toBe(true)
    expect(preview.ready_to_commit).toBe(true)
    expect(preview.analysis_id).toBeTruthy()
    expect(preview.normalized_draft_summary.lot_count).toBe(1)
  })

  it('renders findings through safe issue copy and rejects UPDATE_DRAFT envelopes', () => {
    const preview = readyPreview({
      ready_to_commit: false,
      analysis_id: undefined,
      errors: [{
        severity: 'error',
        machine_code: 'missing_lot',
        message_key: 'rfx.buyer_xlsx.missing_lot',
        sheet: 'Lots',
        row: 4,
      }],
    })
    expect(canCommitBuyerXlsxCreatePreview(preview)).toBe(false)
    const copy = resolveBuyerXlsxIssueCopy(preview.errors[0])
    expect(copy.i18nKey).toBe('tenders.buyerXlsx.issues.missing_lot')
    expect(copy.location).toBe('Lots:4')
    expect(isBuyerXlsxCreatePreviewEnvelope({
      ...preview,
      mode: 'UPDATE_DRAFT',
      target_event_id: '11111111-1111-4111-8111-111111111111',
    })).toBe(false)
  })

  it('invalidates analysis when metadata or the file changes', () => {
    const first = buyerXlsxCreatePreviewFingerprint(xlsxFile('a.xlsx'), metadata())
    const afterTitle = buyerXlsxCreatePreviewFingerprint(xlsxFile('a.xlsx'), metadata({ title: 'Changed' }))
    const afterFile = buyerXlsxCreatePreviewFingerprint(xlsxFile('b.xlsx'), metadata())
    expect(afterTitle).not.toBe(first)
    expect(afterFile).not.toBe(first)
    expect(canCommitBuyerXlsxCreatePreview(readyPreview(), { analysisInvalidated: true })).toBe(false)
  })
})

describe('buyer XLSX create commit and idempotency', () => {
  it('sends only analysis_id and reuses a stable Idempotency-Key', () => {
    expect(BUYER_XLSX_CREATE_COMMIT_PATH).toBe('/api/v1/rfx-events/xlsx-create/commit')
    expect(buyerXlsxCreateCommitBody('33333333-3333-4333-8333-333333333333')).toEqual({
      analysis_id: '33333333-3333-4333-8333-333333333333',
    })
    const store = createBuyerXlsxCreateIdempotencyStore()
    const first = store.keyForAnalysis('analysis-a')
    expect(first).toBe('buyer-xlsx-create-commit:analysis-a')
    expect(store.keyForAnalysis('analysis-a')).toBe(first)
    expect(store.rememberPreview('analysis-a')).toBe(first)
    expect(store.currentKey('analysis-a')).toBe(first)
  })

  it('keeps analysis and key after a retryable commit failure', () => {
    const store = createBuyerXlsxCreateIdempotencyStore()
    const key = store.keyForAnalysis('analysis-retry')
    const unavailable = classifyBuyerXlsxCreateHttpError(new ApiError(503, {
      code: 'INTERNAL_ERROR',
      message: 'down',
      details: {},
    }))
    const rateLimited = classifyBuyerXlsxCreateHttpError(new ApiError(429, {
      code: 'RATE_LIMITED',
      message: 'slow down',
      details: {},
    }))
    const network = classifyBuyerXlsxCreateHttpError(new Error('failed to fetch'))
    expect(unavailable.retryCommit).toBe(true)
    expect(rateLimited.retryCommit).toBe(true)
    expect(network.retryCommit).toBe(true)
    expect(shouldInvalidateBuyerXlsxCreateAnalysis(unavailable.kind)).toBe(false)
    expect(store.keyForAnalysis('analysis-retry')).toBe(key)
    expect(canCommitBuyerXlsxCreatePreview(readyPreview({ analysis_id: 'analysis-retry' }))).toBe(true)
  })

  it('blocks a second commit after success', () => {
    expect(canCommitBuyerXlsxCreatePreview(readyPreview(), { alreadyCommitted: true })).toBe(false)
  })
})

describe('buyer XLSX create HTTP error mapping', () => {
  it('classifies 400/409/413/422/429/5xx/network and non-JSON 404 without inventing machine codes', () => {
    const validation = classifyBuyerXlsxCreateHttpError(new ApiError(400, {
      code: 'VALIDATION',
      message: 'bad workbook',
      details: {},
    }))
    expect(validation.kind).toBe('validation')
    expect(validation.machineCode).toBeNull()

    const expired = classifyBuyerXlsxCreateHttpError(new ApiError(409, {
      code: 'CONFLICT',
      message: 'expired',
      details: { machine_code: 'analysis_expired' },
    }))
    expect(expired.kind).toBe('analysis_expired')
    expect(shouldInvalidateBuyerXlsxCreateAnalysis(expired.kind)).toBe(true)

    const consumed = classifyBuyerXlsxCreateHttpError(new ApiError(409, {
      code: 'CONFLICT',
      message: 'consumed',
      details: { machine_code: 'analysis_already_consumed' },
    }))
    expect(shouldInvalidateBuyerXlsxCreateAnalysis(consumed.kind)).toBe(true)

    const tooLarge = classifyBuyerXlsxCreateHttpError(new ApiError(413, {
      code: 'REQUEST_BODY_TOO_LARGE',
      message: 'too big',
      details: {},
    }))
    expect(tooLarge.kind).toBe('payload_too_large')

    const previewInvalid = classifyBuyerXlsxCreateHttpError(new ApiError(422, {
      code: 'UNPROCESSABLE',
      message: 'domain',
      details: {},
    }))
    expect(previewInvalid.kind).toBe('preview_invalid')

    const rateLimited = classifyBuyerXlsxCreateHttpError(new ApiError(429, {
      code: 'RATE_LIMITED',
      message: 'slow',
      details: {},
    }))
    expect(rateLimited.kind).toBe('rate_limited')
    expect(rateLimited.messageKey).toBe('tenders.buyerXlsxCreate.errors.rateLimited')

    const unavailable = classifyBuyerXlsxCreateHttpError(new ApiError(500, {
      code: 'INTERNAL_ERROR',
      message: 'boom',
      details: {},
    }))
    expect(unavailable.kind).toBe('unavailable')

    const notFound = classifyBuyerXlsxCreateHttpError(new ApiError(404, {
      code: 'NOT_FOUND',
      message: '404 page not found',
      details: {},
    }))
    expect(notFound.kind).toBe('not_found')
    expect(notFound.machineCode).toBeNull()
    expect(notFound.messageKey).toBe('tenders.buyerXlsxCreate.errors.notFound')
  })
})

describe('buyer XLSX create i18n and source safety', () => {
  it('keeps RU/EN/ZH key parity for the create-from-Excel copy', () => {
    const en = i18nKeys('en-US')
    expect(i18nKeys('ru-RU')).toEqual(en)
    expect(i18nKeys('zh-CN')).toEqual(en)
    expect(en).toContain('createFromExcel')
    expect(en).toContain('title')
    expect(en).toContain('backManual')
    expect(en).toContain('errors.rateLimited')
    expect(en).toContain('errors.notFound')
  })

  it('does not call ERP, publish, submit, or participant APIs from the create flow', () => {
    const panel = readSource('components/rfx/BuyerXlsxCreatePanel.vue')
    const page = readSource('pages/tenders/new-from-xlsx.vue')
    const api = readSource('composables/useBuyerXlsxCreateApi.ts')
    const form = readSource('utils/buyerXlsxCreateForm.ts')
    const routes = readSource('utils/buyerXlsxCreateApiRoutes.ts')
    const sources = [panel, page, api, form].join('\n')
    expect(sources).not.toMatch(/\/integrations\/erp/)
    expect(sources).not.toMatch(/publishRfxEvent|\/publish/)
    expect(sources).not.toMatch(/\/submit|submitQuestionnaire/)
    expect(sources).not.toMatch(/addRfxParticipant|\/participants/)
    expect(panel).not.toMatch(/tenant_id/)
    expect(page).not.toMatch(/tenant_id/)
    expect(api).not.toMatch(/tenant_id/)
    expect(routes).toContain("'tenant_id'")
    expect(form).not.toMatch(/form\.append\(\s*['"]tenant_id['"]/)
    expect(panel).toContain('/tenders/new')
    expect(panel).toContain('/tenders/${result.event_id}')
    expect(readSource('pages/tenders/index.vue')).toContain('/tenders/new-from-xlsx')
    expect(readSource('pages/tenders/index.vue')).toContain('buyer-xlsx-create-entry')
    expect(readSource('pages/tenders/index.vue')).toContain("import PageHeader from '~/components/ui/PageHeader.vue'")
  })
})
