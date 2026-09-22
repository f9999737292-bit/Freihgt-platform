/**
 * @vitest-environment jsdom
 */
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import BuyerXlsxCreateTemplateDownload from '~/components/rfx/BuyerXlsxCreateTemplateDownload.vue'
import { ApiError } from '~/utils/apiClient'
import { buyerXlsxCreateTemplateFlowSnapshot } from '~/utils/buyerXlsxCreateTemplate'
import type {
  BuyerXlsxCreateTemplateLabels,
  BuyerXlsxCreateTemplatePayload,
} from '~/utils/buyerXlsxCreateTemplate'
import type { BuyerXlsxCreateMetadata, BuyerXlsxCreatePreviewResponse } from '~/types/buyerXlsxCreate'

const MIME = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
const FILENAME = 'bintrans-rfx-buyer-xlsx-v1-create-template.xlsx'

function labels(): BuyerXlsxCreateTemplateLabels {
  const raw = JSON.parse(readFileSync(resolve(__dirname, '../i18n/en-US/tenders.json'), 'utf8')) as {
    tenders: { buyerXlsxCreate: { template: {
      download: string
      hint: string
      status: { downloading: string; success: string }
      errors: Record<string, string>
    } } }
  }
  const template = raw.tenders.buyerXlsxCreate.template
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

class ReadableBlob {
  readonly size: number
  readonly type: string
  private readonly bytes: Uint8Array

  constructor(parts: Uint8Array[], options?: { type?: string }) {
    const size = parts.reduce((sum, part) => sum + part.byteLength, 0)
    const bytes = new Uint8Array(size)
    let offset = 0
    for (const part of parts) {
      bytes.set(part, offset)
      offset += part.byteLength
    }
    this.bytes = bytes
    this.size = size
    this.type = options?.type ?? ''
  }

  slice(start = 0, end = this.size) {
    return new ReadableBlob([this.bytes.slice(start, end)], { type: this.type })
  }

  async arrayBuffer() {
    return this.bytes.buffer.slice(this.bytes.byteOffset, this.bytes.byteOffset + this.bytes.byteLength)
  }
}

function payload(filename: string | null = FILENAME, bytes = [0x50, 0x4b, 0x03, 0x04, 0x14]): BuyerXlsxCreateTemplatePayload {
  return {
    blob: new ReadableBlob([new Uint8Array(bytes)], { type: MIME }) as unknown as Blob,
    filename,
    contentType: MIME,
  }
}

function metadata(): BuyerXlsxCreateMetadata {
  return {
    owner_company_id: '11111111-1111-4111-8111-111111111111',
    rfx_number: 'RFX-F5-1',
    title: 'Create from Excel',
    rfx_type: 'SPOT_RFQ',
    category: 'FREIGHT',
    description: 'Keep me',
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
    ready_to_commit: true,
    normalized_draft_summary: {
      rfx_number: 'RFX-F5-1',
      title: 'Create from Excel',
      rfx_type: 'SPOT_RFQ',
      category: 'FREIGHT',
      lot_count: 1,
      section_count: 0,
      question_count: 0,
    },
    errors: [],
    warnings: [],
  }
}

function createFlow() {
  const file = new File([new Uint8Array([9])], 'filled.xlsx', { type: MIME })
  const currentMetadata = metadata()
  const currentPreview = preview()
  const flow = buyerXlsxCreateTemplateFlowSnapshot({
    file,
    metadata: currentMetadata,
    preview: currentPreview,
    idempotencyKey: 'buyer-xlsx-create-commit:33333333-3333-4333-8333-333333333333',
  })
  return { flow, file, currentMetadata, currentPreview }
}

const mounted: Array<{ unmount: () => void }> = []

function mountControl(options: {
  roles?: string[]
  enabled?: boolean
  download?: () => Promise<BuyerXlsxCreateTemplatePayload>
  flow?: ReturnType<typeof createFlow>['flow']
} = {}) {
  const created = options.flow ? null : createFlow()
  const flow = options.flow ?? created!.flow
  const download = options.download ?? vi.fn(async () => payload())
  const wrapper = mount(BuyerXlsxCreateTemplateDownload, {
    attachTo: document.body,
    props: {
      excelExchangeEnabled: options.enabled ?? true,
      roles: options.roles ?? ['PROCUREMENT_MANAGER'],
      download,
      labels: labels(),
      flow,
    },
  })
  mounted.push(wrapper)
  return { wrapper, download, flow, file: created?.file ?? flow.file }
}

const blobUrls = {
  created: [] as string[],
  revoked: [] as string[],
}

beforeEach(() => {
  blobUrls.created = []
  blobUrls.revoked = []
  const createObjectURL = (blob: Blob) => {
    if (!blob || blob.size <= 0) {
      throw new Error('createObjectURL received an empty blob')
    }
    const url = `blob:template-${blobUrls.created.length + 1}`
    blobUrls.created.push(url)
    return url
  }
  const revokeObjectURL = (url: string) => {
    blobUrls.revoked.push(String(url))
  }
  const targets = new Set<typeof URL>([URL, window.URL])
  for (const target of targets) {
    Object.defineProperty(target, 'createObjectURL', {
      configurable: true,
      writable: true,
      value: createObjectURL,
    })
    Object.defineProperty(target, 'revokeObjectURL', {
      configurable: true,
      writable: true,
      value: revokeObjectURL,
    })
  }
})

afterEach(() => {
  for (const wrapper of mounted) wrapper.unmount()
  mounted.length = 0
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

function expectFlowUnchanged(flow: ReturnType<typeof createFlow>['flow'], file: File | null) {
  expect(flow.file).toBe(file)
  expect(flow.file?.name).toBe('filled.xlsx')
  expect(flow.metadata.title).toBe('Create from Excel')
  expect(flow.metadata.description).toBe('Keep me')
  expect(flow.preview?.analysis_id).toBe('33333333-3333-4333-8333-333333333333')
  expect(flow.analysisId).toBe('33333333-3333-4333-8333-333333333333')
  expect(flow.idempotencyKey).toBe('buyer-xlsx-create-commit:33333333-3333-4333-8333-333333333333')
}

describe('buyer XLSX create template download control', () => {
  it('shows a native button for BuyerManage roles when the Excel flag is on', () => {
    const allowed = ['PLATFORM_ADMIN', 'PROCUREMENT_MANAGER', 'SHIPPER_ADMIN', 'FORWARDER_MANAGER']
    for (const role of allowed) {
      const { wrapper } = mountControl({ roles: [role], enabled: true })
      const button = wrapper.get('[data-testid="buyer-xlsx-create-template-download"]')
      expect(button.element.tagName).toBe('BUTTON')
      expect(button.attributes('type')).toBe('button')
      expect(button.text()).toBe('Download blank template')
      expect(button.text()).not.toContain('tenders.')
      expect(wrapper.text()).toContain('Create from Excel')
    }
  })

  it('hides the button for non-manage roles and when the flag is off', () => {
    for (const roles of [['SHIPPER_LOGIST'], ['CARRIER_ADMIN'], ['CARRIER_MANAGER'], []]) {
      const { wrapper, download } = mountControl({ roles, enabled: true })
      expect(wrapper.find('[data-testid="buyer-xlsx-create-template-download"]').exists()).toBe(false)
      expect(download).not.toHaveBeenCalled()
    }
    const flagOff = mountControl({ roles: ['PLATFORM_ADMIN'], enabled: false })
    expect(flagOff.wrapper.find('[data-testid="buyer-xlsx-create-template-download"]').exists()).toBe(false)
    expect(flagOff.download).not.toHaveBeenCalled()
  })

  it('downloads once per user action, then revokes the object URL and removes the anchor', async () => {
    let resolveDownload: (value: BuyerXlsxCreateTemplatePayload) => void = () => {}
    const download = vi.fn(() => new Promise<BuyerXlsxCreateTemplatePayload>((resolve) => {
      resolveDownload = resolve
    }))
    const anchors: HTMLAnchorElement[] = []
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function click(this: HTMLAnchorElement) {
      expect(this.isConnected).toBe(true)
      anchors.push(this)
    })
    const { wrapper, flow, file } = mountControl({ download })
    expect(download).not.toHaveBeenCalled()
    const button = wrapper.get('[data-testid="buyer-xlsx-create-template-download"]')
    const buttonElement = button.element as HTMLButtonElement
    buttonElement.focus()
    expect(buttonElement.ownerDocument.activeElement).toBe(buttonElement)
    expect(getComputedStyle(buttonElement).outlineStyle).not.toBe('none')

    await button.trigger('click')
    await button.trigger('click')
    expect(download).toHaveBeenCalledTimes(1)
    expect(download.mock.calls[0]).toEqual([])
    expect(button.attributes('disabled')).toBeDefined()
    expect(button.attributes('aria-busy')).toBe('true')
    expect(button.text()).toBe('Downloading blank template')
    expect(wrapper.get('[data-testid="buyer-xlsx-create-template-download-status"]').text())
      .toBe('Downloading blank template')

    resolveDownload(payload('../secret.xlsx'))
    await flushPromises()

    expect(blobUrls.created).toHaveLength(1)
    expect(blobUrls.revoked).toEqual(blobUrls.created)
    expect(anchors).toHaveLength(1)
    expect(anchors[0]?.download).toBe(FILENAME)
    expect(anchors[0]?.isConnected).toBe(false)
    expect(document.querySelectorAll('a[download]')).toHaveLength(0)
    expect(wrapper.attributes('data-phase')).toBe('success')
    expect(button.attributes('disabled')).toBeUndefined()
    expect(button.attributes('aria-busy')).toBe('false')
    expect(button.text()).toBe('Download blank template')
    expect(wrapper.get('[data-testid="buyer-xlsx-create-template-download-status"]').text())
      .toContain('Blank template downloaded')
    expect(wrapper.find('[data-testid="buyer-xlsx-create-template-download-error"]').exists()).toBe(false)
    expectFlowUnchanged(flow, file)
    click.mockRestore()
  })

  it('shows a localized error and keeps the create flow after a failed download', async () => {
    const cases = [
      [401, 'unauthorized', 'active session'],
      [403, 'forbidden', 'do not have permission'],
      [404, 'notFound', 'unavailable, or this feature is turned off'],
      [429, 'rateLimited', 'Try the download again later'],
      [500, 'unavailable', 'could not be downloaded'],
      [0, 'unavailable', 'could not be downloaded'],
    ] as const
    for (const [status, kind, snippet] of cases) {
      const download = vi.fn(async () => {
        throw new ApiError(status, {
          code: 'BACKEND_CODE',
          message: 'SECRET_BACKEND_BODY',
          details: {},
        })
      })
      const { wrapper, flow, file } = mountControl({ download })
      await wrapper.get('[data-testid="buyer-xlsx-create-template-download"]').trigger('click')
      await flushPromises()
      const error = wrapper.get('[data-testid="buyer-xlsx-create-template-download-error"]')
      expect(error.attributes('role')).toBe('alert')
      expect(error.attributes('data-error-kind')).toBe(kind)
      expect(error.text().toLowerCase()).toContain(snippet.toLowerCase())
      expect(error.text()).not.toContain('SECRET_BACKEND_BODY')
      expect(error.text()).not.toContain('tenders.')
      expect(wrapper.get('[data-testid="buyer-xlsx-create-template-download"]').attributes('disabled')).toBeUndefined()
      expect(download).toHaveBeenCalledTimes(1)
      expectFlowUnchanged(flow, file)
    }
  })

  it('retries with a new download and still keeps the current upload state', async () => {
    const calls: string[] = []
    const download = vi.fn(async () => {
      calls.push('get')
      if (calls.length === 1) {
        return payload(FILENAME, [])
      }
      return payload('blank-template.xlsx')
    })
    const downloadedNames: string[] = []
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function click(this: HTMLAnchorElement) {
      downloadedNames.push(this.download)
    })
    const { wrapper, flow, file } = mountControl({ download })
    const button = wrapper.get('[data-testid="buyer-xlsx-create-template-download"]')
    await button.trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="buyer-xlsx-create-template-download-error"]').attributes('data-error-kind'))
      .toBe('invalid_binary')
    expect(wrapper.get('[data-testid="buyer-xlsx-create-template-download-error"]').text())
      .toContain('empty or invalid')
    expectFlowUnchanged(flow, file)
    expect(calls).toEqual(['get'])

    await button.trigger('click')
    await flushPromises()
    expect(calls).toEqual(['get', 'get'])
    expect(downloadedNames, wrapper.text()).toEqual(['blank-template.xlsx'])
    expect(wrapper.attributes('data-phase'), wrapper.text()).toBe('success')
    expect(wrapper.find('[data-testid="buyer-xlsx-create-template-download-error"]').exists()).toBe(false)
    expectFlowUnchanged(flow, file)
    click.mockRestore()
  })
})
