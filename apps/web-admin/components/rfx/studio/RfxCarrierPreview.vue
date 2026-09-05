<script setup lang="ts">

import type { RfxSectionWithQuestions } from '~/types/rfx-questionnaire'
import {
  isPreviewRenderableQuestionType,
  previewShowsRequiredMark,
  resolvePreviewQuestionTypeLabel,
} from '~/utils/rfxStudioQuestionnaire'



defineProps<{

  eventTitle: string

  rfxNumber: string

  sections: RfxSectionWithQuestions[]

}>()



const { t } = useI18n()



function typeLabel(type: string) {
  return resolvePreviewQuestionTypeLabel(type, t)
}

</script>



<template>

  <div class="carrier-preview">

    <header class="preview-header">

      <p class="eyebrow">{{ t('rfx.studio.previewEyebrow') }}</p>

      <h1>{{ rfxNumber }} · {{ eventTitle }}</h1>

      <p class="preview-note">{{ t('rfx.studio.previewReadOnly') }}</p>

    </header>



    <p v-if="sections.length === 0" class="muted">{{ t('rfx.studio.noSections') }}</p>



    <UiCard v-for="swq in sections" :key="swq.section.id" class="preview-section">

      <h2>{{ swq.section.title }}</h2>

      <p v-if="swq.section.description" class="section-desc">{{ swq.section.description }}</p>



      <div v-for="question in swq.questions" :key="question.id" class="preview-question">

        <label class="preview-label">

          <span>{{ question.label }}</span>

          <span v-if="previewShowsRequiredMark(question.required)" class="required-mark">*</span>

        </label>

        <p v-if="question.help_text" class="help-text">{{ question.help_text }}</p>



        <template v-if="isPreviewRenderableQuestionType(question.question_type)">

          <input

            v-if="question.question_type === 'TEXT'"

            type="text"

            class="preview-control"

            disabled

            :placeholder="t('rfx.studio.previewPlaceholder')"

          >

          <textarea

            v-else-if="question.question_type === 'LONG_TEXT'"

            class="preview-control preview-control--textarea"

            disabled

            :placeholder="t('rfx.studio.previewPlaceholder')"

          />

          <input

            v-else-if="question.question_type === 'NUMBER'"

            type="number"

            class="preview-control"

            disabled

          >

          <select v-else-if="question.question_type === 'YES_NO'" class="preview-control" disabled>

            <option value="">{{ t('rfx.studio.previewSelect') }}</option>

            <option>{{ t('rfx.studio.yes') }}</option>

            <option>{{ t('rfx.studio.no') }}</option>

          </select>

          <select

            v-else-if="question.question_type === 'SINGLE_SELECT'"

            class="preview-control"

            disabled

          >

            <option value="">{{ t('rfx.studio.previewSelect') }}</option>

            <option v-for="opt in question.options ?? []" :key="opt.id" :value="opt.option_code">

              {{ opt.label }}

            </option>

          </select>

          <div v-else-if="question.question_type === 'MULTI_SELECT'" class="preview-options">

            <label v-for="opt in question.options ?? []" :key="opt.id" class="checkbox-row">

              <input type="checkbox" disabled>

              <span>{{ opt.label }}</span>

            </label>

          </div>

          <input

            v-else-if="question.question_type === 'DATE'"

            type="date"

            class="preview-control"

            disabled

          >

        </template>

        <div v-else class="coming-next">

          <UiBadge status="PLANNED" tone="neutral">{{ t('rfx.studio.comingNextWave') }}</UiBadge>

          <span>{{ typeLabel(question.question_type) }}</span>

        </div>

      </div>

    </UiCard>

  </div>

</template>



<style scoped>

.carrier-preview {

  max-width: 720px;

}



.preview-header {

  margin-bottom: 1.5rem;

}



.eyebrow {

  font-size: 0.75rem;

  text-transform: uppercase;

  letter-spacing: 0.05em;

  color: var(--color-text-muted);

}



.preview-header h1 {

  margin: 0.25rem 0;

  font-size: 1.25rem;

}



.preview-note {

  font-size: 0.875rem;

  color: var(--color-text-muted);

}



.preview-section {

  padding: 1rem;

  margin-bottom: 1rem;

}



.preview-section h2 {

  margin: 0 0 0.5rem;

  font-size: 1.0625rem;

}



.section-desc {

  font-size: 0.875rem;

  color: var(--color-text-muted);

  margin: 0 0 1rem;

}



.preview-question {

  margin-bottom: 1rem;

  padding-bottom: 1rem;

  border-bottom: 1px solid var(--color-border);

}



.preview-question:last-child {

  border-bottom: none;

  margin-bottom: 0;

  padding-bottom: 0;

}



.preview-label {

  display: block;

  font-weight: 500;

  margin-bottom: 0.25rem;

}



.required-mark {

  color: #b45309;

  margin-left: 0.25rem;

}



.help-text {

  font-size: 0.8125rem;

  color: var(--color-text-muted);

  margin: 0 0 0.5rem;

}



.preview-control {

  width: 100%;

  min-height: 38px;

  padding: 0.5rem 0.75rem;

  border: 1px solid var(--color-border);

  border-radius: var(--radius-md);

  background: var(--color-bg);

  font: inherit;

}



.preview-control--textarea {

  min-height: 80px;

  resize: vertical;

}



.preview-options {

  display: flex;

  flex-direction: column;

  gap: 0.375rem;

}



.checkbox-row {

  display: flex;

  align-items: center;

  gap: 0.5rem;

  font-size: 0.875rem;

}



.coming-next {

  display: flex;

  align-items: center;

  gap: 0.5rem;

  font-size: 0.875rem;

  color: var(--color-text-muted);

}



.muted {

  color: var(--color-text-muted);

}

</style>

