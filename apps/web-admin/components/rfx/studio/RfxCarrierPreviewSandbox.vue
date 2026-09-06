<script setup lang="ts">
import type { RfxQuestion, RfxQuestionRule, RfxSectionWithQuestions } from '~/types/rfx-questionnaire'
import { isWave1QuestionType } from '~/types/rfx-questionnaire'
import {
  computePreviewCompletionPercent,
  computePreviewSectionSummaries,
  resolvePreviewQuestionRequired,
  resolvePreviewQuestionVisibility,
  validatePreviewAnswers,
  type PreviewSandboxValidationError,
} from '~/utils/rfxPreviewSandbox'

const props = defineProps<{
  eventTitle: string
  rfxNumber: string
  sections: RfxSectionWithQuestions[]
  rules: RfxQuestionRule[]
}>()

const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()

const localValues = reactive(new Map<string, unknown>())
const fieldErrors = ref<PreviewSandboxValidationError[]>([])
const submitAttempted = ref(false)
const submitSuccess = ref(false)
const focusQuestionId = ref<string | null>(null)
const activeSectionId = ref<string | null>(props.sections[0]?.section.id ?? null)

const completionPercent = computed(() =>
  computePreviewCompletionPercent({
    sections: props.sections,
    rules: props.rules,
    localValues,
  }),
)

const sectionSummaries = computed(() =>
  computePreviewSectionSummaries({
    sections: props.sections,
    rules: props.rules,
    localValues,
    fieldErrors: fieldErrors.value,
  }),
)

const activeSection = computed(() =>
  props.sections.find((swq) => swq.section.id === activeSectionId.value) ?? null,
)

function answersByCode() {
  const out: Record<string, unknown> = {}
  for (const swq of props.sections) {
    for (const q of swq.questions) {
      if (localValues.has(q.id)) out[q.question_code] = localValues.get(q.id)
    }
  }
  return out
}

function isVisible(question: RfxQuestion): boolean {
  return resolvePreviewQuestionVisibility(question, props.sections, props.rules, answersByCode())
}

function isRequired(question: RfxQuestion): boolean {
  return resolvePreviewQuestionRequired(question, props.sections, props.rules, answersByCode())
}

function inlineErrors(questionId: string) {
  return fieldErrors.value.filter((e) => e.questionId === questionId)
}

function setAnswer(questionId: string, value: unknown) {
  submitSuccess.value = false
  localValues.set(questionId, value)
  if (submitAttempted.value) {
    fieldErrors.value = validatePreviewAnswers({
      sections: props.sections,
      rules: props.rules,
      localValues,
    })
  }
}

function resetAnswers() {
  localValues.clear()
  fieldErrors.value = []
  submitAttempted.value = false
  submitSuccess.value = false
}

function runSimulatedSubmit() {
  submitAttempted.value = true
  fieldErrors.value = validatePreviewAnswers({
    sections: props.sections,
    rules: props.rules,
    localValues,
  })
  if (fieldErrors.value.length > 0) {
    submitSuccess.value = false
    return
  }
  submitSuccess.value = true
}

function navigateToError(item: PreviewSandboxValidationError) {
  activeSectionId.value = item.sectionId
  focusQuestionId.value = item.questionId
}

watch(focusQuestionId, async (questionId) => {
  if (!questionId) return
  await nextTick()
  const el = document.querySelector(`[data-preview-question-id="${questionId}"]`) as HTMLElement | null
  el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  const input = el?.querySelector('input, textarea, select') as HTMLElement | null
  input?.focus()
  focusQuestionId.value = null
})
</script>

<template>
  <div class="preview-sandbox" data-testid="carrier-preview-sandbox">
    <header class="preview-sandbox__header">
      <div>
        <p class="preview-sandbox__eyebrow">{{ t('rfx.studio.previewSandbox.modeBanner') }}</p>
        <h1>{{ rfxNumber }} · {{ eventTitle }}</h1>
        <p class="preview-sandbox__note">{{ t('rfx.studio.previewSandbox.modeHint') }}</p>
      </div>
      <div class="preview-sandbox__actions">
        <span data-testid="preview-completion">{{ t('rfx.studio.previewSandbox.simulatedProgress', { percent: completionPercent }) }}</span>
        <button type="button" data-testid="preview-reset" @click="resetAnswers">{{ t('rfx.studio.previewSandbox.reset') }}</button>
        <button type="button" data-testid="preview-close" @click="emit('close')">{{ t('rfx.studio.previewSandbox.close') }}</button>
      </div>
    </header>

    <div class="preview-sandbox__layout">
      <nav class="preview-sandbox__nav">
        <button
          v-for="summary in sectionSummaries"
          :key="summary.sectionId"
          type="button"
          :data-testid="`preview-section-nav-${summary.sectionCode}`"
          :class="{ active: activeSectionId === summary.sectionId }"
          @click="activeSectionId = summary.sectionId"
        >
          {{ summary.title }}
          <span v-if="summary.errorCount">{{ t('rfx.studio.previewSandbox.section.error', { count: summary.errorCount }) }}</span>
          <span v-else-if="summary.incompleteCount">{{ t('rfx.studio.previewSandbox.section.incomplete', { count: summary.incompleteCount }) }}</span>
        </button>
      </nav>

      <main v-if="activeSection" :data-testid="`preview-section-${activeSection.section.section_code}`">
        <h2>{{ activeSection.section.title }}</h2>
        <div
          v-for="question in activeSection.questions"
          v-show="isVisible(question)"
          :key="question.id"
          class="preview-sandbox__question"
          :data-testid="`preview-question-${question.question_code}`"
          :data-preview-question-id="question.id"
        >
          <label>
            {{ question.label }}
            <span v-if="isRequired(question)" class="req">*</span>
          </label>
          <p v-if="question.help_text" class="help">{{ question.help_text }}</p>

          <div v-if="!isWave1QuestionType(question.question_type)" data-testid="preview-unsupported">
            <p>{{ t('rfx.studio.previewSandbox.unsupportedType', { type: question.question_type }) }}</p>
            <p v-if="isRequired(question)" class="err">{{ t('rfx.studio.previewSandbox.unsupportedRequired') }}</p>
          </div>

          <template v-else>
            <input
              v-if="question.question_type === 'TEXT'"
              type="text"
              class="ctrl"
              :value="(localValues.get(question.id) as string) ?? ''"
              @input="setAnswer(question.id, ($event.target as HTMLInputElement).value)"
            >
            <textarea
              v-else-if="question.question_type === 'LONG_TEXT'"
              class="ctrl"
              rows="3"
              :value="(localValues.get(question.id) as string) ?? ''"
              @input="setAnswer(question.id, ($event.target as HTMLTextAreaElement).value)"
            />
            <input
              v-else-if="question.question_type === 'NUMBER'"
              type="number"
              class="ctrl"
              :value="localValues.get(question.id) === undefined ? '' : String(localValues.get(question.id))"
              @input="setAnswer(question.id, ($event.target as HTMLInputElement).value === '' ? null : Number(($event.target as HTMLInputElement).value))"
            >
            <div v-else-if="question.question_type === 'YES_NO'" class="yesno">
              <label>
                <input
                  type="radio"
                  :name="`yesno-${question.id}`"
                  :checked="localValues.get(question.id) === true"
                  @change="setAnswer(question.id, true)"
                >
                {{ t('rfx.studio.yes') }}
              </label>
              <label>
                <input
                  type="radio"
                  :name="`yesno-${question.id}`"
                  :checked="localValues.get(question.id) === false"
                  @change="setAnswer(question.id, false)"
                >
                {{ t('rfx.studio.no') }}
              </label>
            </div>
            <select
              v-else-if="question.question_type === 'SINGLE_SELECT'"
              class="ctrl"
              :value="(localValues.get(question.id) as string) ?? ''"
              @change="setAnswer(question.id, ($event.target as HTMLSelectElement).value || null)"
            >
              <option value="">—</option>
              <option v-for="opt in question.options ?? []" :key="opt.id" :value="opt.option_code">{{ opt.label }}</option>
            </select>
            <div v-else-if="question.question_type === 'MULTI_SELECT'" class="yesno">
              <label v-for="opt in question.options ?? []" :key="opt.id">
                <input
                  type="checkbox"
                  :checked="Array.isArray(localValues.get(question.id)) && (localValues.get(question.id) as string[]).includes(opt.option_code)"
                  @change="(e) => {
                    const cur = Array.isArray(localValues.get(question.id)) ? [...(localValues.get(question.id) as string[])] : []
                    if ((e.target as HTMLInputElement).checked) { if (!cur.includes(opt.option_code)) cur.push(opt.option_code) }
                    else { const i = cur.indexOf(opt.option_code); if (i >= 0) cur.splice(i, 1) }
                    setAnswer(question.id, cur)
                  }"
                >
                {{ opt.label }}
              </label>
            </div>
            <input
              v-else-if="question.question_type === 'DATE'"
              type="date"
              class="ctrl"
              :value="(localValues.get(question.id) as string) ?? ''"
              @input="setAnswer(question.id, ($event.target as HTMLInputElement).value)"
            >
          </template>

          <ul v-if="inlineErrors(question.id).length" class="errs" data-testid="preview-inline-errors">
            <li v-for="(err, idx) in inlineErrors(question.id)" :key="idx">
              {{ t(`rfx.studio.previewSandbox.validation.${err.messageKey}`, err.params ?? {}) }}
            </li>
          </ul>
        </div>
      </main>
    </div>

    <aside class="preview-sandbox__summary" data-testid="preview-global-summary">
      <h3>{{ t('rfx.studio.previewSandbox.globalSummary.title') }}</h3>
      <p v-if="fieldErrors.length === 0">{{ t('rfx.studio.previewSandbox.globalSummary.empty') }}</p>
      <ul v-else>
        <li v-for="(item, idx) in fieldErrors" :key="idx">
          <span>{{ item.questionCode }}: {{ t(`rfx.studio.previewSandbox.validation.${item.messageKey}`, item.params ?? {}) }}</span>
          <button type="button" @click="navigateToError(item)">{{ t('rfx.studio.previewSandbox.globalSummary.fix') }}</button>
        </li>
      </ul>
    </aside>

    <footer class="preview-sandbox__footer">
      <p v-if="submitAttempted && fieldErrors.length" data-testid="preview-submit-blocked">
        {{ t('rfx.studio.previewSandbox.submitBlocked', { count: fieldErrors.length }) }}
      </p>
      <p v-if="submitSuccess" data-testid="preview-submit-success">{{ t('rfx.studio.previewSandbox.submitSuccess') }}</p>
      <button type="button" data-testid="preview-check-submit" @click="runSimulatedSubmit">
        {{ t('rfx.studio.previewSandbox.checkSubmit') }}
      </button>
    </footer>
  </div>
</template>

<style scoped>
.preview-sandbox { max-width: 960px; }
.preview-sandbox__header { display: flex; justify-content: space-between; gap: 1rem; margin-bottom: 1rem; }
.preview-sandbox__eyebrow { font-size: 0.75rem; text-transform: uppercase; color: var(--color-text-muted); }
.preview-sandbox__note { font-size: 0.875rem; color: var(--color-text-muted); }
.preview-sandbox__actions { display: flex; flex-direction: column; align-items: flex-end; gap: 0.5rem; }
.preview-sandbox__layout { display: grid; grid-template-columns: 200px 1fr; gap: 1rem; }
.preview-sandbox__nav { display: flex; flex-direction: column; gap: 0.5rem; }
.preview-sandbox__nav button { text-align: left; padding: 0.5rem; border: 1px solid var(--color-border); background: #fff; cursor: pointer; }
.preview-sandbox__nav button.active { border-color: var(--color-primary); }
.preview-sandbox__question { margin-bottom: 1rem; }
.req { color: #b45309; }
.help { font-size: 0.8125rem; color: var(--color-text-muted); }
.ctrl { width: 100%; max-width: 28rem; padding: 0.5rem; border: 1px solid var(--color-border); border-radius: 4px; }
.yesno { display: flex; gap: 1rem; flex-wrap: wrap; }
.errs { color: #c0392b; font-size: 0.875rem; }
.preview-sandbox__summary { margin-top: 1.5rem; padding: 1rem; border: 1px solid var(--color-border); }
.preview-sandbox__footer { margin-top: 1rem; }
</style>
