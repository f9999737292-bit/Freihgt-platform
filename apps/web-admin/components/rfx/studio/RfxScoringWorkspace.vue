<script setup lang="ts">
import type { ScoreBindingInput, ScoreCriterionInput } from '~/types/rfx-score-model'
import type { RfxQuestion } from '~/types/rfx-questionnaire'
import { RFX_QUESTIONNAIRE_API_KEY } from '~/composables/useRfxQuestionnaireApi'
import { RFX_SCORE_MODEL_API_KEY, useRfxScoreModelApi } from '~/composables/useRfxScoreModelApi'
import {
  bindingForCriterion,
  defaultNormalizationForQuestionType,
  editorStateLabel,
  filterBindableQuestions,
  isScoringCompatibleQuestionType,
  multiSelectAggregation,
  newCriterion,
  normalizationTypeOf,
  parseJsonField,
  readinessErrorMessage,
  totalWeight,
} from '~/utils/rfxStudioScoring'

const props = defineProps<{ eventId: string }>()

const { t } = useI18n()
const { pushToast } = useToast()

const questionnaireApi = inject(RFX_QUESTIONNAIRE_API_KEY)!
const scoreApi = inject(RFX_SCORE_MODEL_API_KEY) ?? useRfxScoreModelApi(toRef(props, 'eventId'))

const showPublishConfirm = ref(false)

const bindableQuestions = computed(() => {
  const studioSections = questionnaireApi.studio.value?.sections ?? []
  const questionnaireSections = questionnaireApi.questionnaire.value?.sections ?? []
  const sections = studioSections.length > 0 ? studioSections : questionnaireSections
  const questions: Array<RfxQuestion & { sectionTitle?: string }> = []
  for (const section of sections) {
    for (const q of section.questions ?? []) {
      questions.push({ ...q, sectionTitle: section.title })
    }
  }
  return filterBindableQuestions(
    questions.map((q) => ({
      id: q.id,
      question_code: q.question_code,
      label: q.label,
      question_type: q.question_type,
      options: (q.options ?? []).map((o) => ({
        id: o.id,
        option_code: o.option_code,
        label: o.label,
      })),
    })),
  )
})

const questionByCode = computed(() => {
  const map = new Map<string, (typeof bindableQuestions.value)[number]>()
  for (const q of bindableQuestions.value) map.set(q.question_code, q)
  return map
})

const weightTotal = computed(() => totalWeight(scoreApi.draftCriteria.value))

async function loadAll() {
  await questionnaireApi.loadAll()
  await scoreApi.loadScoreModel()
  if (scoreApi.view.value) {
    scoreApi.bindQuestionCodes(
      bindableQuestions.value.map((q) => ({ id: q.id, question_code: q.question_code })),
    )
  }
}

function addCriterion() {
  const next = [...scoreApi.draftCriteria.value, newCriterion(scoreApi.draftCriteria.value.length + 1)]
  scoreApi.setCriteria(next)
}

function removeCriterion(code: string) {
  scoreApi.setCriteria(scoreApi.draftCriteria.value.filter((c) => c.criterion_code !== code))
  scoreApi.setBindings(scoreApi.draftBindings.value.filter((b) => b.criterion_code !== code))
}

function updateCriterion(index: number, patch: Partial<ScoreCriterionInput>) {
  const next = scoreApi.draftCriteria.value.map((c, i) => (i === index ? { ...c, ...patch } : c))
  scoreApi.setCriteria(next)
}

function bindQuestion(criterionCode: string, questionCode: string) {
  const question = questionByCode.value.get(questionCode)
  const existing = bindingForCriterion(scoreApi.draftBindings.value, criterionCode)
  const norm = question
    ? defaultNormalizationForQuestionType(question.question_type)
    : existing
      ? undefined
      : { type: 'BOOLEAN_MAP', true_score: 100, false_score: 0 }
  scoreApi.updateBinding(criterionCode, { question_code: questionCode })
  if (norm) {
    const idx = scoreApi.draftCriteria.value.findIndex((c) => c.criterion_code === criterionCode)
    if (idx >= 0) {
      updateCriterion(idx, { normalization_json: norm })
    }
  }
}

function updateNormalization(index: number, norm: Record<string, unknown>) {
  updateCriterion(index, { normalization_json: norm })
}

function updateKnockout(criterionCode: string, rule: Record<string, unknown> | null) {
  scoreApi.updateBinding(criterionCode, { knockout_rule_json: rule })
}

function knockoutFor(criterionCode: string): Record<string, unknown> | null {
  const binding = bindingForCriterion(scoreApi.draftBindings.value, criterionCode)
  if (!binding?.knockout_rule_json) return null
  return parseJsonField(binding.knockout_rule_json as Record<string, unknown>)
}

function boundQuestionCode(criterionCode: string): string {
  return bindingForCriterion(scoreApi.draftBindings.value, criterionCode)?.question_code ?? ''
}

async function handleSave() {
  try {
    await scoreApi.saveDraft()
    pushToast('success', t('rfx.studio.scoring.saved'))
  } catch {
    pushToast('error', t('rfx.studio.scoring.saveFailed'))
  }
}

async function handleValidate() {
  try {
    const result = await scoreApi.validateReadiness()
    pushToast(result.ready ? 'success' : 'warning', result.ready ? t('rfx.studio.scoring.ready') : t('rfx.studio.scoring.notReady'))
  } catch {
    pushToast('error', t('rfx.studio.scoring.validateFailed'))
  }
}

async function handlePublish() {
  showPublishConfirm.value = false
  try {
    await scoreApi.publish()
    pushToast('success', t('rfx.studio.scoring.published'))
  } catch {
    pushToast('error', t('rfx.studio.scoring.publishFailed'))
  }
}

onMounted(loadAll)
watch(() => props.eventId, loadAll)
</script>

<template>
  <div class="scoring-workspace" data-testid="rfx-scoring-workspace">
    <div v-if="scoreApi.loading.value" class="state-banner" data-testid="scoring-state-loading">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="scoreApi.loadFailed.value" class="state-banner state-banner--error" data-testid="scoring-state-load-failed">
      {{ t('rfx.studio.scoring.loadFailed') }}
    </div>

    <template v-else>
      <UiCard class="scoring-header">
        <div class="header-row">
          <div>
            <h2>{{ t('rfx.studio.scoring.title') }}</h2>
            <p class="muted">{{ t('rfx.studio.scoring.subtitle') }}</p>
          </div>
          <UiBadge
            :status="scoreApi.isPublished.value ? 'PUBLISHED' : 'DRAFT'"
            data-testid="scoring-model-status"
          >
            {{ scoreApi.isPublished.value ? t('rfx.studio.scoring.statusPublished') : t('rfx.studio.scoring.statusDraft') }}
          </UiBadge>
        </div>

        <dl class="meta-grid">
          <div>
            <dt>{{ t('rfx.studio.scoring.modelVersion') }}</dt>
            <dd data-testid="scoring-model-version">{{ scoreApi.view.value?.model.model_version ?? '—' }}</dd>
          </div>
          <div>
            <dt>{{ t('rfx.studio.scoring.questionnaireVersion') }}</dt>
            <dd>{{ questionnaireApi.studio.value?.published_version?.version ?? questionnaireApi.draftVersion.value ?? '—' }}</dd>
          </div>
          <div>
            <dt>{{ t('rfx.studio.scoring.editorState') }}</dt>
            <dd data-testid="scoring-editor-state">{{ editorStateLabel(scoreApi.editorState.value, t) }}</dd>
          </div>
        </dl>

        <p v-if="scoreApi.isPublished.value" class="immutable-note" data-testid="scoring-published-lock">
          {{ t('rfx.studio.scoring.publishedImmutable') }}
        </p>
      </UiCard>

      <div v-if="!scoreApi.isPublished.value" class="toolbar">
        <UiButton data-testid="scoring-save-draft" :disabled="scoreApi.saving.value || !scoreApi.dirty.value" @click="handleSave">
          {{ scoreApi.saving.value ? t('rfx.studio.scoring.saving') : t('rfx.studio.scoring.saveDraft') }}
        </UiButton>
        <UiButton variant="secondary" data-testid="scoring-validate" :disabled="scoreApi.validating.value" @click="handleValidate">
          {{ t('rfx.studio.scoring.validateReadiness') }}
        </UiButton>
        <UiButton
          variant="primary"
          data-testid="scoring-publish"
          :disabled="!scoreApi.readiness.value?.ready || scoreApi.publishing.value"
          @click="showPublishConfirm = true"
        >
          {{ t('rfx.studio.scoring.publish') }}
        </UiButton>
        <span class="weight-total" data-testid="scoring-weight-total">
          {{ t('rfx.studio.scoring.weightTotal', { total: weightTotal }) }}
        </span>
      </div>

      <UiCard v-if="scoreApi.readiness.value && !scoreApi.readiness.value.ready" class="readiness-panel" data-testid="scoring-readiness-panel">
        <h3>{{ t('rfx.studio.scoring.readinessTitle') }}</h3>
        <ul>
          <li v-for="(err, idx) in scoreApi.readiness.value.errors ?? []" :key="`${err.code}-${idx}`" data-testid="scoring-readiness-error">
            <code>{{ err.code }}</code>
            <span>{{ readinessErrorMessage(err, t) }}</span>
          </li>
        </ul>
      </UiCard>

      <UiCard v-if="scoreApi.readiness.value?.ready && !scoreApi.isPublished.value" class="readiness-panel readiness-panel--ok" data-testid="scoring-readiness-ready">
        {{ t('rfx.studio.scoring.ready') }}
      </UiCard>

      <div class="criteria-list">
        <UiCard
          v-for="(criterion, index) in scoreApi.draftCriteria.value"
          :key="criterion.criterion_code"
          class="criterion-card"
          data-testid="scoring-criterion-card"
        >
          <div class="criterion-header">
            <h3>{{ t('rfx.studio.scoring.criterionTitle', { n: index + 1 }) }}</h3>
            <UiButton
              v-if="!scoreApi.isPublished.value"
              size="sm"
              variant="danger"
              data-testid="scoring-remove-criterion"
              @click="removeCriterion(criterion.criterion_code)"
            >
              {{ t('rfx.studio.scoring.removeCriterion') }}
            </UiButton>
          </div>

          <div class="field-grid">
            <label>
              {{ t('rfx.studio.scoring.criterionCode') }}
              <input
                v-model="criterion.criterion_code"
                :disabled="scoreApi.isPublished.value"
                data-testid="scoring-criterion-code"
                @input="updateCriterion(index, { criterion_code: criterion.criterion_code })"
              >
            </label>
            <label>
              {{ t('rfx.studio.scoring.criterionName') }}
              <input
                v-model="criterion.name"
                :disabled="scoreApi.isPublished.value"
                data-testid="scoring-criterion-name"
                @input="updateCriterion(index, { name: criterion.name })"
              >
            </label>
            <label>
              {{ t('rfx.studio.scoring.criterionWeight') }}
              <input
                v-model.number="criterion.weight"
                type="number"
                min="0"
                max="100"
                :disabled="scoreApi.isPublished.value"
                data-testid="scoring-criterion-weight"
                @input="updateCriterion(index, { weight: criterion.weight })"
              >
            </label>
            <label>
              {{ t('rfx.studio.scoring.boundQuestion') }}
              <select
                :value="boundQuestionCode(criterion.criterion_code)"
                :disabled="scoreApi.isPublished.value"
                data-testid="scoring-question-binding"
                @change="bindQuestion(criterion.criterion_code, ($event.target as HTMLSelectElement).value)"
              >
                <option value="">{{ t('rfx.studio.scoring.selectQuestion') }}</option>
                <option
                  v-for="q in bindableQuestions"
                  :key="q.id"
                  :value="q.question_code"
                >
                  {{ q.question_code }} — {{ q.label }} ({{ q.question_type }})
                </option>
              </select>
            </label>
          </div>

          <div v-if="boundQuestionCode(criterion.criterion_code)" class="normalization-block">
            <h4>{{ t('rfx.studio.scoring.normalizationTitle') }}</h4>

            <template v-if="normalizationTypeOf(criterion.normalization_json) === 'BOOLEAN_MAP'">
              <div class="field-grid" data-testid="scoring-yes-no-editor">
                <label>
                  {{ t('rfx.studio.scoring.trueScore') }}
                  <input
                    v-model.number="(criterion.normalization_json as Record<string, number>).true_score"
                    type="number"
                    :disabled="scoreApi.isPublished.value"
                    @input="updateNormalization(index, { ...criterion.normalization_json, type: 'BOOLEAN_MAP' })"
                  >
                </label>
                <label>
                  {{ t('rfx.studio.scoring.falseScore') }}
                  <input
                    v-model.number="(criterion.normalization_json as Record<string, number>).false_score"
                    type="number"
                    :disabled="scoreApi.isPublished.value"
                    @input="updateNormalization(index, { ...criterion.normalization_json, type: 'BOOLEAN_MAP' })"
                  >
                </label>
              </div>
              <div class="knockout-block" data-testid="scoring-knockout-editor">
                <h5>{{ t('rfx.studio.scoring.knockoutTitle') }}</h5>
                <p class="muted">{{ t('rfx.studio.scoring.knockoutHint') }}</p>
                <label class="knockout-toggle">
                  <input
                    type="checkbox"
                    :checked="Boolean(knockoutFor(criterion.criterion_code)?.value === false)"
                    :disabled="scoreApi.isPublished.value"
                    data-testid="scoring-knockout-boolean-false"
                    @change="updateKnockout(criterion.criterion_code, ($event.target as HTMLInputElement).checked ? { type: 'BOOLEAN_EQUALS', value: false } : null)"
                  >
                  {{ t('rfx.studio.scoring.knockoutWhenFalse') }}
                </label>
              </div>
            </template>

            <template v-else-if="normalizationTypeOf(criterion.normalization_json) === 'NUMBER_LINEAR'">
              <div class="field-grid" data-testid="scoring-number-linear-editor">
                <label>
                  {{ t('rfx.studio.scoring.numberMin') }}
                  <input
                    v-model.number="(criterion.normalization_json as Record<string, number>).min"
                    type="number"
                    :disabled="scoreApi.isPublished.value"
                    @input="updateNormalization(index, { ...criterion.normalization_json, type: 'NUMBER_LINEAR' })"
                  >
                </label>
                <label>
                  {{ t('rfx.studio.scoring.numberMax') }}
                  <input
                    v-model.number="(criterion.normalization_json as Record<string, number>).max"
                    type="number"
                    :disabled="scoreApi.isPublished.value"
                    @input="updateNormalization(index, { ...criterion.normalization_json, type: 'NUMBER_LINEAR' })"
                  >
                </label>
              </div>
            </template>

            <template v-else-if="normalizationTypeOf(criterion.normalization_json) === 'OPTION_MAP'">
              <div data-testid="scoring-single-select-editor">
                <p class="muted">{{ t('rfx.studio.scoring.optionMapHint') }}</p>
                <div
                  v-for="opt in questionByCode.get(boundQuestionCode(criterion.criterion_code))?.options ?? []"
                  :key="opt.option_code"
                  class="option-score-row"
                >
                  <span>{{ opt.option_code }} — {{ opt.label }}</span>
                  <input
                    type="number"
                    :value="(criterion.normalization_json as Record<string, Record<string, number>>).option_scores?.[opt.option_code] ?? 0"
                    :disabled="scoreApi.isPublished.value"
                    data-testid="scoring-option-score"
                    @input="updateNormalization(index, {
                      type: 'OPTION_MAP',
                      option_scores: {
                        ...((criterion.normalization_json as Record<string, Record<string, number>>).option_scores ?? {}),
                        [opt.option_code]: Number(($event.target as HTMLInputElement).value),
                      },
                    })"
                  >
                </div>
              </div>
            </template>

            <template v-else-if="normalizationTypeOf(criterion.normalization_json) === 'MULTI_SELECT'">
              <div data-testid="scoring-multi-select-editor">
                <p class="muted">
                  {{ t('rfx.studio.scoring.multiSelectAggregation', { mode: multiSelectAggregation(criterion.normalization_json) }) }}
                </p>
                <label v-if="multiSelectAggregation(criterion.normalization_json) === 'SUM_CAPPED'">
                  {{ t('rfx.studio.scoring.multiSelectCap') }}
                  <input
                    v-model.number="(criterion.normalization_json as Record<string, number>).cap"
                    type="number"
                    :disabled="scoreApi.isPublished.value"
                    @input="updateNormalization(index, { ...criterion.normalization_json, type: 'MULTI_SELECT', aggregation: 'SUM_CAPPED' })"
                  >
                </label>
                <div
                  v-for="opt in questionByCode.get(boundQuestionCode(criterion.criterion_code))?.options ?? []"
                  :key="opt.option_code"
                  class="option-score-row"
                >
                  <span>{{ opt.option_code }} — {{ opt.label }}</span>
                  <input
                    type="number"
                    :value="(criterion.normalization_json as Record<string, Record<string, number>>).option_scores?.[opt.option_code] ?? 0"
                    :disabled="scoreApi.isPublished.value"
                    @input="updateNormalization(index, {
                      type: 'MULTI_SELECT',
                      aggregation: multiSelectAggregation(criterion.normalization_json),
                      cap: (criterion.normalization_json as Record<string, number>).cap ?? 100,
                      option_scores: {
                        ...((criterion.normalization_json as Record<string, Record<string, number>>).option_scores ?? {}),
                        [opt.option_code]: Number(($event.target as HTMLInputElement).value),
                      },
                    })"
                  >
                </div>
              </div>
            </template>
          </div>
        </UiCard>
      </div>

      <UiButton
        v-if="!scoreApi.isPublished.value"
        variant="secondary"
        data-testid="scoring-add-criterion"
        @click="addCriterion"
      >
        {{ t('rfx.studio.scoring.addCriterion') }}
      </UiButton>
    </template>

    <UiModal v-if="showPublishConfirm" @close="showPublishConfirm = false">
      <template #title>{{ t('rfx.studio.scoring.publishConfirmTitle') }}</template>
      <p>{{ t('rfx.studio.scoring.publishConfirmBody') }}</p>
      <template #actions>
        <UiButton variant="secondary" @click="showPublishConfirm = false">{{ t('common.cancel') }}</UiButton>
        <UiButton variant="primary" data-testid="scoring-publish-confirm" @click="handlePublish">{{ t('rfx.studio.scoring.publish') }}</UiButton>
      </template>
    </UiModal>
  </div>
</template>

<style scoped>
.scoring-workspace { display: flex; flex-direction: column; gap: 1rem; }
.state-banner { padding: 1rem; text-align: center; color: var(--color-text-muted); }
.state-banner--error { color: var(--color-danger); }
.scoring-header { padding: 1rem; }
.header-row { display: flex; justify-content: space-between; align-items: flex-start; gap: 1rem; }
.meta-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 0.75rem; margin-top: 1rem; }
.meta-grid dt { font-size: 0.75rem; color: var(--color-text-muted); }
.meta-grid dd { margin: 0; font-weight: 600; }
.immutable-note { margin-top: 0.75rem; color: var(--color-text-muted); }
.toolbar { display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: center; }
.weight-total { margin-left: auto; font-size: 0.875rem; color: var(--color-text-muted); }
.readiness-panel { padding: 1rem; }
.readiness-panel--ok { border-color: var(--color-success); color: var(--color-success); }
.readiness-panel ul { margin: 0; padding-left: 1.25rem; }
.criterion-card { padding: 1rem; }
.criterion-header { display: flex; justify-content: space-between; align-items: center; }
.field-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 0.75rem; margin-top: 0.75rem; }
.field-grid label { display: flex; flex-direction: column; gap: 0.25rem; font-size: 0.875rem; }
.field-grid input, .field-grid select { padding: 0.375rem 0.5rem; border: 1px solid var(--color-border); border-radius: 4px; }
.normalization-block { margin-top: 1rem; padding-top: 1rem; border-top: 1px solid var(--color-border); }
.knockout-block { margin-top: 0.75rem; padding: 0.75rem; background: var(--color-surface-muted, #f8fafc); border-radius: 4px; }
.knockout-toggle { display: flex; align-items: center; gap: 0.5rem; }
.option-score-row { display: flex; justify-content: space-between; align-items: center; gap: 1rem; margin: 0.25rem 0; }
.muted { color: var(--color-text-muted); font-size: 0.875rem; }
</style>
