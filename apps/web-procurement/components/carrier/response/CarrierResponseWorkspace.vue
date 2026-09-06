<script setup lang="ts">
import type { RfxEvent } from '~/types/rfx'
import { formatRfxDate } from '~/types/rfx'
import {
  resolveCarrierAutosaveLabelKey,
  resolveCarrierAutosaveStatusClass,
} from '~/utils/carrierResponseAutosave'
import { formatLocalValueForDisplay } from '~/utils/carrierResponseValidation'

const props = defineProps<{
  event: RfxEvent
  buyerName: string
  carrierCompanyId: string
}>()

const { t, locale } = useI18n()
const { pushToast } = useToast()

const eventId = computed(() => props.event.id)
const carrierCompanyId = computed(() => props.carrierCompanyId)

const {
  workspace,
  loading,
  loadError,
  autosaveStatus,
  lastSavedAt,
  completionPercent,
  productStatus,
  isLocked,
  sectionSummaries,
  globalErrors,
  blockingErrorCount,
  activeSectionId,
  focusQuestionId,
  submitBlockedMessage,
  showLeaveWarning,
  loadWorkspace,
  reloadFromServer,
  setLocalAnswer,
  getLocalAnswer,
  isQuestionVisible,
  isQuestionRequired,
  inlineErrorsForQuestion,
  submitResponse,
  navigateToError,
  clearFocusQuestion,
  confirmLeave,
  discardInvalidAndLeave,
  flushPendingPatches,
} = useCarrierResponseWorkspace(eventId, carrierCompanyId)

const submitting = ref(false)
const starting = ref(false)

const autosaveLabel = computed(() => {
  const key = resolveCarrierAutosaveLabelKey(autosaveStatus.value)
  if (!key) return ''
  if (key === 'carrierResponse.autosave.savedAt' && lastSavedAt.value) {
    const time = new Date(lastSavedAt.value).toLocaleTimeString(locale.value)
    return t(key, { time })
  }
  return t(key)
})

const autosaveClass = computed(() => resolveCarrierAutosaveStatusClass(autosaveStatus.value))

const activeSection = computed(() =>
  workspace.value?.questionnaire.sections.find((swq) => swq.section.id === activeSectionId.value) ?? null,
)

function statusLabel(status: string) {
  const key = `carrierResponse.status.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

function sectionBadge(summary: { state: string; errorCount: number; warningCount: number; incompleteCount: number }) {
  if (summary.state === 'ERROR') return t('carrierResponse.section.error', { count: summary.errorCount })
  if (summary.state === 'INCOMPLETE') return t('carrierResponse.section.incomplete', { count: summary.incompleteCount })
  if (summary.state === 'WARNING') return t('carrierResponse.section.warning', { count: summary.warningCount })
  return t('carrierResponse.section.complete')
}

async function ensureStarted() {
  starting.value = true
  try {
    await loadWorkspace({ startIfMissing: true })
  } finally {
    starting.value = false
  }
}

async function onSubmit() {
  if (!window.confirm(t('carrierResponse.submit.confirm'))) return
  submitting.value = true
  try {
    const ok = await submitResponse()
    if (ok) pushToast('success', t('carrierResponse.submit.success'))
  } finally {
    submitting.value = false
  }
}

async function onConflictReload() {
  await reloadFromServer()
  pushToast('info', t('carrierResponse.autosave.saved'))
}

watch(
  () => focusQuestionId.value,
  async (questionId) => {
    if (!questionId) return
    await nextTick()
    const el = document.querySelector(`[data-question-id="${questionId}"]`) as HTMLElement | null
    el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    const input = el?.querySelector('input, textarea, select') as HTMLElement | null
    input?.focus()
    clearFocusQuestion()
  },
)

onMounted(() => {
  void ensureStarted()
})
</script>

<template>
  <div class="cr-workspace" data-testid="carrier-response-workspace">
    <header class="cr-workspace__header">
      <div>
        <p class="cr-workspace__eyebrow">{{ t('carrierResponse.title') }}</p>
        <h1>{{ event.rfx_number }} · {{ event.title }}</h1>
        <p class="cr-workspace__meta">
          {{ t('carrierResponse.buyer') }}: {{ buyerName }}
          · {{ t('carrierResponse.deadline') }}: {{ formatRfxDate(event.response_deadline) }}
        </p>
      </div>
      <div class="cr-workspace__status-panel">
        <span class="cr-workspace__status-badge">{{ statusLabel(productStatus) }}</span>
        <span class="cr-workspace__completion" data-testid="completion-percent">
          {{ t('carrierResponse.completion', { percent: Math.round(completionPercent) }) }}
        </span>
        <span
          class="cr-workspace__autosave"
          :class="autosaveClass"
          data-testid="autosave-status"
        >
          {{ autosaveLabel }}
        </span>
      </div>
    </header>

    <div v-if="starting || loading" class="cr-workspace__loading">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="loadError" class="cr-workspace__error">
      {{ loadError }}
    </div>

    <template v-else-if="workspace">
      <div v-if="autosaveStatus === 'conflict'" class="cr-workspace__conflict" data-testid="conflict-banner">
        <p>{{ t('carrierResponse.conflict.body') }}</p>
        <button type="button" @click="onConflictReload">{{ t('carrierResponse.conflict.reload') }}</button>
      </div>

      <div class="cr-workspace__layout">
        <nav class="cr-workspace__nav" aria-label="Sections">
          <button
            v-for="summary in sectionSummaries"
            :key="summary.sectionId"
            type="button"
            class="cr-workspace__nav-item"
            :class="{ 'cr-workspace__nav-item--active': activeSectionId === summary.sectionId }"
            :data-testid="`section-nav-${summary.sectionCode}`"
            @click="activeSectionId = summary.sectionId"
          >
            <span>{{ summary.title }}</span>
            <span class="cr-workspace__nav-badge">{{ sectionBadge(summary) }}</span>
          </button>
        </nav>

        <main class="cr-workspace__main">
          <section v-if="activeSection" :data-testid="`section-${activeSection.section.section_code}`">
            <h2>{{ activeSection.section.title }}</h2>
            <p v-if="activeSection.section.description">{{ activeSection.section.description }}</p>

            <CarrierResponseQuestion
              v-for="question in activeSection.questions"
              v-show="isQuestionVisible(question)"
              :key="question.id"
              :question="question"
              :model-value="getLocalAnswer(question.id)"
              :required="isQuestionRequired(question)"
              :disabled="isLocked"
              :errors="inlineErrorsForQuestion(question.id)"
              @update:model-value="setLocalAnswer(question.id, activeSection!.section.id, $event)"
            />
          </section>

          <aside class="cr-workspace__summary" data-testid="global-error-summary">
            <h3>{{ t('carrierResponse.globalSummary.title') }}</h3>
            <p v-if="globalErrors.length === 0" class="muted">
              {{ t('carrierResponse.globalSummary.empty') }}
            </p>
            <ul v-else class="cr-workspace__error-list">
              <li v-for="(item, idx) in globalErrors" :key="idx">
                <div>
                  <strong>{{ item.sectionTitle }} · {{ item.questionLabel }}</strong>
                  <span>{{ t(`carrierResponse.validation.${item.messageKey.replace(/^rfx\\.carrier\\.validation\\./, '')}`, item.params ?? {}) }}</span>
                  <span v-if="item.localValue !== undefined" class="cr-workspace__local-value">
                    ({{ formatLocalValueForDisplay(item.localValue) }})
                  </span>
                </div>
                <button type="button" @click="navigateToError(item)">
                  {{ t('carrierResponse.globalSummary.fix') }}
                </button>
              </li>
            </ul>
          </aside>

          <footer class="cr-workspace__footer">
            <p v-if="submitBlockedMessage" class="cr-workspace__submit-blocked" data-testid="submit-blocked">
              {{ submitBlockedMessage }}
            </p>
            <p v-if="isLocked" class="cr-workspace__locked" data-testid="post-submit-lock">
              {{ t('carrierResponse.submit.locked') }}
            </p>
            <button
              v-else
              type="button"
              data-testid="submit-questionnaire"
              :disabled="submitting || autosaveStatus === 'saving'"
              @click="onSubmit"
            >
              {{ t('carrierResponse.submit.action') }}
            </button>
            <button
              v-if="autosaveStatus === 'save_failed'"
              type="button"
              data-testid="retry-save"
              @click="flushPendingPatches()"
            >
              {{ t('carrierResponse.retrySave') }}
            </button>
          </footer>
        </main>
      </div>
    </template>

    <dialog v-if="showLeaveWarning" open class="cr-workspace__dialog">
      <h3>{{ t('carrierResponse.leaveWarning.title') }}</h3>
      <p>{{ t('carrierResponse.leaveWarning.body', { count: blockingErrorCount }) }}</p>
      <div class="cr-workspace__dialog-actions">
        <button type="button" @click="confirmLeave(true)">
          {{ t('carrierResponse.leaveWarning.stay') }}
        </button>
        <button type="button" @click="discardInvalidAndLeave()">
          {{ t('carrierResponse.leaveWarning.leaveWithoutInvalid') }}
        </button>
      </div>
    </dialog>
  </div>
</template>

<style scoped>
.cr-workspace__header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1.5rem;
}
.cr-workspace__eyebrow {
  text-transform: uppercase;
  font-size: 0.75rem;
  letter-spacing: 0.05em;
  color: var(--color-muted, #666);
}
.cr-workspace__meta {
  color: var(--color-muted, #666);
}
.cr-workspace__status-panel {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.35rem;
}
.cr-workspace__status-badge {
  font-weight: 600;
}
.cr-workspace__autosave.save-status--ok {
  color: var(--color-success, #2d8a4e);
}
.cr-workspace__autosave.save-status--error {
  color: var(--color-danger, #c0392b);
}
.cr-workspace__autosave.save-status--warn {
  color: var(--color-warning, #b8860b);
}
.cr-workspace__layout {
  display: grid;
  grid-template-columns: 220px 1fr;
  gap: 1.5rem;
}
.cr-workspace__nav {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.cr-workspace__nav-item {
  text-align: left;
  padding: 0.75rem;
  border: 1px solid var(--color-border, #ddd);
  border-radius: 4px;
  background: #fff;
  cursor: pointer;
}
.cr-workspace__nav-item--active {
  border-color: var(--color-primary, #2563eb);
  background: var(--color-primary-soft, #eff6ff);
}
.cr-workspace__nav-badge {
  display: block;
  font-size: 0.75rem;
  color: var(--color-muted, #666);
  margin-top: 0.25rem;
}
.cr-workspace__summary {
  margin-top: 2rem;
  padding: 1rem;
  border: 1px solid var(--color-border, #ddd);
  border-radius: 4px;
}
.cr-workspace__error-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.cr-workspace__error-list li {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.5rem 0;
  border-bottom: 1px solid var(--color-border, #eee);
}
.cr-workspace__footer {
  margin-top: 2rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.cr-workspace__conflict {
  padding: 1rem;
  margin-bottom: 1rem;
  border: 1px solid var(--color-warning, #b8860b);
  border-radius: 4px;
  background: #fffbeb;
}
.cr-workspace__dialog {
  border: 1px solid var(--color-border, #ccc);
  border-radius: 8px;
  padding: 1.25rem;
  max-width: 28rem;
}
.cr-workspace__dialog-actions {
  display: flex;
  gap: 0.75rem;
  margin-top: 1rem;
}
.muted {
  color: var(--color-muted, #666);
}
</style>
