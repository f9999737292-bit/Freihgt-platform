<script setup lang="ts">
const props = defineProps<{ open: boolean; shipmentId: string }>()
const emit = defineEmits<{ close: []; cancelled: [] }>()

const { cancelShipment } = useShipmentsApi()
const { pushToast } = useToast()
const { t } = useI18n()

const reason = ref('Отмена тестовой перевозки')
const saving = ref(false)
const errorMessage = ref('')

watch(
  () => props.open,
  (open) => {
    if (!open) return
    reason.value = 'Отмена тестовой перевозки'
    errorMessage.value = ''
  },
)

async function submit() {
  if (!reason.value.trim()) {
    errorMessage.value = t('rfx.validation.required')
    return
  }

  saving.value = true
  errorMessage.value = ''
  try {
    await cancelShipment(props.shipmentId, { reason: reason.value.trim() })
    pushToast('success', t('shipments.shipmentCancelled'))
    emit('cancelled')
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('common.error')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('shipments.cancelShipment')" @close="$emit('close')">
    <p class="text-muted">{{ $t('shipments.cancelConfirm') }}</p>
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <UiInput v-model="reason" :label="$t('shipments.cancelReason')" required />
    <template #footer>
      <UiButton variant="secondary" @click="$emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="saving" :disabled="saving" @click="submit">{{ $t('common.submit') }}</UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.modal-error {
  margin: 0 0 1rem;
  padding: 0.75rem;
  border-radius: var(--radius-sm);
  background: #fee2e2;
  color: #991b1b;
  font-size: 0.875rem;
}
</style>
