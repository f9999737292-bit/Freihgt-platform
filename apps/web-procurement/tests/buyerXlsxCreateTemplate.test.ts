import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it, vi } from 'vitest'
import BuyerXlsxCreateTemplateDownload from '~/components/rfx/BuyerXlsxCreateTemplateDownload.vue'
import { ApiError } from '~/utils/apiClient'
import { buyerXlsxExportPath } from '~/utils/buyerXlsxApiRoutes'
import {
  BUYER_XLSX_CREATE_COMMIT_PATH,
  BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES,
  BUYER_XLSX_CREATE_PREVIEW_PATH,
  BUYER_XLSX_CREATE_TEMPLATE_FILENAME,
  BUYER_XLSX_CREATE_TEMPLATE_PATH,
} from '~/utils/buyerXlsxCreateApiRoutes'
import { buyerXlsxCreateCommitBody } from '~/utils/buyerXlsxCreateForm'
import { createBuyerXlsxCreateIdempotencyStore } from '~/utils/buyerXlsxCreateIdempotency'
import {
  BUYER_XLSX_CREATE_TEMPLATE_MIME,
  BuyerXlsxCreateTemplateBinaryError,
  BuyerXlsxCreateTemplateBrowserError,
  browserTemplateDownloadAvailable,
  buyerXlsxCreateTemplateFlowSnapshot,
  buyerXlsxCreateTemplateRequest,
  classifyBuyerXlsxCreateTemplateError,
  fetchBuyerXlsxCreateTemplate,
  performBuyerXlsxCreateTemplateDownload,
  resolveBuyerXlsxCreateTemplateFilename,
  triggerBuyerXlsxCreateTemplateDownload,
  validateBuyerXlsxCreateTemplatePayload,
  type BuyerXlsxCreateTemplateLabels,
  type BuyerXlsxCreateTemplatePayload,
} from '~/utils/buyerXlsxCreateTemplate'
import type { BuyerXlsxCreateMetadata, BuyerXlsxCreatePreviewResponse } from '~/types/buyerXlsxCreate'

const MIME = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
const FILENAME = 'bintrans-rfx-buyer-xlsx-v1-create-template.xlsx'

function readJson(relativePath: string): Record<string, unknown> {
  return JSON.parse(readFileSync(resolve(__dirname, '..', relativePath), 'utf8')) as Record<string, unknown>
}

function templateCopy(locale: string) {
  const raw = readJson(`i18n/${locale}/tenders.json`) as {
    tenders: { createFromExcel: string; buyerXlsxCreate: { template: BuyerXlsxCreateTemplateLabels & {
      status: { downloading: string; success: string }
      errors: Record<string, string>
    } } }
  }
  return raw.tenders
}

function labelsFrom(locale: string): BuyerXlsxCreateTemplateLabels {
  const template = templateCopy(locale).buyerXlsxCreate.template
  return {
    download: template.download,
    downloading: template.status.downloading,
    success: template.status.success,
    hint: template.hint,
    unauthorized: template.errors.unauthorized,
    forbidden: template.errors.forbidden,
    notFound: template.errors.notFound,
    rateLimited: template.errors.rateLimited,
    unavailable: template.errors.unavailable,
    invalidBinary: template.errors.invalidBinary,
  }
}

function xlsxBlob(bytes: number[] = [0x50, 0x4b, 0x03, 0x04, 0x14], type = MIME) {
  return new Blob([new Uint8Array(bytes)], { type })
}

function payload(overrides: Partial<BuyerXlsxCreateTemplatePayload> = {}): BuyerXlsxCreateTemplatePayload {
  const blob = overrides.blob ?? xlsxBlob()
  return {
    blob,
    filename: overrides.filename === undefined ? FILENAME : overrides.filename,
    contentType: overrides.contentType === undefined ? MIME : overrides.contentType,
  }
}

function metadata(): BuyerXlsxCreateMetadata {
  return {
    owner_company_id: '11111111-1111-4111-8111-111111111111',
    rfx_number: 'RFX-F5-1',
    title: 'Create from Excel',
    rfx_type: 'SPOT_RFQ',
    category: 'FREIGHT',
    description: 'Optional note',
    response_deadline: '2026-10-01T18:00',
    currency_code: 'RUB',
  }
}

function preview(): BuyerXlsxCreatePreviewResponse {
  return {
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
    warnings: [],
  }
}

function flowSnapshot() {
  const currentPreview = preview()
  const file = new File([new Uint8Array([1, 2, 3])], 'filled.xlsx', { type: MIME })
  const store = createBuyerXlsxCreateIdempotencyStore()
  const idempotencyKey = store.rememberPreview(currentPreview.analysis_id)
  const currentMetadata = metadata()
  const flow = buyerXlsxCreateTemplateFlowSnapshot({
    file,
    metadata: currentMetadata,
    preview: currentPreview,
    idempotencyKey,
  })
  return { flow, file, currentPreview, currentMetadata, store, idempotencyKey }
}

describe('buyer XLSX create template request', () => {
  it('uses the exact template GET route without a body, query, or tenant id', async () => {
    const request = buyerXlsxCreateTemplateRequest()
    expect(request).toEqual({
      method: 'GET',
      path: '/api/v1/rfx-events/xlsx-create/template',
      headers: { Accept: MIME },
    })
    expect(BUYER_XLSX_CREATE_TEMPLATE_PATH).toBe('/api/v1/rfx-events/xlsx-create/template')
    expect(request).not.toHaveProperty('body')
    expect(request).not.toHaveProperty('query')
    expect(buyerXlsxExportPath('event-1')).toBe('/api/v1/rfx-events/event-1/xlsx-export')
    expect(request.path).not.toBe(buyerXlsxExportPath('event-1'))

    const calls: Array<{ path: string; options?: { headers?: Record<string, string>; query?: unknown } }> = []
    const file = await fetchBuyerXlsxCreateTemplate(async (path, options) => {
      calls.push({ path, options })
      return payload()
    })

    expect(calls).toEqual([{
      path: '/api/v1/rfx-events/xlsx-create/template',
      options: { headers: { Accept: MIME } },
    }])
    expect(Object.keys(calls[0]?.options ?? {})).toEqual(['headers'])
    expect(calls[0]?.options?.headers).not.toHaveProperty('Idempotency-Key')
    expect(calls[0]?.options?.query).toBeUndefined()
    expect(JSON.stringify(calls)).not.toContain('tenant_id')
    expect(file.filename).toBe(FILENAME)
    expect(file.contentType).toBe(MIME)
    expect(file.blob.size).toBeGreaterThan(0)
  })

  it('keeps a safe backend filename and falls back when the header is unsafe', () => {
    expect(resolveBuyerXlsxCreateTemplateFilename(FILENAME)).toBe(FILENAME)
    expect(resolveBuyerXlsxCreateTemplateFilename('blank-template.xlsx')).toBe('blank-template.xlsx')
    expect(resolveBuyerXlsxCreateTemplateFilename(null)).toBe(FILENAME)
    expect(resolveBuyerXlsxCreateTemplateFilename('')).toBe(FILENAME)
    expect(resolveBuyerXlsxCreateTemplateFilename('../secret.xlsx')).toBe(FILENAME)
    expect(resolveBuyerXlsxCreateTemplateFilename('..\\secret.xlsx')).toBe(FILENAME)
    expect(resolveBuyerXlsxCreateTemplateFilename('folder/file.xlsx')).toBe(FILENAME)
    expect(resolveBuyerXlsxCreateTemplateFilename('C:\\temp\\file.xlsx')).toBe(FILENAME)
    expect(resolveBuyerXlsxCreateTemplateFilename('blank.xlsx\r\nX-Evil: 1')).toBe(FILENAME)
    expect(resolveBuyerXlsxCreateTemplateFilename('blank.xlsx\nSet-Cookie: a=b')).toBe(FILENAME)
    expect(resolveBuyerXlsxCreateTemplateFilename('evil..xlsx')).toBe(FILENAME)
    expect(resolveBuyerXlsxCreateTemplateFilename('.xlsx')).toBe(FILENAME)
  })

  it('accepts the spreadsheet MIME and rejects an empty or non-xlsx body', async () => {
    const accepted = await validateBuyerXlsxCreateTemplatePayload(payload({
      contentType: `${MIME}; charset=utf-8`,
    }))
    expect(accepted.contentType).toBe(MIME)
    expect(accepted.filename).toBe(FILENAME)

    await expect(validateBuyerXlsxCreateTemplatePayload(payload({
      blob: new Blob([], { type: MIME }),
    }))).rejects.toBeInstanceOf(BuyerXlsxCreateTemplateBinaryError)

    await expect(validateBuyerXlsxCreateTemplatePayload(payload({
      blob: xlsxBlob([0x3c, 0x68, 0x74, 0x6d]),
      contentType: MIME,
    }))).rejects.toBeInstanceOf(BuyerXlsxCreateTemplateBinaryError)

    await expect(validateBuyerXlsxCreateTemplatePayload(payload({
      contentType: 'text/html',
    }))).rejects.toBeInstanceOf(BuyerXlsxCreateTemplateBinaryError)

    await expect(validateBuyerXlsxCreateTemplatePayload(payload({
      contentType: null,
      blob: xlsxBlob([0x50, 0x4b, 0x03, 0x04], ''),
    }))).rejects.toBeInstanceOf(BuyerXlsxCreateTemplateBinaryError)
  })
})

describe('buyer XLSX create template errors and preserved create flow', () => {
  it('maps 401, 403, 404, 429, 5xx, and network failures without the raw backend body', () => {
    const cases = [
      [401, 'unauthorized', 'tenders.buyerXlsxCreate.template.errors.unauthorized'],
      [403, 'forbidden', 'tenders.buyerXlsxCreate.template.errors.forbidden'],
      [404, 'notFound', 'tenders.buyerXlsxCreate.template.errors.notFound'],
      [429, 'rateLimited', 'tenders.buyerXlsxCreate.template.errors.rateLimited'],
      [500, 'unavailable', 'tenders.buyerXlsxCreate.template.errors.unavailable'],
      [503, 'unavailable', 'tenders.buyerXlsxCreate.template.errors.unavailable'],
      [0, 'unavailable', 'tenders.buyerXlsxCreate.template.errors.unavailable'],
    ] as const
    for (const [status, kind, messageKey] of cases) {
      const error = new ApiError(status, {
        code: 'BACKEND_CODE',
        message: 'SECRET_BACKEND_BODY',
        details: { machine_code: 'DO_NOT_SHOW' },
      })
      const classified = classifyBuyerXlsxCreateTemplateError(error)
      expect(classified.kind).toBe(kind)
      expect(classified.messageKey).toBe(messageKey)
      expect(JSON.stringify(classified)).not.toContain('SECRET_BACKEND_BODY')
      expect(JSON.stringify(classified)).not.toContain('DO_NOT_SHOW')
    }
    const binary = classifyBuyerXlsxCreateTemplateError(new BuyerXlsxCreateTemplateBinaryError())
    expect(binary.kind).toBe('invalid_binary')
    expect(binary.messageKey).toBe('tenders.buyerXlsxCreate.template.errors.invalidBinary')
    expect(binary.messageKey).not.toContain('invalid template binary')
  })

  it('does not clear the upload file, metadata, preview, analysis id, or idempotency key', async () => {
    const { flow, file, currentPreview, currentMetadata, store, idempotencyKey } = flowSnapshot()
    const saved: string[] = []
    const failed = await performBuyerXlsxCreateTemplateDownload({
      fetchTemplate: async () => {
        throw new ApiError(403, { code: 'FORBIDDEN', message: 'SECRET_BACKEND_BODY', details: {} })
      },
      saveFile: () => {
        saved.push('saved')
      },
      flow,
    })
    expect(failed.phase).toBe('error')
    expect(failed.kind).toBe('forbidden')
    expect(saved).toEqual([])
    expect(failed.flow).toBe(flow)
    expect(flow.file).toBe(file)
    expect(flow.metadata).toBe(currentMetadata)
    expect(flow.metadata.title).toBe('Create from Excel')
    expect(flow.preview).toBe(currentPreview)
    expect(flow.analysisId).toBe('33333333-3333-4333-8333-333333333333')
    expect(flow.idempotencyKey).toBe(idempotencyKey)
    expect(store.currentKey(currentPreview.analysis_id)).toBe(idempotencyKey)

    let fetches = 0
    const succeeded = await performBuyerXlsxCreateTemplateDownload({
      fetchTemplate: async () => {
        fetches += 1
        return payload({ filename: '../secret.xlsx' })
      },
      saveFile: (downloaded) => {
        saved.push(downloaded.filename)
      },
      flow,
    })
    expect(fetches).toBe(1)
    expect(succeeded.phase).toBe('success')
    expect(saved).toEqual([FILENAME])
    expect(flow.file).toBe(file)
    expect(flow.preview).toBe(currentPreview)
    expect(flow.analysisId).toBe(currentPreview.analysis_id)
    expect(flow.idempotencyKey).toBe(idempotencyKey)
    expect(store.currentKey(currentPreview.analysis_id)).toBe(idempotencyKey)
    expect(flow.metadata).toEqual(currentMetadata)
  })
})

describe('buyer XLSX create template SSR and copy', () => {
  it('does not touch browser download globals on the server', () => {
    expect(typeof document).toBe('undefined')
    expect(typeof window).toBe('undefined')
    expect(BuyerXlsxCreateTemplateDownload).toBeTruthy()
    expect(browserTemplateDownloadAvailable()).toBe(false)
    const original = URL.createObjectURL
    let calls = 0
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      writable: true,
      value: (blob: Blob) => {
        calls += 1
        return original.call(URL, blob)
      },
    })
    try {
      expect(() => triggerBuyerXlsxCreateTemplateDownload(xlsxBlob(), FILENAME))
        .toThrow(BuyerXlsxCreateTemplateBrowserError)
      expect(calls).toBe(0)
    } finally {
      Object.defineProperty(URL, 'createObjectURL', {
        configurable: true,
        writable: true,
        value: original,
      })
    }
  })

  it('has RU, EN, and ZH copy that is translated and does not promise publish or participants', () => {
    const en = templateCopy('en-US')
    const ru = templateCopy('ru-RU')
    const zh = templateCopy('zh-CN')
    expect(en.createFromExcel).toBe('Create from Excel')
    expect(ru.createFromExcel).toBe('Создать из Excel')
    expect(zh.createFromExcel).toBe('从 Excel 创建')
    expect(en.buyerXlsxCreate.template.download).toBe('Download blank template')
    expect(ru.buyerXlsxCreate.template.download).toBe('Скачать пустой шаблон')
    expect(zh.buyerXlsxCreate.template.download).toBe('下载空白模板')
    expect(ru.buyerXlsxCreate.template.download).not.toBe(en.buyerXlsxCreate.template.download)
    expect(zh.buyerXlsxCreate.template.download).not.toBe(en.buyerXlsxCreate.template.download)

    const bundles = [en, ru, zh]
    for (const bundle of bundles) {
      const values = JSON.stringify(bundle.buyerXlsxCreate.template)
      expect(values).not.toContain('tenders.buyerXlsxCreate')
      expect(values.toLowerCase()).not.toContain('operationid')
    }
    expect(en.buyerXlsxCreate.template.hint).toContain('Create from Excel')
    expect(en.buyerXlsxCreate.template.hint).toContain('new tender')
    expect(en.buyerXlsxCreate.template.hint).toContain('this page')
    expect(en.buyerXlsxCreate.template.hint).toContain('does not create a tender')
    expect(en.buyerXlsxCreate.template.hint).toContain('do not publish')
    expect(en.buyerXlsxCreate.template.hint.toLowerCase()).toContain('participants')
    expect(ru.buyerXlsxCreate.template.hint).toContain('Создать из Excel')
    expect(ru.buyerXlsxCreate.template.hint).toContain('нового тендера')
    expect(ru.buyerXlsxCreate.template.hint).toContain('этой странице')
    expect(ru.buyerXlsxCreate.template.hint).toContain('не создаёт тендер')
    expect(ru.buyerXlsxCreate.template.hint).toContain('не публикуют тендер автоматически')
    expect(ru.buyerXlsxCreate.template.hint).toContain('не добавляют участников')
    expect(zh.buyerXlsxCreate.template.hint).toContain('从 Excel 创建')
    expect(zh.buyerXlsxCreate.template.hint).toContain('新招标')
    expect(zh.buyerXlsxCreate.template.hint).toContain('本页')
    expect(zh.buyerXlsxCreate.template.hint).toContain('不会创建招标')
    expect(zh.buyerXlsxCreate.template.hint).toContain('不会自动发布')
    expect(zh.buyerXlsxCreate.template.hint).toContain('参与者')
    expect(ru.buyerXlsxCreate.template.hint).not.toBe(en.buyerXlsxCreate.template.hint)
    expect(zh.buyerXlsxCreate.template.hint).not.toBe(en.buyerXlsxCreate.template.hint)
  })
})

describe('buyer XLSX create template regression guards', () => {
  it('leaves upload, commit, list entry, and F1 update placement unchanged', () => {
    expect(BUYER_XLSX_CREATE_MAX_UPLOAD_BYTES).toBe(5 * 1024 * 1024)
    expect(BUYER_XLSX_CREATE_PREVIEW_PATH).toBe('/api/v1/rfx-events/xlsx-create/preview')
    expect(BUYER_XLSX_CREATE_COMMIT_PATH).toBe('/api/v1/rfx-events/xlsx-create/commit')
    expect(buyerXlsxCreateCommitBody('analysis-1')).toEqual({ analysis_id: 'analysis-1' })
    expect(BUYER_XLSX_CREATE_TEMPLATE_FILENAME).toBe(FILENAME)
    expect(BUYER_XLSX_CREATE_TEMPLATE_MIME).toBe(MIME)

    const root = resolve(__dirname, '..')
    const read = (path: string) => readFileSync(resolve(root, path), 'utf8')
    const api = read('composables/useBuyerXlsxCreateApi.ts')
    const index = read('pages/tenders/index.vue')
    const updatePage = read('pages/tenders/[id]/index.vue')
    const updatePanel = read('components/rfx/BuyerXlsxExchangePanel.vue')
    expect(api).toContain('return fetchBuyerXlsxCreateTemplate(apiGetBlob)')
    expect(api).not.toContain('tenant_id')
    expect(index).toContain('buyer-xlsx-create-entry')
    expect(index).toContain('/tenders/new-from-xlsx')
    expect(index).not.toContain('buyer-xlsx-create-template-download')
    expect(updatePage).not.toContain('buyer-xlsx-create-template-download')
    expect(updatePanel).not.toContain('buyer-xlsx-create-template-download')
    expect(updatePanel).toContain('buyer-xlsx-export')
  })
})
