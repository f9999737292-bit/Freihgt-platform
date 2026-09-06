import type {
  CarrierAnswerPatchItem,
  CarrierAutosaveStatus,
  CarrierGlobalErrorItem,
  CarrierQuestion,
  CarrierResponseProductStatus,
  CarrierResponseWorkspace,
  CarrierSectionSummary,
  CarrierValidationErrorItem,
} from '~/types/carrierResponse'
import { useCarrierResponseApi } from '~/composables/useCarrierResponseApi'
import {
  autosaveStatusFromCarrierHttpError,
  canShowCarrierSaved,
  isCarrierLeaveWarningState,
  markCarrierDirtyTransition,
  markCarrierSavedTransition,
} from '~/utils/carrierResponseAutosave'
import {
  isCarrierConflictError,
  isCarrierValidationError,
} from '~/utils/carrierResponseErrors'
import {
  buildGlobalErrorSummary,
  computeSectionSummaries,
  countBlockingErrors,
  groupErrorsByQuestionId,
  shouldBlockSubmit,
} from '~/utils/carrierResponseValidation'
import {
  answersByCodeFromLocalValues,
  buildQuestionMaps,
  buildRulesByTargetQuestionId,
  isHiddenQuestionForSave,
  mergeDebouncedAnswerPatches,
  resolveQuestionRequired,
  resolveQuestionVisibility,
  type CarrierRuleRuntimeContext,
} from '~/utils/carrierResponseRuntime'
import { ApiError } from '~/utils/apiClient'

const PATCH_DEBOUNCE_MS = 800

export function useCarrierResponseWorkspace(
  eventId: Ref<string>,
  carrierCompanyId: Ref<string>,
) {
  const api = useCarrierResponseApi()
  const { t } = useI18n()

  const workspace = ref<CarrierResponseWorkspace | null>(null)
  const loading = ref(false)
  const loadError = ref<string | null>(null)
  const autosaveStatus = ref<CarrierAutosaveStatus>('empty')
  const lastSavedAt = ref<string | null>(null)
  const saveVersion = ref(0)
  const completionPercent = ref(0)
  const submitBlockedMessage = ref<string | null>(null)
  const showLeaveWarning = ref(false)
  const pendingLeaveResolve = ref<((stay: boolean) => void) | null>(null)

  const localValues = reactive(new Map<string, unknown>())
  const serverValues = reactive(new Map<string, unknown>())
  const fieldErrors = reactive(new Map<string, CarrierValidationErrorItem[]>())
  const serverValidation = ref<{ valid: boolean; blocking_error_count: number } | null>(null)

  const activeSectionId = ref<string | null>(null)
  const focusQuestionId = ref<string | null>(null)

  const patchTimer = ref<ReturnType<typeof setTimeout> | null>(null)
  const pendingPatches = ref<CarrierAnswerPatchItem[]>([])
  let inFlightSave: Promise<void> | null = null
  let reloadAfterSave = false

  const productStatus = computed<CarrierResponseProductStatus>(
    () => workspace.value?.product_status ?? 'NOT_STARTED',
  )
  const isSubmitted = computed(() => productStatus.value === 'SUBMITTED')
  const isLocked = computed(() => isSubmitted.value)

  const questionMaps = computed(() => {
    if (!workspace.value?.questionnaire) {
      return { questionById: new Map<string, CarrierQuestion>(), questionByCode: new Map<string, CarrierQuestion>() }
    }
    return buildQuestionMaps(workspace.value.questionnaire)
  })

  const questionById = computed(() => questionMaps.value.questionById)
  const questionByCode = computed(() => questionMaps.value.questionByCode)

  const rulesByTarget = computed(() =>
    workspace.value?.questionnaire
      ? buildRulesByTargetQuestionId(workspace.value.questionnaire.rules)
      : new Map(),
  )

  const runtimeContext = computed<CarrierRuleRuntimeContext | null>(() => {
    if (!workspace.value?.questionnaire) return null
    return {
      questionnaire: workspace.value.questionnaire,
      answersByQuestionCode: answersByCodeFromLocalValues(workspace.value.questionnaire, localValues),
    }
  })

  const sectionSummaries = computed<CarrierSectionSummary[]>(() => {
    if (!workspace.value?.questionnaire) return []
    return computeSectionSummaries({
      questionnaire: workspace.value.questionnaire,
      localValues,
      fieldErrors,
    })
  })

  const globalErrors = computed<CarrierGlobalErrorItem[]>(() => {
    if (!workspace.value?.questionnaire) return []
    const flat: CarrierValidationErrorItem[] = []
    for (const items of fieldErrors.values()) flat.push(...items)
    return buildGlobalErrorSummary(flat, workspace.value.questionnaire, localValues, t)
  })

  const blockingErrorCount = computed(() => countBlockingErrors(fieldErrors))

  function applyWorkspace(data: CarrierResponseWorkspace) {
    workspace.value = data
    saveVersion.value = data.save_version
    completionPercent.value = data.completion_percent
    lastSavedAt.value = data.last_saved_at ?? null
    localValues.clear()
    serverValues.clear()
    for (const answer of data.answers ?? []) {
      localValues.set(answer.question_id, answer.value)
      serverValues.set(answer.question_id, answer.value)
    }
    if (!activeSectionId.value && data.questionnaire.sections.length > 0) {
      activeSectionId.value = data.questionnaire.sections[0].section.id
    }
    if (data.product_status === 'SUBMITTED') {
      autosaveStatus.value = 'saved'
    } else if (autosaveStatus.value === 'empty') {
      autosaveStatus.value = data.product_status === 'NOT_STARTED' ? 'empty' : 'saved'
    }
  }

  function syncFromServerAnswers(data: CarrierResponseWorkspace) {
    for (const answer of data.answers ?? []) {
      serverValues.set(answer.question_id, answer.value)
      if (!fieldErrors.has(answer.question_id)) {
        localValues.set(answer.question_id, answer.value)
      }
    }
    saveVersion.value = data.save_version
    completionPercent.value = data.completion_percent
    lastSavedAt.value = data.last_saved_at ?? null
  }

  async function loadWorkspace(options: { startIfMissing?: boolean } = {}) {
    if (!carrierCompanyId.value) return
    loading.value = true
    loadError.value = null
    try {
      let data: CarrierResponseWorkspace
      try {
        data = await api.getCarrierResponse(eventId.value, carrierCompanyId.value)
      } catch (err) {
        if (err instanceof ApiError && err.status === 404 && options.startIfMissing) {
          data = await api.startCarrierResponse(eventId.value, carrierCompanyId.value)
        } else {
          throw err
        }
      }
      applyWorkspace(data)
      if (data.product_status === 'NOT_STARTED' && options.startIfMissing) {
        const started = await api.startCarrierResponse(eventId.value, carrierCompanyId.value)
        applyWorkspace(started)
      }
    } catch (err) {
      loadError.value = err instanceof Error ? err.message : 'LOAD_FAILED'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function reloadFromServer() {
    const data = await api.getCarrierResponse(eventId.value, carrierCompanyId.value)
    applyWorkspace(data)
    fieldErrors.clear()
    autosaveStatus.value = data.product_status === 'SUBMITTED' ? 'saved' : 'saved'
    return data
  }

  function markDirty() {
    if (isLocked.value) return
    autosaveStatus.value = markCarrierDirtyTransition(autosaveStatus.value)
  }

  function queuePatch(patch: CarrierAnswerPatchItem) {
    if (isLocked.value) return
    pendingPatches.value = mergeDebouncedAnswerPatches([...pendingPatches.value, patch])
    markDirty()
    if (patchTimer.value) clearTimeout(patchTimer.value)
    patchTimer.value = setTimeout(() => {
      patchTimer.value = null
      void flushPendingPatches().catch(() => undefined)
    }, PATCH_DEBOUNCE_MS)
  }

  async function flushPendingPatches() {
    if (isLocked.value || pendingPatches.value.length === 0) return
    if (inFlightSave) {
      reloadAfterSave = true
      await inFlightSave.catch(() => undefined)
    }
    const batch = [...pendingPatches.value]
    pendingPatches.value = []
    inFlightSave = saveBatch(batch)
    await inFlightSave
    inFlightSave = null
    if (reloadAfterSave && pendingPatches.value.length > 0) {
      reloadAfterSave = false
      await flushPendingPatches()
    }
  }

  async function saveBatch(batch: CarrierAnswerPatchItem[]) {
    if (!workspace.value || isLocked.value || batch.length === 0) return
    autosaveStatus.value = 'saving'

    const ctx = runtimeContext.value
    const filtered = batch.filter((item) => {
      const question = questionById.value.get(item.question_id)
      if (!question || !ctx) return true
      return !isHiddenQuestionForSave(question, ctx, rulesByTarget.value)
    })

    if (filtered.length === 0) {
      autosaveStatus.value = 'dirty'
      return
    }

    try {
      const result = await api.patchCarrierAnswers(
        eventId.value,
        { save_version: saveVersion.value, answers: filtered },
        carrierCompanyId.value,
      )
      saveVersion.value = result.save_version
      completionPercent.value = result.completion_percent
      lastSavedAt.value = result.last_saved_at
      for (const item of filtered) {
        serverValues.set(item.question_id, localValues.get(item.question_id))
        fieldErrors.delete(item.question_id)
      }
      const next = markCarrierSavedTransition(autosaveStatus.value, result.last_saved_at)
      autosaveStatus.value = next.status
      if (next.lastSavedAt) lastSavedAt.value = next.lastSavedAt
      workspace.value = workspace.value
        ? {
            ...workspace.value,
            save_version: result.save_version,
            completion_percent: result.completion_percent,
            last_saved_at: result.last_saved_at,
            product_status: 'IN_PROGRESS',
          }
        : workspace.value
    } catch (err) {
      if (isCarrierValidationError(err)) {
        autosaveStatus.value = 'invalid'
        const grouped = groupErrorsByQuestionId(err.errors)
        for (const [questionId, items] of grouped.entries()) {
          fieldErrors.set(questionId, items)
        }
        return
      }
      if (isCarrierConflictError(err)) {
        autosaveStatus.value = 'conflict'
        return
      }
      autosaveStatus.value = autosaveStatusFromCarrierHttpError(
        err instanceof ApiError ? err.status : 500,
      )
    }
  }

  function setLocalAnswer(questionId: string, sectionId: string, value: unknown) {
    if (isLocked.value) return
    localValues.set(questionId, value)
    fieldErrors.delete(questionId)
    queuePatch({ question_id: questionId, section_id: sectionId, value })
  }

  function getLocalAnswer(questionId: string): unknown {
    return localValues.get(questionId)
  }

  function isQuestionVisible(question: CarrierQuestion): boolean {
    if (!runtimeContext.value) return true
    return resolveQuestionVisibility(question, runtimeContext.value, rulesByTarget.value)
  }

  function isQuestionRequired(question: CarrierQuestion): boolean {
    if (!runtimeContext.value) return question.required
    return resolveQuestionRequired(question, runtimeContext.value, rulesByTarget.value)
  }

  function inlineErrorsForQuestion(questionId: string): CarrierValidationErrorItem[] {
    return fieldErrors.get(questionId) ?? []
  }

  async function validateBeforeSubmit() {
    const result = await api.validateCarrierResponse(eventId.value, carrierCompanyId.value)
    serverValidation.value = {
      valid: result.valid,
      blocking_error_count: result.blocking_error_count,
    }
    completionPercent.value = result.completion_percent
    fieldErrors.clear()
    const grouped = groupErrorsByQuestionId(result.errors)
    for (const [questionId, items] of grouped.entries()) {
      fieldErrors.set(questionId, items)
    }
    return result
  }

  async function submitResponse() {
    if (isLocked.value) return
    await flushPendingPatches().catch(() => undefined)
    const validation = await validateBeforeSubmit()
    if (!validation.valid || validation.blocking_error_count > 0) {
      submitBlockedMessage.value = t('carrierResponse.submit.blocked', {
        count: validation.blocking_error_count,
      })
      autosaveStatus.value = validation.blocking_error_count > 0 ? 'invalid' : autosaveStatus.value
      return false
    }
    const result = await api.submitCarrierResponse(
      eventId.value,
      saveVersion.value,
      carrierCompanyId.value,
    )
    saveVersion.value = result.save_version
    if (workspace.value) {
      workspace.value = {
        ...workspace.value,
        product_status: 'SUBMITTED',
        status: result.status,
        submitted_at: result.submitted_at,
        save_version: result.save_version,
      }
    }
    autosaveStatus.value = 'saved'
    submitBlockedMessage.value = null
    if (patchTimer.value) {
      clearTimeout(patchTimer.value)
      patchTimer.value = null
    }
    pendingPatches.value = []
    return true
  }

  function navigateToError(item: CarrierGlobalErrorItem) {
    if (item.sectionId) activeSectionId.value = item.sectionId
    focusQuestionId.value = item.questionId
  }

  function clearFocusQuestion() {
    focusQuestionId.value = null
  }

  function requestLeave(): Promise<boolean> {
    if (!isCarrierLeaveWarningState(autosaveStatus.value) && blockingErrorCount.value === 0) {
      return Promise.resolve(true)
    }
    showLeaveWarning.value = true
    return new Promise((resolve) => {
      pendingLeaveResolve.value = resolve
    })
  }

  function confirmLeave(stay: boolean) {
    showLeaveWarning.value = false
    pendingLeaveResolve.value?.(stay)
    pendingLeaveResolve.value = null
  }

  function discardInvalidAndLeave() {
    for (const [questionId] of fieldErrors.entries()) {
      const serverValue = serverValues.get(questionId)
      if (serverValue !== undefined) {
        localValues.set(questionId, serverValue)
      } else {
        localValues.delete(questionId)
      }
    }
    fieldErrors.clear()
    autosaveStatus.value = canShowCarrierSaved(autosaveStatus.value) ? 'saved' : 'empty'
    confirmLeave(false)
  }

  const canSubmit = computed(() =>
    !shouldBlockSubmit(
      autosaveStatus.value,
      blockingErrorCount.value,
      serverValidation.value?.valid ?? null,
      isSubmitted.value,
    ) && !isLocked.value,
  )

  onBeforeRouteLeave((_to, _from, next) => {
    void requestLeave().then((allow) => {
      if (allow) next()
      else next(false)
    })
  })

  watch(carrierCompanyId, () => {
    void loadWorkspace({ startIfMissing: true }).catch(() => undefined)
  })

  return {
    workspace,
    loading,
    loadError,
    autosaveStatus,
    lastSavedAt,
    saveVersion,
    completionPercent,
    productStatus,
    isSubmitted,
    isLocked,
    localValues,
    serverValues,
    fieldErrors,
    sectionSummaries,
    globalErrors,
    blockingErrorCount,
    activeSectionId,
    focusQuestionId,
    submitBlockedMessage,
    showLeaveWarning,
    canSubmit,
    loadWorkspace,
    reloadFromServer,
    setLocalAnswer,
    getLocalAnswer,
    isQuestionVisible,
    isQuestionRequired,
    inlineErrorsForQuestion,
    validateBeforeSubmit,
    submitResponse,
    navigateToError,
    clearFocusQuestion,
    requestLeave,
    confirmLeave,
    discardInvalidAndLeave,
    flushPendingPatches,
    questionById,
    questionByCode,
  }
}
