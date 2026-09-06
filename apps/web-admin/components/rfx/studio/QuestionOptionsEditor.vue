<script setup lang="ts">

import type { RfxQuestion } from '~/types/rfx-questionnaire'

import { OPTION_REORDER_UI, nextSortOrder } from '~/types/rfx-questionnaire'



const props = defineProps<{ question: RfxQuestion }>()



const api = useInjectedRfxQuestionnaireApi()

const { t } = useI18n()

const { pushToast } = useToast()



const options = computed(() => props.question.options ?? [])



const labelDrafts = ref<Record<string, string>>({})



watch(

  () => props.question.options,

  (opts) => {

    const next: Record<string, string> = {}

    for (const opt of opts ?? []) next[opt.id] = opt.label

    labelDrafts.value = next

  },

  { immediate: true, deep: true },

)



function onLabelInput(optionId: string) {

  api.scheduleOptionUpdate(

    props.question.id,

    optionId,

    { label: labelDrafts.value[optionId] ?? '' },

  )

}



async function addOption() {

  const sortOrder = nextSortOrder(options.value)

  const code = `OPT_${sortOrder}`

  try {

    await api.createOption(props.question.id, {

      option_code: code,

      label: t('rfx.studio.newOption'),

      sort_order: sortOrder,

    })

  } catch (e) {

    pushToast('error', e instanceof Error ? e.message : t('common.error'))

  }

}



async function removeOption(optionId: string, version: number) {

  if (!confirm(t('rfx.studio.confirmDeleteOption'))) return

  try {

    await api.deleteOption(props.question.id, optionId, version)

  } catch (e) {

    pushToast('error', e instanceof Error ? e.message : t('common.error'))

  }

}

</script>



<template>

  <UiCard class="options-editor">

    <header class="options-header">

      <h3>{{ t('rfx.studio.optionsTitle') }}</h3>

      <UiBadge status="PLANNED" tone="neutral">{{ t('rfx.studio.optionReorderUnavailable') }}</UiBadge>

    </header>



    <p class="options-note">{{ t('rfx.studio.optionReorderNote', { flag: OPTION_REORDER_UI }) }}</p>



    <div v-if="options.length === 0" class="muted">{{ t('rfx.studio.noOptions') }}</div>



    <div v-for="opt in options" :key="opt.id" class="option-row">

      <UiInput

        v-model="labelDrafts[opt.id]"

        :label="opt.option_code"

        @input="onLabelInput(opt.id)"

      />

      <UiButton variant="ghost" size="sm" @click="removeOption(opt.id, opt.version)">

        {{ t('rfx.studio.deleteOption') }}

      </UiButton>

    </div>



    <UiButton variant="secondary" size="sm" @click="addOption">{{ t('rfx.studio.addOption') }}</UiButton>

  </UiCard>

</template>



<style scoped>

.options-editor {

  padding: 1rem;

  display: flex;

  flex-direction: column;

  gap: 0.75rem;

}



.options-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  gap: 0.5rem;

  flex-wrap: wrap;

}



.options-header h3 {

  margin: 0;

  font-size: 1rem;

}



.options-note {

  font-size: 0.8125rem;

  color: var(--color-text-muted);

  margin: 0;

}



.option-row {

  display: flex;

  flex-direction: column;

  gap: 0.25rem;

  padding-bottom: 0.5rem;

  border-bottom: 1px solid var(--color-border);

}



.muted {

  color: var(--color-text-muted);

  font-size: 0.875rem;

}

</style>

