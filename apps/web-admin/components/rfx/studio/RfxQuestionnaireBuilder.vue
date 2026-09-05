<script setup lang="ts">
import type { RfxQuestion, RfxSectionWithQuestions } from '~/types/rfx-questionnaire'
import { isWave1QuestionType, nextSortOrder } from '~/types/rfx-questionnaire'

const props = defineProps<{
  sections: RfxSectionWithQuestions[]
}>()

const selectedQuestionId = defineModel<string | null>('selectedQuestionId', { default: null })

const api = useInjectedRfxQuestionnaireApi()
const { t } = useI18n()
const { pushToast } = useToast()

const selectedQuestion = computed(() => {
  if (!selectedQuestionId.value) return null
  for (const swq of props.sections) {
    const q = swq.questions.find((item) => item.id === selectedQuestionId.value)
    if (q) return q
  }
  return null
})

async function handleAddSection() {
  const sortOrder = nextSortOrder(props.sections.map((s) => s.section))
  const code = `SEC_${sortOrder}`
  try {
    await api.createSection({ section_code: code, title: t('rfx.studio.newSection'), sort_order: sortOrder })
    pushToast('success', t('rfx.studio.sectionCreated'))
  } catch (e) {
    pushToast('error', e instanceof Error ? e.message : t('common.error'))
  }
}

async function handleAddQuestion(sectionId: string) {
  const swq = props.sections.find((s) => s.section.id === sectionId)
  const sortOrder = nextSortOrder(swq?.questions ?? [])
  const code = `Q_${sortOrder}`
  try {
    const question = await api.createQuestion({
      section_id: sectionId,
      question_code: code,
      question_type: 'TEXT',
      label: t('rfx.studio.newQuestion'),
      sort_order: sortOrder,
    })
    selectedQuestionId.value = question.id
  } catch (e) {
    pushToast('error', e instanceof Error ? e.message : t('common.error'))
  }
}

function selectQuestion(question: RfxQuestion) {
  selectedQuestionId.value = question.id
}
</script>

<template>
  <div class="builder-layout">
    <div class="builder-main">
      <div class="builder-toolbar">
        <UiButton @click="handleAddSection">{{ t('rfx.studio.addSection') }}</UiButton>
      </div>

      <p v-if="sections.length === 0" class="muted">{{ t('rfx.studio.noSections') }}</p>

      <RfxSectionCard
        v-for="(swq, sectionIndex) in sections"
        :key="swq.section.id"
        :section-with-questions="swq"
        :section-index="sectionIndex"
        :sections-count="sections.length"
        :selected-question-id="selectedQuestionId"
        @add-question="handleAddQuestion(swq.section.id)"
        @select-question="selectQuestion"
      />
    </div>

    <aside v-if="selectedQuestion" class="builder-panel">
      <QuestionPropertyPanel :question="selectedQuestion" />
      <QuestionOptionsEditor
        v-if="selectedQuestion.question_type === 'SINGLE_SELECT' || selectedQuestion.question_type === 'MULTI_SELECT'"
        :question="selectedQuestion"
      />
      <ConditionalRuleEditor
        v-if="isWave1QuestionType(selectedQuestion.question_type)"
        :target-question="selectedQuestion"
      />
    </aside>
    <aside v-else class="builder-panel builder-panel--empty">
      <p class="muted">{{ t('rfx.studio.selectQuestionHint') }}</p>
    </aside>
  </div>
</template>

<style scoped>
.builder-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 1rem;
  align-items: start;
}

.builder-toolbar {
  margin-bottom: 1rem;
}

.builder-main {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-width: 0;
}

.builder-panel {
  position: sticky;
  top: 1rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-height: calc(100vh - 2rem);
  overflow-y: auto;
}

.builder-panel--empty {
  padding: 1rem;
  border: 1px dashed var(--color-border);
  border-radius: var(--radius-lg);
}

.muted {
  color: var(--color-text-muted);
}

@media (max-width: 1100px) {
  .builder-layout {
    grid-template-columns: 1fr;
  }

  .builder-panel {
    position: static;
    max-height: none;
  }
}
</style>
