<script setup lang="ts">

import type { RfxQuestion, RfxSectionWithQuestions } from '~/types/rfx-questionnaire'

import { reorderByIds } from '~/types/rfx-questionnaire'



const props = defineProps<{

  sectionWithQuestions: RfxSectionWithQuestions

  sectionIndex: number

  sectionsCount: number

  selectedQuestionId: string | null

}>()



defineEmits<{ addQuestion: []; selectQuestion: [question: RfxQuestion] }>()



const api = useInjectedRfxQuestionnaireApi()

const { t } = useI18n()

const { pushToast } = useToast()



const section = computed(() => props.sectionWithQuestions.section)

const questions = computed(() => props.sectionWithQuestions.questions)



const titleDraft = ref(section.value.title)

const descriptionDraft = ref(section.value.description ?? '')



watch(

  () => section.value.title,

  (v) => { titleDraft.value = v },

)

watch(

  () => section.value.description,

  (v) => { descriptionDraft.value = v ?? '' },

)



function onTitleInput() {

  api.scheduleSectionUpdate(section.value.id, { title: titleDraft.value })

}



function onDescriptionInput() {

  api.scheduleSectionUpdate(

    section.value.id,

    { description: descriptionDraft.value || null },

  )

}



async function moveSection(direction: -1 | 1) {

  const studio = api.studio.value

  if (!studio) return

  const ids = studio.sections.map((s) => s.section.id)

  const idx = props.sectionIndex

  const target = idx + direction

  if (target < 0 || target >= ids.length) return

  const swapped = [...ids]

  ;[swapped[idx], swapped[target]] = [swapped[target], swapped[idx]]

  try {

    await api.reorderSections({ ordered_ids: swapped })

  } catch (e) {

    pushToast('error', e instanceof Error ? e.message : t('common.error'))

  }

}



async function moveQuestion(questionIndex: number, direction: -1 | 1) {

  const ordered = reorderByIds(questions.value, questions.value.map((q) => q.id))

  const target = questionIndex + direction

  if (target < 0 || target >= ordered.length) return

  ;[ordered[questionIndex], ordered[target]] = [ordered[target], ordered[questionIndex]]

  try {

    await api.reorderQuestions({

      section_id: section.value.id,

      ordered_ids: ordered.map((q) => q.id),

    })

  } catch (e) {

    pushToast('error', e instanceof Error ? e.message : t('common.error'))

  }

}



async function handleDeleteSection() {

  if (!confirm(t('rfx.studio.confirmDeleteSection'))) return

  try {

    await api.deleteSection(section.value.id, section.value.version)

    pushToast('success', t('rfx.studio.sectionDeleted'))

  } catch (e) {

    pushToast('error', e instanceof Error ? e.message : t('common.error'))

  }

}



async function handleDeleteQuestion(question: RfxQuestion) {

  if (!confirm(t('rfx.studio.confirmDeleteQuestion'))) return

  try {

    await api.deleteQuestion(question.id, question.version)

    pushToast('success', t('rfx.studio.questionDeleted'))

  } catch (e) {

    pushToast('error', e instanceof Error ? e.message : t('common.error'))

  }

}



async function handleDuplicateQuestion(question: RfxQuestion) {

  try {

    await api.duplicateQuestion(question.id)

    pushToast('success', t('rfx.studio.questionDuplicated'))

  } catch (e) {

    pushToast('error', e instanceof Error ? e.message : t('common.error'))

  }

}

</script>



<template>

  <UiCard class="section-card">

    <header class="section-header">

      <div class="section-header__fields">

        <UiInput v-model="titleDraft" :label="t('rfx.studio.sectionTitle')" @input="onTitleInput" />

        <UiInput

          v-model="descriptionDraft"

          :label="t('rfx.studio.sectionDescription')"

          @input="onDescriptionInput"

        />

        <code class="section-code">{{ section.section_code }}</code>

      </div>

      <div class="section-actions">

        <UiButton

          variant="ghost"

          size="sm"

          :disabled="sectionIndex === 0"

          @click="moveSection(-1)"

        >

          {{ t('rfx.studio.moveUp') }}

        </UiButton>

        <UiButton

          variant="ghost"

          size="sm"

          :disabled="sectionIndex >= sectionsCount - 1"

          @click="moveSection(1)"

        >

          {{ t('rfx.studio.moveDown') }}

        </UiButton>

        <UiButton variant="danger" size="sm" @click="handleDeleteSection">

          {{ t('rfx.studio.deleteSection') }}

        </UiButton>

      </div>

    </header>



    <div v-if="questions.length === 0" class="muted">{{ t('rfx.studio.noQuestions') }}</div>



    <div class="questions-list">

      <RfxStudioRfxQuestionCard

        v-for="(question, qIndex) in questions"

        :key="question.id"

        :question="question"

        :selected="selectedQuestionId === question.id"

        @select="$emit('selectQuestion', question)"

        @duplicate="handleDuplicateQuestion(question)"

        @delete="handleDeleteQuestion(question)"

        @move-up="moveQuestion(qIndex, -1)"

        @move-down="moveQuestion(qIndex, 1)"

        :can-move-up="qIndex > 0"

        :can-move-down="qIndex < questions.length - 1"

      />

    </div>



    <UiButton variant="secondary" size="sm" class="add-question-btn" @click="$emit('addQuestion')">

      {{ t('rfx.studio.addQuestion') }}

    </UiButton>

  </UiCard>

</template>



<style scoped>

.section-card {

  padding: 1rem;

}



.section-header {

  display: flex;

  justify-content: space-between;

  gap: 1rem;

  margin-bottom: 0.75rem;

  flex-wrap: wrap;

}



.section-header__fields {

  flex: 1;

  min-width: 200px;

  display: flex;

  flex-direction: column;

  gap: 0.5rem;

}



.section-code {

  font-size: 0.75rem;

  color: var(--color-text-muted);

}



.section-actions {

  display: flex;

  gap: 0.25rem;

  flex-wrap: wrap;

  align-items: flex-start;

}



.questions-list {

  display: flex;

  flex-direction: column;

  gap: 0.5rem;

}



.add-question-btn {

  margin-top: 0.75rem;

}



.muted {

  color: var(--color-text-muted);

  font-size: 0.875rem;

}

</style>

