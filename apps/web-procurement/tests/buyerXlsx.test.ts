import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  buyerXlsxCommitPath,
  buyerXlsxExportPath,
  buyerXlsxPreviewPath,
} from '~/utils/buyerXlsxApiRoutes'
import { canShowBuyerXlsxPanel, hasBuyerXlsxManageRole } from '~/utils/buyerXlsxAccess'
import { isRfxExcelExchangeEnabled } from '~/utils/buyerXlsxFeatureFlag'
import {
  canCommitBuyerXlsxPreview,
  classifyBuyerXlsxHttpError,
  isBuyerXlsxPreviewEnvelope,
  shouldInvalidateBuyerXlsxAnalysis,
} from '~/utils/buyerXlsxErrors'
import { createBuyerXlsxIdempotencyStore } from '~/utils/buyerXlsxIdempotency'
import { buyerXlsxIssueShowsInternal, resolveBuyerXlsxIssueCopy } from '~/utils/buyerXlsxIssueText'
import { ApiError } from '~/utils/apiClient'
import type { BuyerXlsxPreviewResponse } from '~/types/buyerXlsx'

const readyPreview = (overrides: Partial<BuyerXlsxPreviewResponse> = {}): BuyerXlsxPreviewResponse => ({
  schema_name: 'BINTRANS_RFX_BUYER_XLSX_V1',
  schema_version: '1',
  mode: 'UPDATE_DRAFT',
  target_event_id: '11111111-1111-1111-1111-111111111111',
  target_draft_version_id: '22222222-2222-2222-2222-222222222222',
  target_version_number: 1,
  target_event_row_version: 3,
  target_draft_row_version: 4,
  analysis_id: '33333333-3333-3333-3333-333333333333',
  expires_at: '2026-09-21T00:00:00Z',
  ready_to_commit: true,
  summary: { errors: 0, warnings: 1 },
  errors: [],
  warnings: [{ severity: 'warning', machine_code: 'unused_column', message_key: 'rfx.buyer_xlsx.unused_column' }],
  ...overrides,
})

function i18nKeys(locale: string): string[] {
  const raw = JSON.parse(
    readFileSync(resolve(__dirname, `../i18n/${locale}/tenders.json`), 'utf8'),
  ) as { tenders: { buyerXlsx: Record<string, unknown> } }
  return Object.keys(flatten(raw.tenders.buyerXlsx))
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

describe('buyer XLSX F1 routes and access', () => {
  it('uses the accepted human JWT paths', () => {
    const eventId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
    expect(buyerXlsxExportPath(eventId)).toBe(`/api/v1/rfx-events/${eventId}/xlsx-export`)
    expect(buyerXlsxPreviewPath(eventId)).toBe(`/api/v1/rfx-events/${eventId}/xlsx-import/preview`)
    expect(buyerXlsxCommitPath(eventId)).toBe(`/api/v1/rfx-events/${eventId}/xlsx-import/commit`)
  })

  it('shows the panel only for BuyerManage + flag + DRAFT', () => {
    expect(hasBuyerXlsxManageRole(['SHIPPER_LOGIST'])).toBe(false)
    expect(hasBuyerXlsxManageRole(['PROCUREMENT_MANAGER'])).toBe(true)
    expect(canShowBuyerXlsxPanel({
      excelExchangeEnabled: true,
      roles: ['PROCUREMENT_MANAGER'],
      eventStatus: 'DRAFT',
    })).toBe(true)
    expect(canShowBuyerXlsxPanel({
      excelExchangeEnabled: false,
      roles: ['PROCUREMENT_MANAGER'],
      eventStatus: 'DRAFT',
    })).toBe(false)
    expect(canShowBuyerXlsxPanel({
      excelExchangeEnabled: true,
      roles: ['PROCUREMENT_MANAGER'],
      eventStatus: 'PUBLISHED',
    })).toBe(false)
    expect(canShowBuyerXlsxPanel({
      excelExchangeEnabled: true,
      roles: ['SHIPPER_LOGIST'],
      eventStatus: 'DRAFT',
    })).toBe(false)
    expect(isRfxExcelExchangeEnabled(true)).toBe(true)
    expect(isRfxExcelExchangeEnabled('true')).toBe(true)
    expect(isRfxExcelExchangeEnabled('false')).toBe(false)
    expect(isRfxExcelExchangeEnabled(false)).toBe(false)
  })
})

describe('buyer XLSX preview and commit guards', () => {
  it('treats 422 preview envelopes as not commitable', () => {
    const preview = readyPreview({
      ready_to_commit: false,
      analysis_id: undefined,
      errors: [{ severity: 'error', machine_code: 'missing_lot', message_key: 'rfx.buyer_xlsx.missing_lot' }],
    })
    expect(isBuyerXlsxPreviewEnvelope(preview)).toBe(true)
    expect(canCommitBuyerXlsxPreview(preview)).toBe(false)
  })

  it('allows commit only for ready persisted analysis', () => {
    expect(canCommitBuyerXlsxPreview(readyPreview())).toBe(true)
    expect(canCommitBuyerXlsxPreview(readyPreview({ ready_to_commit: true, analysis_id: undefined }))).toBe(false)
    expect(canCommitBuyerXlsxPreview(null)).toBe(false)
  })

  it('classifies 409 machine codes and asks to redo preview after stale', () => {
    const stale = classifyBuyerXlsxHttpError(new ApiError(409, {
      code: 'CONFLICT',
      message: 'stale',
      details: { machine_code: 'stale_target' },
    }))
    expect(stale.kind).toBe('stale_target')
    expect(stale.retryPreview).toBe(true)

    const expired = classifyBuyerXlsxHttpError(new ApiError(409, {
      code: 'CONFLICT',
      message: 'expired',
      details: { machine_code: 'analysis_expired' },
    }))
    expect(expired.kind).toBe('analysis_expired')
    expect(expired.retryPreview).toBe(true)

    const consumed = classifyBuyerXlsxHttpError(new ApiError(409, {
      code: 'CONFLICT',
      message: 'consumed',
      details: { machine_code: 'analysis_already_consumed' },
    }))
    expect(consumed.kind).toBe('analysis_already_consumed')
    expect(shouldInvalidateBuyerXlsxAnalysis(stale.kind)).toBe(true)
    expect(shouldInvalidateBuyerXlsxAnalysis(expired.kind)).toBe(true)
    expect(shouldInvalidateBuyerXlsxAnalysis(consumed.kind)).toBe(true)
    expect(canCommitBuyerXlsxPreview(readyPreview(), { analysisInvalidated: true })).toBe(false)
    expect(canCommitBuyerXlsxPreview(readyPreview(), { alreadyCommitted: true })).toBe(false)

    const idem = classifyBuyerXlsxHttpError(new ApiError(409, {
      code: 'CONFLICT',
      message: 'idem',
      details: { machine_code: 'idempotency_conflict' },
    }))
    expect(idem.kind).toBe('idempotency_conflict')
    expect(idem.retryPreview).toBe(false)
  })

  it('reuses the same idempotency key for one analysis and issues a new key after a new preview', () => {
    const store = createBuyerXlsxIdempotencyStore()
    const first = store.keyForAnalysis('analysis-a')
    expect(store.keyForAnalysis('analysis-a')).toBe(first)
    const second = store.rememberPreview('analysis-b')
    expect(second).not.toBe(first)
    expect(second).toBe(store.keyForAnalysis('analysis-b'))
  })

  it('maps preview issues to localized copy and keeps unknown codes technical-only', () => {
    const known = resolveBuyerXlsxIssueCopy({
      machine_code: 'missing_lot',
      message_key: 'rfx.buyer_xlsx.missing_lot',
      sheet: 'Lots',
      row: 4,
    })
    expect(known.known).toBe(true)
    expect(known.i18nKey).toBe('tenders.buyerXlsx.issues.missing_lot')
    expect(known.technicalCode).toBe('missing_lot')
    expect(known.location).toBe('Lots:4')
    expect(buyerXlsxIssueShowsInternal(known.i18nKey, { message_key: 'rfx.buyer_xlsx.missing_lot' })).toBe(false)

    const unknown = resolveBuyerXlsxIssueCopy({
      machine_code: 'future_code',
      message_key: 'rfx.buyer_xlsx_import.future_internal_payload',
    })
    expect(unknown.known).toBe(false)
    expect(unknown.i18nKey).toBe('tenders.buyerXlsx.issues.unknown')
    expect(unknown.technicalCode).toBe('future_code')
    expect(unknown.i18nKey).not.toContain('future_internal_payload')
  })
})

describe('buyer XLSX i18n', () => {
  it('keeps RU/EN/ZH key parity', () => {
    const en = i18nKeys('en-US')
    expect(i18nKeys('ru-RU')).toEqual(en)
    expect(i18nKeys('zh-CN')).toEqual(en)
    expect(en).toContain('title')
    expect(en).toContain('errors.staleTarget')
    expect(en).toContain('errors.idempotencyConflict')
    expect(en).toContain('issues.unknown')
    expect(en).toContain('issues.missing_lot')
  })
})
