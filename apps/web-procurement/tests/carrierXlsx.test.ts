import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  assertCarrierXlsxUploadSize,
  CARRIER_XLSX_IDEMPOTENCY_KEY_MAX_LENGTH,
  CARRIER_XLSX_MAX_UPLOAD_BYTES,
  carrierXlsxCommitPath,
  carrierXlsxExportPath,
  carrierXlsxHumanJwtOperations,
  carrierXlsxPreviewPath,
} from '~/utils/carrierXlsxApiRoutes'
import {
  canImportCarrierXlsx,
  canShowCarrierXlsxPanel,
  hasCarrierXlsxReadRole,
  hasCarrierXlsxRespondRole,
} from '~/utils/carrierXlsxAccess'
import { isRfxExcelExchangeEnabled } from '~/utils/buyerXlsxFeatureFlag'
import {
  canCommitCarrierXlsxPreview,
  classifyCarrierXlsxHttpError,
  isCarrierXlsxPreviewEnvelope,
  shouldInvalidateCarrierXlsxAnalysis,
} from '~/utils/carrierXlsxErrors'
import { CARRIER_XLSX_COMMIT_KEY_PREFIX, createCarrierXlsxIdempotencyStore } from '~/utils/carrierXlsxIdempotency'
import { carrierXlsxIssueShowsInternal, resolveCarrierXlsxIssueCopy } from '~/utils/carrierXlsxIssueText'
import { ApiError } from '~/utils/apiClient'
import type { CarrierXlsxPreviewResponse } from '~/types/carrierXlsx'

const eventId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
const responseId = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'

const readyPreview = (overrides: Partial<CarrierXlsxPreviewResponse> = {}): CarrierXlsxPreviewResponse => ({
  schema_name: 'BINTRANS_RFX_CARRIER_XLSX_V1',
  schema_version: '1',
  mode: 'UPDATE_CARRIER_DRAFT',
  target_event_id: eventId,
  target_response_id: responseId,
  target_rfx_version_id: '22222222-2222-2222-2222-222222222222',
  target_version_number: 1,
  target_event_row_version: 3,
  target_response_save_version: 4,
  analysis_id: '33333333-3333-3333-3333-333333333333',
  expires_at: '2026-09-21T00:00:00Z',
  ready_to_commit: true,
  summary: { errors: 0, warnings: 1 },
  errors: [],
  warnings: [{ severity: 'warning', machine_code: 'hidden_row_warning', message_key: 'rfx.carrier_xlsx_import.hidden_row_warning' }],
  ...overrides,
})

function i18nKeys(locale: string): string[] {
  const raw = JSON.parse(
    readFileSync(resolve(__dirname, `../i18n/${locale}/carrierTenders.json`), 'utf8'),
  ) as { carrierTenders: { xlsx: Record<string, unknown> } }
  return Object.keys(flatten(raw.carrierTenders.xlsx))
}

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

describe('carrier XLSX F2 routes and access', () => {
  it('uses the accepted human JWT paths and never includes submit', () => {
    expect(carrierXlsxExportPath(eventId, responseId)).toBe(
      `/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-export`,
    )
    expect(carrierXlsxPreviewPath(eventId, responseId)).toBe(
      `/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/preview`,
    )
    expect(carrierXlsxCommitPath(eventId, responseId)).toBe(
      `/api/v1/rfx-events/${eventId}/carrier-responses/${responseId}/xlsx-import/commit`,
    )
    const ops = Object.values(carrierXlsxHumanJwtOperations())
    expect(ops.every((path) => !path.includes('/submit'))).toBe(true)
    expect(carrierXlsxCommitPath(eventId, responseId)).not.toContain('/submit')
  })

  it('loads the tender page event through the carrier invited-event path', () => {
    const api = readFileSync(resolve(__dirname, '../composables/useCarrierRfxApi.ts'), 'utf8')
    expect(api).toContain('/api/v1/carrier/rfx-events/')
    expect(api).not.toMatch(/function getTender[\s\S]{0,200}\/api\/v1\/rfx-events\//)
  })

  it('shows the panel only for carrier roles + flag + own response id', () => {
    expect(hasCarrierXlsxReadRole(['PROCUREMENT_MANAGER'])).toBe(false)
    expect(hasCarrierXlsxReadRole(['CARRIER_ADMIN'])).toBe(true)
    expect(hasCarrierXlsxRespondRole(['CARRIER_DISPATCHER'])).toBe(true)
    expect(hasCarrierXlsxRespondRole(['SHIPPER_ADMIN'])).toBe(false)

    expect(canShowCarrierXlsxPanel({
      excelExchangeEnabled: true,
      roles: ['CARRIER_DISPATCHER'],
      responseId,
    })).toBe(true)
    expect(canShowCarrierXlsxPanel({
      excelExchangeEnabled: true,
      roles: ['CARRIER_ADMIN'],
      responseId,
    })).toBe(true)
    expect(canShowCarrierXlsxPanel({
      excelExchangeEnabled: false,
      roles: ['CARRIER_DISPATCHER'],
      responseId,
    })).toBe(false)
    expect(canShowCarrierXlsxPanel({
      excelExchangeEnabled: true,
      roles: ['PROCUREMENT_MANAGER'],
      responseId,
    })).toBe(false)
    expect(canShowCarrierXlsxPanel({
      excelExchangeEnabled: true,
      roles: ['CARRIER_DISPATCHER'],
      responseId: '',
    })).toBe(false)
    expect(canImportCarrierXlsx({ responseStatus: 'DRAFT' })).toBe(true)
    expect(canImportCarrierXlsx({ responseStatus: 'SUBMITTED' })).toBe(false)
    expect(canImportCarrierXlsx({ responseStatus: 'DRAFT', responseNotEditable: true })).toBe(false)
    expect(isRfxExcelExchangeEnabled(true)).toBe(true)
    expect(isRfxExcelExchangeEnabled('true')).toBe(true)
    expect(isRfxExcelExchangeEnabled('1')).toBe(true)
    expect(isRfxExcelExchangeEnabled('false')).toBe(false)
    expect(isRfxExcelExchangeEnabled(false)).toBe(false)
  })

  it('rejects uploads larger than 5 MiB before preview', () => {
    expect(CARRIER_XLSX_MAX_UPLOAD_BYTES).toBe(5 * 1024 * 1024)
    expect(() => assertCarrierXlsxUploadSize(CARRIER_XLSX_MAX_UPLOAD_BYTES)).not.toThrow()
    try {
      assertCarrierXlsxUploadSize(CARRIER_XLSX_MAX_UPLOAD_BYTES + 1)
      throw new Error('expected 413')
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError)
      expect((error as ApiError).status).toBe(413)
    }
  })
})

describe('carrier XLSX preview and commit guards', () => {
  it('treats 422 preview envelopes as not commitable', () => {
    const preview = readyPreview({
      ready_to_commit: false,
      analysis_id: undefined,
      errors: [{ severity: 'error', machine_code: 'unknown_lot', message_key: 'rfx.carrier_xlsx_import.unknown_lot' }],
    })
    expect(isCarrierXlsxPreviewEnvelope(preview)).toBe(true)
    expect(canCommitCarrierXlsxPreview(preview)).toBe(false)
    expect(canCommitCarrierXlsxPreview(preview, { analysisInvalidated: true })).toBe(false)
  })

  it('allows commit only for ready persisted analysis', () => {
    expect(canCommitCarrierXlsxPreview(readyPreview())).toBe(true)
    expect(canCommitCarrierXlsxPreview(readyPreview({ ready_to_commit: true, analysis_id: undefined }))).toBe(false)
    expect(canCommitCarrierXlsxPreview(null)).toBe(false)
    expect(canCommitCarrierXlsxPreview(readyPreview(), { alreadyCommitted: true })).toBe(false)
    expect(canCommitCarrierXlsxPreview(readyPreview(), { importDisabled: true })).toBe(false)
  })

  it('classifies 409 machine codes and asks to redo preview after stale', () => {
    const stale = classifyCarrierXlsxHttpError(new ApiError(409, {
      code: 'CONFLICT',
      message: 'stale',
      details: { machine_code: 'stale_target' },
    }))
    expect(stale.kind).toBe('stale_target')
    expect(stale.retryPreview).toBe(true)

    const expired = classifyCarrierXlsxHttpError(new ApiError(409, {
      code: 'CONFLICT',
      message: 'expired',
      details: { machine_code: 'analysis_expired' },
    }))
    expect(expired.kind).toBe('analysis_expired')
    expect(expired.retryPreview).toBe(true)

    const consumed = classifyCarrierXlsxHttpError(new ApiError(409, {
      code: 'CONFLICT',
      message: 'consumed',
      details: { machine_code: 'analysis_already_consumed' },
    }))
    expect(consumed.kind).toBe('analysis_already_consumed')
    expect(shouldInvalidateCarrierXlsxAnalysis(stale.kind)).toBe(true)
    expect(shouldInvalidateCarrierXlsxAnalysis(expired.kind)).toBe(true)
    expect(shouldInvalidateCarrierXlsxAnalysis(consumed.kind)).toBe(true)
    expect(canCommitCarrierXlsxPreview(readyPreview(), { analysisInvalidated: true })).toBe(false)

    const idem = classifyCarrierXlsxHttpError(new ApiError(409, {
      code: 'CONFLICT',
      message: 'idem',
      details: { machine_code: 'idempotency_conflict' },
    }))
    expect(idem.kind).toBe('idempotency_conflict')
    expect(idem.retryPreview).toBe(false)

    const locked = classifyCarrierXlsxHttpError(new ApiError(409, {
      code: 'CONFLICT',
      message: 'locked',
      details: { machine_code: 'response_not_editable' },
    }))
    expect(locked.kind).toBe('response_not_editable')
    expect(locked.disableImport).toBe(true)
    expect(canImportCarrierXlsx({ responseStatus: 'DRAFT', responseNotEditable: locked.disableImport })).toBe(false)
  })

  it('classifies 400, 403, 404, 413 and network failures without leaking payload text', () => {
    expect(classifyCarrierXlsxHttpError(new ApiError(400, { code: 'VALIDATION_ERROR', message: '{"secret":1}', details: {} })).kind).toBe('validation')
    expect(classifyCarrierXlsxHttpError(new ApiError(403, { code: 'FORBIDDEN', message: 'no', details: {} })).kind).toBe('forbidden')
    expect(classifyCarrierXlsxHttpError(new ApiError(404, { code: 'NOT_FOUND', message: 'missing', details: {} })).kind).toBe('not_found')
    expect(classifyCarrierXlsxHttpError(new ApiError(413, { code: 'REQUEST_BODY_TOO_LARGE', message: 'big', details: {} })).kind).toBe('payload_too_large')
    expect(classifyCarrierXlsxHttpError(new Error('network down')).kind).toBe('unavailable')
  })

  it('reuses the same idempotency key for one analysis and issues a new key after a new preview', () => {
    const store = createCarrierXlsxIdempotencyStore()
    const first = store.keyForAnalysis('analysis-a')
    expect(first).toBe(`${CARRIER_XLSX_COMMIT_KEY_PREFIX}analysis-a`)
    expect(first.length).toBeLessThanOrEqual(CARRIER_XLSX_IDEMPOTENCY_KEY_MAX_LENGTH)
    expect(store.keyForAnalysis('analysis-a')).toBe(first)
    const second = store.rememberPreview('analysis-b')
    expect(second).not.toBe(first)
    expect(second).toBe(store.keyForAnalysis('analysis-b'))
  })

  it('maps preview issues to localized copy and keeps unknown codes technical-only', () => {
    const known = resolveCarrierXlsxIssueCopy({
      machine_code: 'unknown_lot',
      message_key: 'rfx.carrier_xlsx_import.unknown_lot',
      sheet: 'OfferLines',
      row: 4,
    })
    expect(known.known).toBe(true)
    expect(known.i18nKey).toBe('carrierTenders.xlsx.issues.unknown_lot')
    expect(known.technicalCode).toBe('unknown_lot')
    expect(known.location).toBe('OfferLines:4')
    expect(carrierXlsxIssueShowsInternal(known.i18nKey, { message_key: 'rfx.carrier_xlsx_import.unknown_lot' })).toBe(false)

    const unknown = resolveCarrierXlsxIssueCopy({
      machine_code: 'future_code',
      message_key: 'rfx.carrier_xlsx_import.future_internal_payload',
    })
    expect(unknown.known).toBe(false)
    expect(unknown.i18nKey).toBe('carrierTenders.xlsx.issues.unknown')
    expect(unknown.technicalCode).toBe('future_code')
    expect(unknown.i18nKey).not.toContain('future_internal_payload')
  })
})

describe('carrier XLSX i18n', () => {
  it('keeps RU/EN/ZH key parity', () => {
    const en = i18nKeys('en-US')
    expect(i18nKeys('ru-RU')).toEqual(en)
    expect(i18nKeys('zh-CN')).toEqual(en)
    expect(en).toContain('title')
    expect(en).toContain('errors.staleTarget')
    expect(en).toContain('errors.idempotencyConflict')
    expect(en).toContain('errors.responseNotEditable')
    expect(en).toContain('issues.unknown')
    expect(en).toContain('issues.unknown_lot')
    expect(en).toContain('issues.competitor_column_denied')
  })
})
