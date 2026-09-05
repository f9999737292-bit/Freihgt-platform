import type {
  AutosaveStatus,
  RfxCreateOptionRequest,
  RfxCreateQuestionRequest,
  RfxCreateRuleRequest,
  RfxCreateSectionRequest,
  RfxPublishReadinessResult,
  RfxQuestion,
  RfxQuestionOption,
  RfxQuestionRule,
  RfxQuestionnaireDefinition,
  RfxReorderQuestionsRequest,
  RfxReorderSectionsRequest,
  RfxSaveDraftRequest,
  RfxSection,
  RfxStudioResponse,
  RfxUpdateOptionRequest,
  RfxUpdateQuestionRequest,
  RfxUpdateRuleRequest,
  RfxUpdateSectionRequest,
  RfxVersionRecord,
  RfxVersionedMutationRequest,
} from '~/types/rfx-questionnaire'
import { ApiError } from '~/composables/useApi'
import { buildApiRequestHeaders } from '~/utils/buildApiRequestHeaders'
import { rfxEventApiPath } from '~/utils/rfxQuestionnaireApiRoutes'
import {
  autosaveStatusFromHttpError,
  canShowSaved,
  markDirtyTransition,
  markSavedTransition,
} from '~/utils/rfxStudioQuestionnaire'

export const RFX_QUESTIONNAIRE_API_KEY = Symbol('rfxQuestionnaireApi')

const PATCH_DEBOUNCE_MS = 800

export function useRfxQuestionnaireApi(rfxEventId: Ref<string> | string) {
  const tenantStore = useTenantStore()
  const authStore = useAuthStore()
  const { apiGet, apiPost, apiPatch } = useApi()

  const studio = ref<RfxStudioResponse | null>(null)
  const questionnaire = ref<RfxQuestionnaireDefinition | null>(null)
  const publishReadiness = ref<RfxPublishReadinessResult | null>(null)
  const autosaveStatus = ref<AutosaveStatus>('idle')
  const lastSavedAt = ref<string | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref<string | null>(null)
  const fieldError = ref<string | null>(null)

  const patchTimers = new Map<string, ReturnType<typeof setTimeout>>()
  let inFlightPatch: Promise<void> | null = null
  let pendingPatchKey: string | null = null

  const eventId = computed(() => (typeof rfxEventId === 'string' ? rfxEventId : rfxEventId.value))

  const draftVersion = computed(() => studio.value?.draft_version?.version ?? null)

  function basePath(suffix = '') {
    return rfxEventApiPath(eventId.value, suffix)
  }

  function canShowSavedStatus(): boolean {
    return canShowSaved(autosaveStatus.value)
  }

  function markDirty() {
    const previous = autosaveStatus.value
    autosaveStatus.value = markDirtyTransition(previous)
    if (previous === 'invalid' && autosaveStatus.value === 'dirty') {
      fieldError.value = null
    }
  }

  function markSavedFromTimestamp(iso?: string | null) {
    const next = markSavedTransition(autosaveStatus.value, iso ?? lastSavedAt.value)
    if (next.lastSavedAt) lastSavedAt.value = next.lastSavedAt
    autosaveStatus.value = next.status
  }

  function handleMutationError(err: unknown) {
    if (err instanceof ApiError) {
      autosaveStatus.value = autosaveStatusFromHttpError(err.status)
      if (err.status === 409) {
        error.value = 'CONFLICT'
        void loadStudio()
        return
      }
      if (err.status === 400) {
        fieldError.value = err.message || 'VALIDATION_ERROR'
        return
      }
    } else {
      autosaveStatus.value = 'save_failed'
    }
    error.value = err instanceof Error ? err.message : 'SAVE_FAILED'
  }

  async function apiDeleteWithBody<T = void>(path: string, body: RfxVersionedMutationRequest) {
    const config = useRuntimeConfig()
    const base = config.public.apiBaseUrl.replace(/\/$/, '')
    const url = new URL(`${base}${path}`)
    const nuxtApp = useNuxtApp()
    const i18n = nuxtApp.$i18n as { locale?: { value?: string } } | undefined
    const response = await fetch(url.toString(), {
      method: 'DELETE',
      headers: buildApiRequestHeaders({
        token: authStore.token,
        tenantId: tenantStore.tenantId,
        companyId: tenantStore.currentCompanyId,
        locale: i18n?.locale?.value ?? 'ru-RU',
      }),
      body: JSON.stringify(body),
    })
    if (!response.ok) {
      let bodyJson: { error: { code: string; message: string; details?: Record<string, unknown> } } | null = null
      try {
        bodyJson = await response.json()
      } catch {
        throw new ApiError(response.status, {
          code: 'INTERNAL_ERROR',
          message: response.statusText || 'Request failed',
          details: {},
        })
      }
      throw new ApiError(response.status, {
        code: bodyJson!.error.code,
        message: bodyJson!.error.message,
        details: bodyJson!.error.details ?? {},
      })
    }
    if (response.status === 204) return undefined as T
    const text = await response.text()
    return text ? (JSON.parse(text) as T) : (undefined as T)
  }

  function clearPatchTimer(key: string) {
    const timer = patchTimers.get(key)
    if (timer) {
      clearTimeout(timer)
      patchTimers.delete(key)
    }
  }

  function clearAllPatchTimers() {
    for (const key of patchTimers.keys()) clearPatchTimer(key)
  }

  async function runDebouncedPatch(key: string, fn: () => Promise<void>) {
    if (inFlightPatch) {
      pendingPatchKey = key
      await inFlightPatch.catch(() => undefined)
    }
    autosaveStatus.value = 'saving'
    fieldError.value = null
    inFlightPatch = (async () => {
      try {
        await fn()
        if (canShowSaved(autosaveStatus.value)) {
          markSavedFromTimestamp(lastSavedAt.value)
        }
      } catch (err) {
        handleMutationError(err)
        throw err
      } finally {
        inFlightPatch = null
        if (pendingPatchKey && pendingPatchKey !== key) {
          const nextKey = pendingPatchKey
          pendingPatchKey = null
          const timer = patchTimers.get(nextKey)
          if (timer) {
            clearPatchTimer(nextKey)
            patchTimers.set(nextKey, setTimeout(() => patchTimers.delete(nextKey), 0))
          }
        } else {
          pendingPatchKey = null
        }
      }
    })()
    await inFlightPatch
  }

  function schedulePatch(key: string, fn: () => Promise<void>) {
    markDirty()
    clearPatchTimer(key)
    patchTimers.set(
      key,
      setTimeout(() => {
        patchTimers.delete(key)
        void runDebouncedPatch(key, fn).catch(() => undefined)
      }, PATCH_DEBOUNCE_MS),
    )
  }

  function getSectionVersion(sectionId: string): number | null {
    const swq = studio.value?.sections.find((item) => item.section.id === sectionId)
    return swq?.section.version ?? null
  }

  function getQuestionVersion(questionId: string): number | null {
    for (const swq of studio.value?.sections ?? []) {
      const question = swq.questions.find((item) => item.id === questionId)
      if (question) return question.version
    }
    return null
  }

  function getOptionVersion(questionId: string, optionId: string): number | null {
    for (const swq of studio.value?.sections ?? []) {
      const question = swq.questions.find((item) => item.id === questionId)
      const option = question?.options?.find((item) => item.id === optionId)
      if (option) return option.version
    }
    return null
  }

  function getRuleVersion(ruleId: string): number | null {
    return studio.value?.rules.find((item) => item.id === ruleId)?.version ?? null
  }

  function applyStudioResponse(data: RfxStudioResponse) {
    studio.value = data
    if (data.draft_version?.updated_at) {
      lastSavedAt.value = data.draft_version.updated_at
    }
  }

  function mergeQuestionInStudio(question: RfxQuestion) {
    if (!studio.value) return
    for (const swq of studio.value.sections) {
      const idx = swq.questions.findIndex((q) => q.id === question.id)
      if (idx >= 0) {
        swq.questions[idx] = question
        return
      }
    }
  }

  function mergeSectionInStudio(section: RfxSection) {
    if (!studio.value) return
    const idx = studio.value.sections.findIndex((s) => s.section.id === section.id)
    if (idx >= 0) studio.value.sections[idx].section = section
  }

  async function getStudio() {
    return apiGet<RfxStudioResponse>(basePath('/studio'))
  }

  async function getQuestionnaire() {
    return apiGet<RfxQuestionnaireDefinition>(basePath('/questionnaire'))
  }

  async function loadStudio() {
    loading.value = true
    error.value = null
    try {
      const data = await getStudio()
      applyStudioResponse(data)
      if (autosaveStatus.value !== 'dirty' && autosaveStatus.value !== 'saving') {
        autosaveStatus.value = data.draft_version ? 'saved' : 'idle'
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'LOAD_FAILED'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function loadQuestionnaire() {
    loading.value = true
    error.value = null
    try {
      questionnaire.value = await getQuestionnaire()
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'LOAD_FAILED'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function loadAll() {
    await Promise.all([loadStudio(), loadQuestionnaire()])
  }

  async function saveDraft(body?: RfxSaveDraftRequest): Promise<RfxVersionRecord> {
    saving.value = true
    autosaveStatus.value = 'saving'
    fieldError.value = null
    error.value = null
    try {
      const payload: RfxSaveDraftRequest = body ?? {}
      if (payload.expected_version === undefined && draftVersion.value != null) {
        payload.expected_version = draftVersion.value
      }
      const result = await apiPost<RfxVersionRecord>(basePath('/save-draft'), payload)
      if (studio.value?.draft_version) {
        studio.value.draft_version = { ...studio.value.draft_version, ...result }
      } else if (studio.value) {
        studio.value.draft_version = result
      }
      lastSavedAt.value = result.updated_at ?? lastSavedAt.value
      if (canShowSavedStatus()) autosaveStatus.value = 'saved'
      return result
    } catch (err) {
      handleMutationError(err)
      throw err
    } finally {
      saving.value = false
    }
  }

  async function validatePublish() {
    const result = await apiPost<RfxPublishReadinessResult>(basePath('/validate-publish'))
    publishReadiness.value = result
    return result
  }

  async function createSection(payload: RfxCreateSectionRequest) {
    const section = await apiPost<RfxSection>(basePath('/sections'), payload)
    await loadStudio()
    markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
    return section
  }

  function scheduleSectionUpdate(sectionId: string, payload: Omit<RfxUpdateSectionRequest, 'expected_version'>) {
    schedulePatch(`section:${sectionId}`, async () => {
      const expectedVersion = getSectionVersion(sectionId)
      if (expectedVersion == null) return
      const section = await apiPatch<RfxSection>(
        basePath(`/sections/${sectionId}`),
        { ...payload, expected_version: expectedVersion },
      )
      mergeSectionInStudio(section)
      lastSavedAt.value = section.updated_at ?? lastSavedAt.value
    })
  }

  async function updateSection(sectionId: string, payload: RfxUpdateSectionRequest) {
    try {
      const section = await apiPatch<RfxSection>(basePath(`/sections/${sectionId}`), payload)
      mergeSectionInStudio(section)
      markSavedFromTimestamp(section.updated_at)
      return section
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  async function deleteSection(sectionId: string, expectedVersion: number) {
    try {
      await apiDeleteWithBody(basePath(`/sections/${sectionId}`), { expected_version: expectedVersion })
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  async function reorderSections(payload: RfxReorderSectionsRequest) {
    try {
      await apiPost<void>(basePath('/sections/reorder'), payload)
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  async function createQuestion(payload: RfxCreateQuestionRequest) {
    try {
      const question = await apiPost<RfxQuestion>(basePath('/questions'), payload)
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
      return question
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  function scheduleQuestionUpdate(
    questionId: string,
    payload: Omit<RfxUpdateQuestionRequest, 'expected_version'>,
  ) {
    schedulePatch(`question:${questionId}`, async () => {
      const expectedVersion = getQuestionVersion(questionId)
      if (expectedVersion == null) return
      const question = await apiPatch<RfxQuestion>(
        basePath(`/questions/${questionId}`),
        { ...payload, expected_version: expectedVersion },
      )
      mergeQuestionInStudio(question)
      lastSavedAt.value = question.updated_at ?? lastSavedAt.value
    })
  }

  async function updateQuestion(questionId: string, payload: RfxUpdateQuestionRequest) {
    try {
      const question = await apiPatch<RfxQuestion>(basePath(`/questions/${questionId}`), payload)
      mergeQuestionInStudio(question)
      markSavedFromTimestamp(question.updated_at)
      return question
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  async function deleteQuestion(questionId: string, expectedVersion: number) {
    try {
      await apiDeleteWithBody(basePath(`/questions/${questionId}`), { expected_version: expectedVersion })
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  async function duplicateQuestion(questionId: string) {
    try {
      const question = await apiPost<RfxQuestion>(basePath(`/questions/${questionId}/duplicate`), {})
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
      return question
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  async function reorderQuestions(payload: RfxReorderQuestionsRequest) {
    try {
      await apiPost<void>(basePath('/questions/reorder'), payload)
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  async function createOption(questionId: string, payload: RfxCreateOptionRequest) {
    try {
      const option = await apiPost<RfxQuestionOption>(basePath(`/questions/${questionId}/options`), payload)
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
      return option
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  function scheduleOptionUpdate(
    questionId: string,
    optionId: string,
    payload: Omit<RfxUpdateOptionRequest, 'expected_version'>,
  ) {
    schedulePatch(`option:${questionId}:${optionId}`, async () => {
      const expectedVersion = getOptionVersion(questionId, optionId)
      if (expectedVersion == null) return
      await apiPatch<RfxQuestionOption>(
        basePath(`/questions/${questionId}/options/${optionId}`),
        { ...payload, expected_version: expectedVersion },
      )
      await loadStudio()
      lastSavedAt.value = studio.value?.draft_version?.updated_at ?? lastSavedAt.value
    })
  }

  async function updateOption(questionId: string, optionId: string, payload: RfxUpdateOptionRequest) {
    try {
      const option = await apiPatch<RfxQuestionOption>(
        basePath(`/questions/${questionId}/options/${optionId}`),
        payload,
      )
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
      return option
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  async function deleteOption(questionId: string, optionId: string, expectedVersion: number) {
    try {
      await apiDeleteWithBody(basePath(`/questions/${questionId}/options/${optionId}`), {
        expected_version: expectedVersion,
      })
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  async function createRule(payload: RfxCreateRuleRequest) {
    try {
      const rule = await apiPost<RfxQuestionRule>(basePath('/rules'), payload)
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
      return rule
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  function scheduleRuleUpdate(ruleId: string, payload: Omit<RfxUpdateRuleRequest, 'expected_version'>) {
    schedulePatch(`rule:${ruleId}`, async () => {
      const expectedVersion = getRuleVersion(ruleId)
      if (expectedVersion == null) return
      await apiPatch<RfxQuestionRule>(
        basePath(`/rules/${ruleId}`),
        { ...payload, expected_version: expectedVersion },
      )
      await loadStudio()
      lastSavedAt.value = studio.value?.draft_version?.updated_at ?? lastSavedAt.value
    })
  }

  async function updateRule(ruleId: string, payload: RfxUpdateRuleRequest) {
    try {
      const rule = await apiPatch<RfxQuestionRule>(basePath(`/rules/${ruleId}`), payload)
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
      return rule
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  async function deleteRule(ruleId: string, expectedVersion: number) {
    try {
      await apiDeleteWithBody(basePath(`/rules/${ruleId}`), { expected_version: expectedVersion })
      await loadStudio()
      markSavedFromTimestamp(studio.value?.draft_version?.updated_at)
    } catch (err) {
      handleMutationError(err)
      throw err
    }
  }

  async function flushPendingPatches() {
    clearAllPatchTimers()
    if (inFlightPatch) await inFlightPatch.catch(() => undefined)
  }

  async function reloadAfterConflict() {
    autosaveStatus.value = 'idle'
    fieldError.value = null
    error.value = null
    await loadStudio()
  }

  onBeforeUnmount(() => {
    clearAllPatchTimers()
  })

  return {
    studio,
    questionnaire,
    publishReadiness,
    autosaveStatus,
    lastSavedAt,
    draftVersion,
    loading,
    saving,
    error,
    fieldError,
    getStudio,
    getQuestionnaire,
    loadStudio,
    loadQuestionnaire,
    loadAll,
    saveDraft,
    validatePublish,
    createSection,
    scheduleSectionUpdate,
    updateSection,
    deleteSection,
    reorderSections,
    createQuestion,
    scheduleQuestionUpdate,
    updateQuestion,
    deleteQuestion,
    duplicateQuestion,
    reorderQuestions,
    createOption,
    scheduleOptionUpdate,
    updateOption,
    deleteOption,
    createRule,
    scheduleRuleUpdate,
    updateRule,
    deleteRule,
    markDirty,
    flushPendingPatches,
    reloadAfterConflict,
  }
}

export type RfxQuestionnaireApi = ReturnType<typeof useRfxQuestionnaireApi>

export function useInjectedRfxQuestionnaireApi(): RfxQuestionnaireApi {
  const api = inject<RfxQuestionnaireApi>(RFX_QUESTIONNAIRE_API_KEY)
  if (!api) throw new Error('RfxQuestionnaireApi not provided')
  return api
}
