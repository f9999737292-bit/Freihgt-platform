<script setup lang="ts">
import {
  CONTROL_TOWER_MITIGATION_CODES,
  type ControlTowerMitigationCode,
  type ControlTowerShipmentRisk,
} from '~/types/controlTower'

const props = defineProps<{
  open: boolean
  risk: ControlTowerShipmentRisk | null
  loading?: boolean
}>()

const emit = defineEmits<{
  close: []
  submit: [code: ControlTowerMitigationCode, comment?: string]
}>()

const { t } = useI18n()

const mitigationCode = ref<ControlTowerMitigationCode>('monitor')
const comment = ref('')

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      mitigationCode.value = 'monitor'
      comment.value = ''
    }
  },
)

const mitigationOptions = computed(() =>
  CONTROL_TOWER_MITIGATION_CODES.map((value) => ({
    value,
    label: t(`controlTower.risk.mitigationCodes.${value}`),
  })),
)

function onSubmit() {
  emit('submit', mitigationCode.value, comment.value.trim() || undefined)
}
</script>

<template>
  <UiModal :open="open" :title="$t('controlTower.risk.mitigateTitle')" @close="emit('close')">
    <p v-if="risk" class="risk-mitigate__subtitle">
      {{ risk.shipmentNumber }} ·
      {{ $t(`controlTower.risk.types.${risk.predictedExceptionType}`) }}
    </p>

    <label class="risk-mitigate__field">
      <span>{{ $t('controlTower.risk.mitigationCode') }}</span>
      <select v-model="mitigationCode" class="risk-mitigate__select">
        <option v-for="option in mitigationOptions" :key="option.value" :value="option.value">
          {{ option.label }}
        </option>
      </select>
    </label>

    <label class="risk-mitigate__field">
      <span>{{ $t('controlTower.risk.comment') }}</span>
      <textarea v-model="comment" rows="3" class="risk-mitigate__textarea" />
    </label>

    <template #footer>
      <button type="button" class="risk-mitigate__btn" @click="emit('close')">
        {{ $t('common.cancel') }}
      </button>
      <button
        type="button"
        class="risk-mitigate__btn risk-mitigate__btn--primary"
        :disabled="loading"
        @click="onSubmit"
      >
        {{ $t('controlTower.risk.startMitigation') }}
      </button>
    </template>
  </UiModal>
</template>

<style scoped>
.risk-mitigate__subtitle {
  margin: 0 0 1rem;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.risk-mitigate__field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  margin-bottom: 1rem;
  font-size: 0.875rem;
}

.risk-mitigate__select,
.risk-mitigate__textarea {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  padding: 0.5rem 0.625rem;
  font: inherit;
}

.risk-mitigate__btn {
  border: 1px solid var(--color-border);
  background: white;
  border-radius: var(--radius-sm);
  padding: 0.5rem 0.875rem;
  cursor: pointer;
}

.risk-mitigate__btn--primary {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.risk-mitigate__btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
