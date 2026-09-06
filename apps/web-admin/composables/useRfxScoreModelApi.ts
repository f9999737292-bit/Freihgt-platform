import type {
  PutScoreModelInput,
  ScoreBindingInput,
  ScoreCriterionInput,
  ScoreModelEditorState,
  ScoreModelReadinessResult,
  ScoreModelView,
} from '~/types/rfx-score-model'
import { ApiError } from '~/composables/useApi'
import { rfxEventApiPath } from '~/utils/rfxQuestionnaireApiRoutes'
import { deriveEditorState } from '~/utils/rfxStudioScoring'

export const RFX_SCORE_MODEL_API_KEY = Symbol('rfxScoreModelApi')

export function useRfxScoreModelApi(rfxEventId: Ref<string> | string) {
  const { apiGet, apiPut, apiPost } = useApi()

  const view = ref<ScoreModelView | null>(null)
  const draftCriteria = ref<ScoreCriterionInput[]>([])
  const draftBindings = ref<ScoreBindingInput[]>([])
  const readiness = ref<ScoreModelReadinessResult | null>(null)
  const loading = ref(false)
  const loadFailed = ref(false)
  const saving = ref(false)
  const saveFailed = ref(false)
  const validating = ref(false)
  const publishing = ref(false)
  const dirty = ref(false)
  const error = ref<string | null>(null)

  const eventId = computed(() => (typeof rfxEventId === 'string' ? rfxEventId : rfxEventId.value))

  const isPublished = computed(() => view.value?.model.status === 'PUBLISHED')

  const editorState = computed<ScoreModelEditorState>(() =>
    deriveEditorState({
      loading: loading.value,
      loadFailed: loadFailed.value,
      published: isPublished.value,
      saving: saving.value,
      saveFailed: saveFailed.value,
      validating: validating.value,
      publishing: publishing.value,
      dirty: dirty.value,
      readiness: readiness.value,
    }),
  )

  function basePath(suffix: string) {
    return rfxEventApiPath(eventId.value, suffix)
  }

  function syncDraftFromView(data: ScoreModelView) {
    draftCriteria.value = data.criteria.map((c, index) => ({
      criterion_code: c.criterion_code,
      name: c.name,
      weight: c.weight,
      normalization_json:
        typeof c.normalization_json === 'string'
          ? JSON.parse(c.normalization_json)
          : { ...(c.normalization_json as Record<string, unknown>) },
      sort_order: c.sort_order ?? index + 1,
    }))
    const codeByCriterionId = new Map(data.criteria.map((c) => [c.id, c.criterion_code]))
    const questionCodeById = new Map<string, string>()
    draftBindings.value = data.bindings.map((b) => {
      const criterionCode = b.criterion_id ? codeByCriterionId.get(b.criterion_id) ?? '' : ''
      return {
        criterion_code: criterionCode,
        question_code: questionCodeById.get(b.question_id ?? '') ?? '',
        knockout_rule_json:
          b.knockout_rule_json && typeof b.knockout_rule_json === 'object'
            ? { ...(b.knockout_rule_json as Record<string, unknown>) }
            : b.knockout_rule_json
              ? JSON.parse(String(b.knockout_rule_json))
              : null,
      }
    })
    dirty.value = false
  }

  function buildPutInput(): PutScoreModelInput {
    return {
      criteria: draftCriteria.value.map((c, index) => ({
        ...c,
        sort_order: c.sort_order ?? index + 1,
      })),
      bindings: draftBindings.value,
    }
  }

  async function loadScoreModel() {
    loading.value = true
    loadFailed.value = false
    error.value = null
    try {
      const data = await apiGet<ScoreModelView>(basePath('/score-model'))
      view.value = data
      readiness.value = data.readiness ?? null
      syncDraftFromView(data)
    } catch (err) {
      loadFailed.value = true
      if (err instanceof ApiError && err.status === 404) {
        view.value = null
        draftCriteria.value = []
        draftBindings.value = []
        readiness.value = null
        dirty.value = false
        loadFailed.value = false
        return
      }
      error.value = err instanceof Error ? err.message : 'LOAD_FAILED'
      throw err
    } finally {
      loading.value = false
    }
  }

  function markDirty() {
    if (!isPublished.value) dirty.value = true
  }

  function setCriteria(next: ScoreCriterionInput[]) {
    draftCriteria.value = next
    markDirty()
  }

  function setBindings(next: ScoreBindingInput[]) {
    draftBindings.value = next
    markDirty()
  }

  function updateBinding(criterionCode: string, patch: Partial<ScoreBindingInput>) {
    const index = draftBindings.value.findIndex((b) => b.criterion_code === criterionCode)
    if (index >= 0) {
      draftBindings.value[index] = { ...draftBindings.value[index], ...patch }
    } else {
      draftBindings.value.push({ criterion_code: criterionCode, question_code: '', ...patch })
    }
    markDirty()
  }

  async function saveDraft() {
    if (isPublished.value) return view.value
    saving.value = true
    saveFailed.value = false
    error.value = null
    try {
      const data = await apiPut<ScoreModelView>(basePath('/score-model'), buildPutInput())
      view.value = data
      readiness.value = data.readiness ?? readiness.value
      syncDraftFromView(data)
      return data
    } catch (err) {
      saveFailed.value = true
      if (err instanceof ApiError) {
        error.value = err.message || `HTTP_${err.status}`
      } else {
        error.value = err instanceof Error ? err.message : 'SAVE_FAILED'
      }
      throw err
    } finally {
      saving.value = false
    }
  }

  async function validateReadiness() {
    validating.value = true
    error.value = null
    try {
      if (dirty.value && !isPublished.value) {
        await saveDraft()
      }
      const result = await apiPost<ScoreModelReadinessResult>(basePath('/score-model/validate'))
      readiness.value = result
      return result
    } catch (err) {
      if (err instanceof ApiError) error.value = err.message || `HTTP_${err.status}`
      throw err
    } finally {
      validating.value = false
    }
  }

  async function publish() {
    if (isPublished.value) return view.value
    publishing.value = true
    error.value = null
    try {
      const data = await apiPost<ScoreModelView>(basePath('/score-model/publish'))
      view.value = data
      readiness.value = data.readiness ?? { ready: true }
      syncDraftFromView(data)
      dirty.value = false
      return data
    } catch (err) {
      if (err instanceof ApiError) error.value = err.message || `HTTP_${err.status}`
      throw err
    } finally {
      publishing.value = false
    }
  }

  function bindQuestionCodes(questions: Array<{ id: string; question_code: string }>) {
    const byId = new Map(questions.map((q) => [q.id, q.question_code]))
    if (!view.value) return
    draftBindings.value = view.value.bindings.map((b) => ({
      criterion_code:
        view.value!.criteria.find((c) => c.id === b.criterion_id)?.criterion_code ?? '',
      question_code: byId.get(b.question_id ?? '') ?? '',
      knockout_rule_json:
        b.knockout_rule_json && typeof b.knockout_rule_json === 'object'
          ? { ...(b.knockout_rule_json as Record<string, unknown>) }
          : null,
    }))
  }

  return {
    view,
    draftCriteria,
    draftBindings,
    readiness,
    loading,
    loadFailed,
    saving,
    saveFailed,
    validating,
    publishing,
    dirty,
    error,
    isPublished,
    editorState,
    loadScoreModel,
    saveDraft,
    validateReadiness,
    publish,
    setCriteria,
    setBindings,
    updateBinding,
    markDirty,
    bindQuestionCodes,
    buildPutInput,
  }
}
