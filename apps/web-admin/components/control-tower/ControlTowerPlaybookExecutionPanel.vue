<script setup lang="ts">
import type { PlaybookExecution } from '~/types/automation'

const props = defineProps<{
  execution: PlaybookExecution | null
  loading?: boolean
}>()

const emit = defineEmits<{
  refresh: []
}>()

const { t } = useI18n()
const { startExecution, completeExecutionStep, skipExecutionStep, completeExecution } = useAutomationApi()

async function onStart() {
  if (!props.execution) return
  await startExecution(props.execution.id)
  emit('refresh')
}

async function onCompleteStep(stepId: string) {
  if (!props.execution) return
  await completeExecutionStep(props.execution.id, stepId)
  emit('refresh')
}

async function onSkipStep(stepId: string) {
  if (!props.execution) return
  await skipExecutionStep(props.execution.id, stepId)
  emit('refresh')
}

async function onCompleteExecution() {
  if (!props.execution) return
  await completeExecution(props.execution.id)
  emit('refresh')
}
</script>

<template>
  <section v-if="execution" class="ct-playbook-execution">
    <header>
      <h4>{{ execution.playbookName || execution.playbookId }}</h4>
      <p>{{ $t('controlTower.automation.playbookProgress', { done: execution.progressDone, total: execution.progressTotal }) }}</p>
    </header>
    <ol>
      <li v-for="step in execution.steps" :key="step.id" :data-status="step.status">
        <div>
          <strong>{{ step.sequence }}. {{ step.title }}</strong>
          <span v-if="step.required">{{ $t('controlTower.automation.requiredStep') }}</span>
          <span v-else>{{ $t('controlTower.automation.optionalStep') }}</span>
        </div>
        <div v-if="execution.status !== 'completed' && execution.status !== 'cancelled'" class="ct-playbook-execution__step-actions">
          <UiButton
            v-if="step.status === 'pending' || step.status === 'in_progress'"
            size="sm"
            :disabled="loading"
            @click="onCompleteStep(step.id)"
          >
            {{ $t('controlTower.automation.completeStep') }}
          </UiButton>
          <UiButton
            v-if="!step.required && (step.status === 'pending' || step.status === 'in_progress')"
            size="sm"
            variant="secondary"
            :disabled="loading"
            @click="onSkipStep(step.id)"
          >
            {{ $t('controlTower.automation.skipStep') }}
          </UiButton>
        </div>
      </li>
    </ol>
    <div class="ct-playbook-execution__footer">
      <UiButton v-if="execution.status === 'not_started'" size="sm" :disabled="loading" @click="onStart">
        {{ $t('controlTower.automation.startPlaybook') }}
      </UiButton>
      <UiButton
        v-if="execution.status === 'in_progress'"
        size="sm"
        :disabled="loading"
        @click="onCompleteExecution"
      >
        {{ $t('controlTower.automation.completed') }}
      </UiButton>
    </div>
  </section>
</template>

<style scoped>
.ct-playbook-execution ol {
  list-style: none;
  padding: 0;
}
.ct-playbook-execution li {
  padding: 8px 0;
  border-bottom: 1px solid var(--border-subtle, #eee);
}
.ct-playbook-execution__step-actions {
  display: flex;
  gap: 8px;
  margin-top: 4px;
}
</style>
