<script setup lang="ts">

import type { RfxQuestion } from '~/types/rfx-questionnaire'

import { isWave1QuestionType } from '~/types/rfx-questionnaire'



defineProps<{

  question: RfxQuestion

  selected?: boolean

  canMoveUp?: boolean

  canMoveDown?: boolean

}>()



defineEmits<{ select: []; duplicate: []; delete: []; moveUp: []; moveDown: [] }>()



const { t } = useI18n()



function typeLabel(type: string) {

  const key = `rfx.studio.questionTypes.${type}`

  const translated = t(key)

  return translated === key ? type : translated

}

</script>



<template>

  <div

    class="question-card"

    :class="{ 'question-card--selected': selected }"

    role="button"

    tabindex="0"

    @click="$emit('select')"

    @keydown.enter="$emit('select')"

  >

    <div class="question-main">

      <strong>{{ question.label || t('rfx.studio.untitledQuestion') }}</strong>

      <UiBadge status="INFO" tone="neutral">{{ typeLabel(question.question_type) }}</UiBadge>

      <UiBadge v-if="question.required" status="WARN" tone="warning">{{ t('rfx.studio.required') }}</UiBadge>

      <UiBadge

        v-if="!isWave1QuestionType(question.question_type)"

        status="PLANNED"

        tone="neutral"

      >

        {{ t('rfx.studio.comingNextWave') }}

      </UiBadge>

    </div>

    <div class="question-meta">

      <code>{{ question.question_code }}</code>

      <div class="question-actions" @click.stop>

        <UiButton variant="ghost" size="sm" :disabled="!canMoveUp" @click="$emit('moveUp')">

          ↑

        </UiButton>

        <UiButton variant="ghost" size="sm" :disabled="!canMoveDown" @click="$emit('moveDown')">

          ↓

        </UiButton>

        <UiButton variant="ghost" size="sm" @click="$emit('duplicate')">

          {{ t('rfx.studio.duplicate') }}

        </UiButton>

        <UiButton variant="ghost" size="sm" @click="$emit('delete')">

          {{ t('rfx.studio.deleteQuestionBtn') }}

        </UiButton>

      </div>

    </div>

  </div>

</template>



<style scoped>

.question-card {

  border: 1px solid var(--color-border, #e5e7eb);

  border-radius: var(--radius-md, 8px);

  padding: 0.75rem;

  cursor: pointer;

  transition: border-color 0.15s, background 0.15s;

}



.question-card:hover {

  border-color: var(--color-primary);

}



.question-card--selected {

  border-color: var(--color-primary);

  background: rgba(37, 99, 235, 0.04);

}



.question-main {

  display: flex;

  gap: 0.5rem;

  align-items: center;

  flex-wrap: wrap;

}



.question-meta {

  display: flex;

  justify-content: space-between;

  align-items: center;

  margin-top: 0.5rem;

  gap: 0.5rem;

  flex-wrap: wrap;

}



.question-meta code {

  font-size: 0.75rem;

  color: var(--color-text-muted);

}



.question-actions {

  display: flex;

  gap: 0.125rem;

  flex-wrap: wrap;

}

</style>

