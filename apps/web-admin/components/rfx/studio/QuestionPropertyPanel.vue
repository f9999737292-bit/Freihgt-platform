<script setup lang="ts">

import type { RfxQuestion, RfxQuestionType } from '~/types/rfx-questionnaire'

import { RFX_QUESTION_TYPES, isWave1QuestionType } from '~/types/rfx-questionnaire'



const props = defineProps<{ question: RfxQuestion }>()



const api = useInjectedRfxQuestionnaireApi()

const { t } = useI18n()



const labelDraft = ref(props.question.label)

const helpTextDraft = ref(props.question.help_text ?? '')

const requiredDraft = ref(props.question.required)

const typeDraft = ref(props.question.question_type)



watch(

  () => props.question,

  (q) => {

    labelDraft.value = q.label

    helpTextDraft.value = q.help_text ?? ''

    requiredDraft.value = q.required

    typeDraft.value = q.question_type

  },

  { deep: true },

)



const typeOptions = computed(() =>

  RFX_QUESTION_TYPES.map((type) => ({

    value: type,

    label: t(`rfx.studio.questionTypes.${type}`),

  })),

)



function patch(fields: Partial<{ label: string; help_text: string | null; required: boolean; question_type: RfxQuestionType }>) {

  api.scheduleQuestionUpdate(props.question.id, fields)

}



function onLabelInput() {

  patch({ label: labelDraft.value })

}



function onHelpTextInput() {

  patch({ help_text: helpTextDraft.value || null })

}



function onRequiredChange() {

  patch({ required: requiredDraft.value })

}



function onTypeChange(value: string) {

  typeDraft.value = value as RfxQuestionType

  patch({ question_type: typeDraft.value })

}



const wave1Editable = computed(() => isWave1QuestionType(props.question.question_type))

</script>



<template>

  <UiCard class="property-panel">

    <h3>{{ t('rfx.studio.propertiesTitle') }}</h3>



    <UiInput

      v-model="labelDraft"

      :label="t('rfx.studio.questionLabel')"

      :disabled="!wave1Editable"

      @input="onLabelInput"

    />



    <UiInput

      v-model="helpTextDraft"

      :label="t('rfx.studio.helpText')"

      :disabled="!wave1Editable"

      @input="onHelpTextInput"

    />



    <label class="checkbox-row">

      <input v-model="requiredDraft" type="checkbox" :disabled="!wave1Editable" @change="onRequiredChange">

      <span>{{ t('rfx.studio.requiredField') }}</span>

    </label>



    <UiSelect

      :model-value="typeDraft"

      :label="t('rfx.studio.questionType')"

      :options="typeOptions"

      :disabled="!wave1Editable"

      @update:model-value="onTypeChange"

    />



    <p v-if="!wave1Editable" class="coming-next-note">

      <UiBadge status="PLANNED" tone="neutral">{{ t('rfx.studio.comingNextWave') }}</UiBadge>

      {{ t('rfx.studio.wave1OnlyEdit') }}

    </p>



    <div class="readonly-meta">

      <span>{{ t('rfx.studio.questionCode') }}: <code>{{ question.question_code }}</code></span>

    </div>

  </UiCard>

</template>



<style scoped>

.property-panel {

  padding: 1rem;

  display: flex;

  flex-direction: column;

  gap: 0.75rem;

}



.property-panel h3 {

  margin: 0;

  font-size: 1rem;

}



.checkbox-row {

  display: flex;

  align-items: center;

  gap: 0.5rem;

  font-size: 0.875rem;

}



.coming-next-note {

  font-size: 0.8125rem;

  color: var(--color-text-muted);

  display: flex;

  align-items: center;

  gap: 0.5rem;

  flex-wrap: wrap;

}



.readonly-meta {

  font-size: 0.8125rem;

  color: var(--color-text-muted);

}



.readonly-meta code {

  font-size: 0.75rem;

}

</style>

