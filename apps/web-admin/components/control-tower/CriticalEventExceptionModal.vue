<script setup lang="ts">
import type { ControlTowerEvent, ControlTowerExceptionPriority } from '~/types/controlTower'
import {
  CONTROL_TOWER_BUSINESS_IMPACTS,
  CONTROL_TOWER_EXCEPTION_CATEGORIES,
  CONTROL_TOWER_PRIORITIES,
} from '~/types/controlTower'

const props = defineProps<{
  open: boolean
  event: ControlTowerEvent | null
  loading?: boolean
}>()

const emit = defineEmits<{
  close: []
  submit: [payload: { priority?: string; category?: string; businessImpact?: string }]
}>()

const { t } = useI18n()

const priority = ref('')
const category = ref('')
const businessImpact = ref('')

watch(
  () => props.event,
  (event) => {
    priority.value = event?.priority ?? 'p3'
    category.value = event?.exceptionCategory ?? 'other'
    businessImpact.value = event?.businessImpact ?? 'none'
  },
  { immediate: true },
)

const priorityOptions = computed(() =>
  CONTROL_TOWER_PRIORITIES.map((value) => ({
    value,
    label: t(`controlTower.exceptions.priorities.${value}`),
  })),
)

const categoryOptions = computed(() =>
  CONTROL_TOWER_EXCEPTION_CATEGORIES.map((value) => ({
    value,
    label: t(`controlTower.exceptions.categories.${value}`),
  })),
)

const impactOptions = computed(() =>
  CONTROL_TOWER_BUSINESS_IMPACTS.map((value) => ({
    value,
    label: t(`controlTower.exceptions.businessImpact.${value}`),
  })),
)

function onSubmit() {
  emit('submit', {
    priority: priority.value as ControlTowerExceptionPriority,
    category: category.value,
    businessImpact: businessImpact.value,
  })
}
</script>

<template>
  <UiModal
    :open="open"
    :title="$t('controlTower.exceptions.editTitle')"
    @close="$emit('close')"
  >
    <div v-if="event" class="exception-form">
      <p class="exception-form__meta">
        <strong>{{ event.shipmentNumber }}</strong>
        ·
        {{ $t(`controlTower.events.types.${event.type}`) }}
      </p>
      <UiSelect
        v-model="priority"
        :label="$t('controlTower.exceptions.priority')"
        :options="priorityOptions"
      />
      <UiSelect
        v-model="category"
        :label="$t('controlTower.exceptions.category')"
        :options="categoryOptions"
      />
      <UiSelect
        v-model="businessImpact"
        :label="$t('controlTower.exceptions.businessImpactLabel')"
        :options="impactOptions"
      />
      <div class="exception-form__actions">
        <UiButton variant="secondary" @click="$emit('close')">
          {{ $t('common.cancel') }}
        </UiButton>
        <UiButton variant="primary" :loading="loading" @click="onSubmit">
          {{ $t('common.save') }}
        </UiButton>
      </div>
    </div>
  </UiModal>
</template>

<style scoped>
.exception-form {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.exception-form__meta {
  margin: 0;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.exception-form__actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  margin-top: 0.5rem;
}
</style>
