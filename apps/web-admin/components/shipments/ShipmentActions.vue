<script setup lang="ts">
import {
  canCancelShipment,
  getNextShipmentStatus,
  type Shipment,
} from '~/types/shipment'

const props = defineProps<{ shipment: Shipment }>()
const emit = defineEmits<{ updated: []; cancel: [] }>()

const { acceptShipment, updateShipmentStatus } = useShipmentsApi()
const { pushToast } = useToast()
const { t } = useI18n()
const router = useRouter()

const accepting = ref(false)
const updatingStatus = ref(false)
const movingToBilling = ref(false)

const nextStatus = computed(() => getNextShipmentStatus(props.shipment.status))
const canAccept = computed(() => props.shipment.status === 'CARRIER_ASSIGNED')
const showNextStatus = computed(
  () =>
    nextStatus.value &&
    props.shipment.status !== 'CARRIER_ASSIGNED' &&
    props.shipment.status !== 'CANCELLED' &&
    props.shipment.status !== 'DOCUMENTS_COMPLETED',
)
const showDocumentsHint = computed(() => props.shipment.status === 'DOCUMENTS_COMPLETED')
const showBillingHint = computed(() => props.shipment.status === 'READY_FOR_BILLING')
const showCancel = computed(() => canCancelShipment(props.shipment.status))

async function onAccept() {
  accepting.value = true
  try {
    await acceptShipment(props.shipment.id)
    pushToast('success', t('shipments.shipmentAccepted'))
    emit('updated')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    accepting.value = false
  }
}

async function onNextStatus() {
  if (!nextStatus.value) return
  updatingStatus.value = true
  try {
    await updateShipmentStatus(props.shipment.id, { status: nextStatus.value })
    pushToast('success', t('shipments.statusUpdated'))
    emit('updated')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('shipments.statusUpdateFailed'))
  } finally {
    updatingStatus.value = false
  }
}

async function onMoveToBilling() {
  movingToBilling.value = true
  try {
    await updateShipmentStatus(props.shipment.id, { status: 'READY_FOR_BILLING' })
    pushToast('success', t('shipments.statusUpdated'))
    emit('updated')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('shipments.statusUpdateFailed'))
  } finally {
    movingToBilling.value = false
  }
}

function onCreateBillingRegister() {
  router.push(`/billing-registers?shipment_id=${props.shipment.id}`)
}
</script>

<template>
  <div class="actions-stack">
    <div v-if="canAccept" class="actions-row">
      <UiButton :loading="accepting" :disabled="accepting" @click="onAccept">
        {{ $t('shipments.acceptShipment') }}
      </UiButton>
    </div>

    <div v-if="showNextStatus" class="actions-row">
      <UiButton
        :loading="updatingStatus"
        :disabled="updatingStatus"
        variant="secondary"
        @click="onNextStatus"
      >
        {{ $t('shipments.nextStatus') }}: {{ nextStatus }}
      </UiButton>
    </div>

    <UiCard v-if="showDocumentsHint" class="hint-card">
      <p>{{ $t('shipments.documentsReadyHint') }}</p>
      <UiButton :loading="movingToBilling" :disabled="movingToBilling" @click="onMoveToBilling">
        {{ $t('shipments.moveToReadyForBilling') }}
      </UiButton>
    </UiCard>

    <UiCard v-if="showBillingHint" class="hint-card">
      <p>{{ $t('shipments.readyForBillingHint') }}</p>
      <UiButton @click="onCreateBillingRegister">{{ $t('shipments.createBillingRegister') }}</UiButton>
    </UiCard>

    <div v-if="showCancel" class="actions-row">
      <UiButton variant="secondary" @click="emit('cancel')">
        {{ $t('shipments.cancelShipment') }}
      </UiButton>
    </div>
  </div>
</template>

<style scoped>
.actions-stack {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.hint-card p {
  margin: 0 0 0.75rem;
  color: var(--color-text-muted);
}
</style>
