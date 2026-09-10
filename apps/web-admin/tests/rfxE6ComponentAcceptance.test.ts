/**
 * E6-008 mounted/component acceptance — real UI behavior with mocked API composables.
 */
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { ApiError } from '../composables/useApi'
import RfxEventPublishPanel from '../components/rfx/versioning/RfxEventPublishPanel.vue'
import RfxProvenanceBanner from '../components/rfx/versioning/RfxProvenanceBanner.vue'
import RfxQuestionnaireReadOnlyView from '../components/rfx/versioning/RfxQuestionnaireReadOnlyView.vue'
import { extractEventProvenance } from '../types/rfx'
import {
  filterCloneableTemplateVersions,
  selectDefaultCloneVersionId,
} from '../utils/rfxTemplateLifecycle'
import { shouldRequireImpactPreview } from '../utils/rfxEventPublishOrchestration'

const flushPendingPatches = vi.fn(async () => undefined)
const validatePublish = vi.fn(async () => ({ ready: true, blocking_fail_count: 0, warning_count: 0, items: [] }))
const listVersions = vi.fn(async () => ({ versions: [] as Array<{ id: string; status: string; version_number: number }> }))
const previewChangeImpact = vi.fn()
const publishQuestionnaire = vi.fn()
const pushToast = vi.fn()

function defaultStudioState() {
  return {
    event: {
      id: 'evt-1',
      rfx_number: 'RFQ-1',
      rfx_type: 'SPOT_RFQ',
      category: 'FREIGHT',
      title: 'Test',
      owner_company_id: 'co-1',
      status: 'DRAFT',
      version: 1,
    },
    draft_version: {
      id: 'draft-1',
      rfx_event_id: 'evt-1',
      version_number: 2,
      status: 'DRAFT',
      questionnaire_enabled: true,
      version: 2,
    },
    sections: [],
    rules: [],
  }
}

const getStudio = vi.fn(async () => defaultStudioState())

vi.mock('~/composables/useRfxQuestionnaireApi', () => ({
  useInjectedRfxQuestionnaireApi: () => ({
    flushPendingPatches,
    validatePublish,
    getStudio,
  }),
  RFX_QUESTIONNAIRE_API_KEY: Symbol('rfxQuestionnaireApi'),
}))

vi.mock('~/composables/useRfxVersionLifecycleApi', () => ({
  useRfxVersionLifecycleApi: () => ({
    listVersions,
    previewChangeImpact,
    publishQuestionnaire,
  }),
}))

vi.stubGlobal('useI18n', () => ({
  t: (key: string) => key,
  locale: ref('en-US'),
}))
vi.stubGlobal('useToast', () => ({ pushToast }))

const publishedVersion = { id: 'pub-1', status: 'PUBLISHED', version_number: 1 }
const supersededVersion = { id: 'sup-1', status: 'SUPERSEDED', version_number: 0 }

function mountPublishPanel() {
  return mount(RfxEventPublishPanel, {
    props: {
      eventId: 'evt-1',
      expectedEventVersion: 1,
      expectedDraftVersion: 2,
      draftVersionId: 'draft-1',
    },
    global: {
      mocks: { $t: (key: string) => key },
      stubs: {
        UiCard: { template: '<div><slot /></div>' },
        RfxStudioRfxPublishReadinessPanel: {
          props: ['result'],
          template: '<div data-testid="readiness">{{ result?.ready }}</div>',
        },
        RfxChangeImpactPanel: {
          props: ['analysis', 'loading'],
          emits: ['confirm'],
          template: '<button data-testid="impact-confirm" @click="$emit(\'confirm\')">confirm</button>',
        },
      },
    },
  })
}

describe('E6 component acceptance', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    flushPendingPatches.mockResolvedValue(undefined)
    validatePublish.mockResolvedValue({ ready: true, blocking_fail_count: 0, warning_count: 0, items: [] })
    listVersions.mockResolvedValue({ versions: [] })
    previewChangeImpact.mockResolvedValue({
      impact_analysis_id: 'ia-1',
      canonical_diff_hash: 'hash-1',
      expires_at: '2099-01-01T00:00:00Z',
      impact_classes: ['NON_MATERIAL'],
      candidate_version_id: 'draft-1',
    })
    publishQuestionnaire.mockResolvedValue({ id: 'pub-res' })
    getStudio.mockImplementation(async () => defaultStudioState())
  })

  it('E6-CMP-01 cloneable versions prefer PUBLISHED over SUPERSEDED', () => {
    const detail = {
      template: { id: 't1', tenant_id: 'tn', template_code: 'T', name_i18n: {}, status: 'ACTIVE', version: 1, created_by: 'u', created_at: '', updated_at: '' },
      draft_version: null,
      published_version: publishedVersion,
      versions: [supersededVersion, publishedVersion],
    }
    const cloneable = filterCloneableTemplateVersions(detail as never)
    expect(selectDefaultCloneVersionId(cloneable)).toBe('pub-1')
    expect(cloneable.some((v) => v.status === 'PUBLISHED')).toBe(true)
  })

  it('E6-CMP-02 DRAFT excluded from clone sources', () => {
    const detail = {
      template: { id: 't1', tenant_id: 'tn', template_code: 'T', name_i18n: {}, status: 'ACTIVE', version: 1, created_by: 'u', created_at: '', updated_at: '' },
      draft_version: { id: 'd1', status: 'DRAFT' },
      published_version: null,
      versions: [{ id: 'd1', status: 'DRAFT', version_number: 2 }],
    }
    expect(filterCloneableTemplateVersions(detail as never)).toHaveLength(0)
  })

  it('E6-CMP-03 provenance banner renders clone provenance fields', () => {
    const wrapper = mount(RfxProvenanceBanner, {
      props: {
        provenance: {
          source_template_id: 'tpl-1',
          source_template_version_id: 'ver-1',
          source_version_number: 2,
          source_version_status: 'PUBLISHED',
        },
        templateId: 'tpl-1',
        templateName: 'Lane RFQ Template',
      },
      global: {
        mocks: { $t: (key: string) => key },
        stubs: { NuxtLink: { template: '<a><slot /></a>' } },
      },
    })
    expect(wrapper.text()).toContain('Lane RFQ Template')
    expect(wrapper.text()).toContain('ver-1')
  })

  it('E6-CMP-04 event GET provenance survives extract after reload simulation', () => {
    const event = {
      id: 'e1',
      tenant_id: 't',
      rfx_number: 'RFX-1',
      rfx_type: 'RFQ',
      category: 'FREIGHT',
      title: 'Test',
      owner_company_id: 'c1',
      status: 'DRAFT',
      source_template_id: 'tpl-1',
      source_template_version_id: 'ver-1',
      source_version_number: 1,
      source_version_status: 'PUBLISHED' as const,
      source_template_name_i18n: { 'en-US': 'Template A' },
    }
    const first = extractEventProvenance(event)
    const reloaded = extractEventProvenance({ ...event, title: 'Test reloaded' })
    expect(reloaded?.source_template_version_id).toBe(first?.source_template_version_id)
    expect(reloaded?.source_template_name_i18n?.['en-US']).toBe('Template A')
  })

  it('E6-CMP-05 manual event without provenance yields null', () => {
    expect(extractEventProvenance({
      id: 'e1', tenant_id: 't', rfx_number: 'RFX-1', rfx_type: 'RFQ', category: 'FREIGHT',
      title: 'Manual', owner_company_id: 'c1', status: 'DRAFT',
    })).toBeNull()
  })

  it('E6-CMP-06 read-only questionnaire view shows sections', () => {
    const wrapper = mount(RfxQuestionnaireReadOnlyView, {
      global: { mocks: { $t: (key: string) => key } },
      props: {
        questionnaire: {
          event_id: 'e1',
          rfx_version_id: 'v1',
          version_number: 1,
          questionnaire_enabled: true,
          version_status: 'PUBLISHED',
          sections: [{
            section: { id: 's1', section_code: 'SEC_1', name_i18n: { 'en-US': 'General' }, sort_order: 1 },
            questions: [{
              id: 'q1', question_code: 'Q_1', name_i18n: { 'en-US': 'Rate' }, question_type: 'TEXT', sort_order: 1,
            }],
          }],
          rules: [],
        },
        loading: false,
        error: null,
      },
    })
    expect(wrapper.text()).toContain('General')
    expect(wrapper.text()).toContain('Rate')
  })

  it('E6-CMP-07 publish panel flushes autosave before validate/publish', async () => {
    listVersions.mockResolvedValue({ versions: [] })
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary text')
    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('rfx.studio.publish'))
    await publishBtn!.trigger('click')
    await flushPromises()
    expect(flushPendingPatches.mock.invocationCallOrder[0]).toBeLessThan(validatePublish.mock.invocationCallOrder[0]!)
    expect(publishQuestionnaire).toHaveBeenCalled()
  })

  it('E6-CMP-08 autosave failure blocks publish', async () => {
    flushPendingPatches.mockRejectedValueOnce(new Error('autosave failed'))
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('rfx.studio.publish'))
    await publishBtn!.trigger('click')
    await flushPromises()
    expect(validatePublish).not.toHaveBeenCalled()
    expect(publishQuestionnaire).not.toHaveBeenCalled()
    expect(pushToast).toHaveBeenCalledWith('error', 'rfx.studio.autosaveError')
  })

  it('E6-CMP-09 NOT_READY from server blocks publish API', async () => {
    validatePublish.mockResolvedValueOnce({ ready: false, blocking_fail_count: 1, warning_count: 0, items: [{ severity: 'FAIL', code: 'X', message: 'blocked' }] })
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('rfx.studio.publish'))
    await publishBtn!.trigger('click')
    await flushPromises()
    expect(publishQuestionnaire).not.toHaveBeenCalled()
  })

  it('E6-CMP-10 first publish does not call impact preview', async () => {
    listVersions.mockResolvedValue({ versions: [] })
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('rfx.studio.publish'))
    await publishBtn!.trigger('click')
    await flushPromises()
    expect(previewChangeImpact).not.toHaveBeenCalled()
    expect(shouldRequireImpactPreview([])).toBe(false)
  })

  it('E6-CMP-11 republish requires impact preview before publish', async () => {
    listVersions.mockResolvedValue({ versions: [publishedVersion] })
    previewChangeImpact.mockResolvedValueOnce({
      impact_analysis_id: 'ia-1',
      canonical_diff_hash: 'hash-1',
      expires_at: '2099-01-01T00:00:00Z',
      impact_classes: ['MATERIAL_NO_RESPONSES'],
      candidate_version_id: 'draft-1',
    })
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('rfx.changeImpact.republish'))
    await publishBtn!.trigger('click')
    await flushPromises()
    expect(previewChangeImpact).toHaveBeenCalledWith({ candidate_version_id: 'draft-1' })
    expect(publishQuestionnaire).not.toHaveBeenCalled()
  })

  it('E6-CMP-12 expired analysis 422 clears path for new preview', async () => {
    listVersions.mockResolvedValue({ versions: [publishedVersion] })
    previewChangeImpact.mockResolvedValueOnce({
      impact_analysis_id: 'ia-old',
      canonical_diff_hash: 'hash-old',
      expires_at: '2020-01-01T00:00:00Z',
      impact_classes: ['MATERIAL_NO_RESPONSES'],
      candidate_version_id: 'draft-1',
    })
    publishQuestionnaire.mockRejectedValueOnce(new ApiError(422, { code: 'IMPACT_ANALYSIS_EXPIRED', message: 'expired', details: {} }))
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    const previewBtn = wrapper.findAll('button').find((b) => b.text().includes('rfx.changeImpact.preview'))
    await previewBtn!.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="impact-confirm"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('rfx.errors.impactAnalysisExpired')
  })

  it('E6-CMP-13 stale diff 409 requires new preview message', async () => {
    listVersions.mockResolvedValue({ versions: [publishedVersion] })
    previewChangeImpact.mockResolvedValueOnce({
      impact_analysis_id: 'ia-1',
      canonical_diff_hash: 'hash-1',
      expires_at: '2099-01-01T00:00:00Z',
      impact_classes: ['MATERIAL_NO_RESPONSES'],
      candidate_version_id: 'draft-1',
    })
    publishQuestionnaire.mockRejectedValueOnce(new ApiError(409, { code: 'STALE_DIFF', message: 'stale', details: {} }))
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    await wrapper.findAll('button').find((b) => b.text().includes('rfx.changeImpact.preview'))!.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="impact-confirm"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('rfx.changeImpact.repreviewRequired')
  })

  it('E6-CMP-14 version history load failure blocks publish path', async () => {
    listVersions.mockRejectedValueOnce(new Error('network'))
    const wrapper = mountPublishPanel()
    await flushPromises()
    expect(wrapper.text()).toContain('rfx.versions.loadFailed')
    await wrapper.find('textarea').setValue('Summary')
    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('rfx.studio.publish'))
    expect(publishBtn!.attributes('disabled')).toBeDefined()
  })

  it('E6-CMP-15 double submit prevented on publish', async () => {
    let resolvePublish: (v: unknown) => void = () => undefined
    publishQuestionnaire.mockImplementation(() => new Promise((r) => { resolvePublish = r }))
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('rfx.studio.publish'))
    void publishBtn!.trigger('click')
    await flushPromises()
    await publishBtn!.trigger('click')
    expect(publishQuestionnaire.mock.calls.length).toBeLessThanOrEqual(1)
    resolvePublish({ id: 'done' })
  })

  it('E6-CMP-16 carrier middleware fails closed via permission checks', async () => {
    const { readFileSync } = await import('node:fs')
    const { resolve } = await import('node:path')
    const middleware = readFileSync(resolve(import.meta.dirname, '../middleware/rfx-buyer-manage.ts'), 'utf8')
    expect(middleware).toMatch(/isCarrierWithoutBuyerAccess\(\)/)
    expect(middleware).toMatch(/canManageRfxTemplates\(\)/)
    expect(middleware).toMatch(/navigateTo\('\/rfx'\)/)
  })

  it('E6-CMP-17 409 optimistic concurrency surfaced to user', async () => {
    publishQuestionnaire.mockRejectedValueOnce(new ApiError(409, { code: 'VERSION_CONFLICT', message: 'conflict', details: {} }))
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    await wrapper.findAll('button').find((b) => b.text().includes('rfx.studio.publish'))!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('rfx.errors.versionConflict')
  })

  it('E6-CMP-18 template version questionnaire route parity', async () => {
    const { RFX_TEMPLATE_OPENAPI_PARITY } = await import('../utils/rfxTemplateApiRoutes')
    expect(RFX_TEMPLATE_OPENAPI_PARITY.some((r) => r.caller === 'getTemplateVersionQuestionnaire')).toBe(true)
  })

  it('E6-CMP-19 consumed analysis 409 does not auto-retry publish', async () => {
    listVersions.mockResolvedValue({ versions: [publishedVersion] })
    previewChangeImpact.mockResolvedValue({
      impact_analysis_id: 'ia-1',
      canonical_diff_hash: 'hash-1',
      expires_at: '2099-01-01T00:00:00Z',
      impact_classes: ['MATERIAL_NO_RESPONSES'],
      candidate_version_id: 'draft-1',
    })
    publishQuestionnaire.mockRejectedValue(new ApiError(409, { code: 'IMPACT_ANALYSIS_CONSUMED', message: 'consumed', details: {} }))
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    await wrapper.findAll('button').find((b) => b.text().includes('rfx.changeImpact.preview'))!.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="impact-confirm"]').trigger('click')
    await flushPromises()
    expect(publishQuestionnaire).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('rfx.errors.impactAnalysisConsumed')
  })

  it('E6-CMP-21 impact confirm re-runs flush, studio reload, and validate before publish', async () => {
    listVersions.mockResolvedValue({ versions: [publishedVersion] })
    previewChangeImpact.mockResolvedValueOnce({
      impact_analysis_id: 'ia-1',
      canonical_diff_hash: 'hash-1',
      expires_at: '2099-01-01T00:00:00Z',
      impact_classes: ['MATERIAL_NO_RESPONSES'],
      candidate_version_id: 'draft-1',
    })
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    await wrapper.findAll('button').find((b) => b.text().includes('rfx.changeImpact.preview'))!.trigger('click')
    await flushPromises()
    const flushCallsBeforeConfirm = flushPendingPatches.mock.calls.length
    const studioCallsBeforeConfirm = getStudio.mock.calls.length
    validatePublish.mockClear()
    publishQuestionnaire.mockClear()
    await wrapper.find('[data-testid="impact-confirm"]').trigger('click')
    await flushPromises()
    expect(flushPendingPatches.mock.calls.length).toBeGreaterThan(flushCallsBeforeConfirm)
    expect(getStudio.mock.calls.length).toBeGreaterThan(studioCallsBeforeConfirm)
    expect(validatePublish).toHaveBeenCalled()
    expect(flushPendingPatches.mock.invocationCallOrder[flushCallsBeforeConfirm]).toBeLessThan(getStudio.mock.invocationCallOrder[studioCallsBeforeConfirm]!)
    expect(getStudio.mock.invocationCallOrder[studioCallsBeforeConfirm]).toBeLessThan(validatePublish.mock.invocationCallOrder[0]!)
    expect(validatePublish.mock.invocationCallOrder[0]).toBeLessThan(publishQuestionnaire.mock.invocationCallOrder[0]!)
    expect(publishQuestionnaire).toHaveBeenCalledWith(
      expect.objectContaining({ expected_event_version: 1, expected_draft_version: 2 }),
      expect.any(String),
    )
  })

  it('E6-CMP-23 stale draft after impact preview blocks confirm publish', async () => {
    listVersions.mockResolvedValue({ versions: [publishedVersion] })
    previewChangeImpact.mockResolvedValueOnce({
      impact_analysis_id: 'ia-1',
      canonical_diff_hash: 'hash-1',
      expires_at: '2099-01-01T00:00:00Z',
      impact_classes: ['MATERIAL_NO_RESPONSES'],
      candidate_version_id: 'draft-1',
    })
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    await wrapper.findAll('button').find((b) => b.text().includes('rfx.changeImpact.preview'))!.trigger('click')
    await flushPromises()
    getStudio.mockResolvedValueOnce({
      ...defaultStudioState(),
      draft_version: {
        id: 'draft-1',
        rfx_event_id: 'evt-1',
        version_number: 2,
        status: 'DRAFT',
        questionnaire_enabled: true,
        version: 3,
      },
    })
    await wrapper.find('[data-testid="impact-confirm"]').trigger('click')
    await flushPromises()
    expect(publishQuestionnaire).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('rfx.changeImpact.repreviewRequired')
  })

  it('E6-CMP-22 impact confirm blocks publish when NOT_READY after confirm', async () => {
    listVersions.mockResolvedValue({ versions: [publishedVersion] })
    previewChangeImpact.mockResolvedValueOnce({
      impact_analysis_id: 'ia-1',
      canonical_diff_hash: 'hash-1',
      expires_at: '2099-01-01T00:00:00Z',
      impact_classes: ['MATERIAL_NO_RESPONSES'],
      candidate_version_id: 'draft-1',
    })
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    await wrapper.findAll('button').find((b) => b.text().includes('rfx.changeImpact.preview'))!.trigger('click')
    await flushPromises()
    validatePublish.mockResolvedValueOnce({ ready: false, blocking_fail_count: 1, warning_count: 0, items: [{ severity: 'FAIL', code: 'X', message: 'blocked' }] })
    await wrapper.find('[data-testid="impact-confirm"]').trigger('click')
    await flushPromises()
    expect(publishQuestionnaire).not.toHaveBeenCalled()
    expect(pushToast).toHaveBeenCalledWith('error', 'rfx.studio.readyFail')
  })

  it('E6-CMP-20 publish sends Idempotency-Key header value', async () => {
    listVersions.mockResolvedValue({ versions: [] })
    const wrapper = mountPublishPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Summary')
    const publishBtn = wrapper.findAll('button').find((b) => b.text().includes('rfx.studio.publish'))
    await publishBtn!.trigger('click')
    await flushPromises()
    const idempotencyKey = publishQuestionnaire.mock.calls[0]?.[1]
    expect(typeof idempotencyKey).toBe('string')
    expect(idempotencyKey.length).toBeGreaterThan(0)
    expect(idempotencyKey.length).toBeLessThanOrEqual(128)
  })
})
