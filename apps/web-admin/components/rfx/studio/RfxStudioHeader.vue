<script setup lang="ts">

import type { AutosaveStatus } from '~/types/rfx-questionnaire'
import { resolveAutosaveLabel, resolveAutosaveStatusClass } from '~/utils/rfxStudioQuestionnaire'

const props = defineProps<{

  rfxNumber?: string

  title?: string

  status?: string

  autosaveStatus: AutosaveStatus

  lastSavedAt?: string | null

  fieldError?: string | null

}>()



defineEmits<{ preview: []; save: []; validate: []; reload: [] }>()



const { t } = useI18n()



const saveStatusText = computed(() =>
  resolveAutosaveLabel({
    status: props.autosaveStatus,
    lastSavedAt: props.lastSavedAt,
    fieldError: props.fieldError,
    t,
  }),
)

const statusClass = computed(() => resolveAutosaveStatusClass(props.autosaveStatus))

</script>



<template>

  <header class="studio-header">

    <div>

      <p class="eyebrow">{{ t('rfx.studio.eyebrow') }}</p>

      <h1>{{ rfxNumber }} · {{ title }}</h1>

      <span v-if="status" class="status-badge">{{ status }}</span>

      <span class="save-status" :class="statusClass">{{ saveStatusText }}</span>

      <UiButton

        v-if="autosaveStatus === 'conflict'"

        variant="secondary"

        size="sm"

        class="reload-btn"

        @click="$emit('reload')"

      >

        {{ t('rfx.studio.reloadStudio') }}

      </UiButton>

    </div>

    <div class="actions">

      <UiButton variant="secondary" @click="$emit('preview')">{{ t('rfx.studio.preview') }}</UiButton>

      <UiButton variant="secondary" :disabled="autosaveStatus === 'saving'" @click="$emit('save')">

        {{ t('rfx.studio.saveDraft') }}

      </UiButton>

      <UiButton @click="$emit('validate')">{{ t('rfx.studio.validate') }}</UiButton>

    </div>

  </header>

</template>



<style scoped>

.studio-header {

  display: flex;

  justify-content: space-between;

  align-items: flex-start;

  gap: 1rem;

  margin-bottom: 1.5rem;

  flex-wrap: wrap;

}



.eyebrow {

  font-size: 0.75rem;

  text-transform: uppercase;

  letter-spacing: 0.05em;

  color: var(--color-text-muted, var(--ui-text-muted));

}



.studio-header h1 {

  margin: 0.25rem 0;

  font-size: 1.25rem;

}



.status-badge {

  display: inline-block;

  font-size: 0.75rem;

  padding: 0.125rem 0.5rem;

  border-radius: var(--radius-sm, 4px);

  background: var(--color-bg, #f3f4f6);

  margin-right: 0.5rem;

}



.actions {

  display: flex;

  gap: 0.5rem;

  flex-wrap: wrap;

}



.save-status {

  margin-left: 0.75rem;

  font-size: 0.875rem;

  color: var(--color-text-muted, var(--ui-text-muted));

}



.save-status--ok {

  color: #047857;

}



.save-status--error {

  color: #b91c1c;

}



.save-status--warn {

  color: #b45309;

}



.reload-btn {

  margin-left: 0.5rem;

}

</style>

