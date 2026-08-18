<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import OfflineBanner from '@/components/OfflineBanner.vue'
import { useSubmissionLock } from '@/composables/useSubmission'
import { useAuthStore } from '@/stores/auth'
import { useNetworkStore } from '@/stores/network'
import type { DriverMilestoneEventType, DriverShipmentDetail } from '@/types/driver'
import { createOperationId } from '@/utils/idempotency'
import { allowedMilestoneActions, canUploadPOD } from '@/utils/milestones'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const network = useNetworkStore()
const { t } = useI18n()
const { submitting, runOnce } = useSubmissionLock()

const shipmentId = computed(() => String(route.params.shipmentId))
const loading = ref(true)
const errorMessage = ref('')
const actionError = ref('')
const shipment = ref<DriverShipmentDetail | null>(null)

const milestoneActions = computed(() =>
  shipment.value ? allowedMilestoneActions(shipment.value.status) : [],
)
const showPodUpload = computed(() => (shipment.value ? canUploadPOD(shipment.value.status) : false))

function formatWhen(value?: string) {
  if (!value) return '—'
  try {
    return new Intl.DateTimeFormat('ru-RU', {
      day: '2-digit',
      month: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(value))
  } catch {
    return value
  }
}

function milestoneLabel(type: DriverMilestoneEventType) {
  return t(`milestone.actions.${type}`)
}

async function loadShipment() {
  loading.value = true
  errorMessage.value = ''
  const api = auth.createApi(() => network.online)
  const result = await api.getShipment(shipmentId.value)

  if (result.outcome === 'SUCCESS' && result.data) {
    shipment.value = result.data
    loading.value = false
    return
  }

  if (result.error?.status === 404) {
    errorMessage.value = t('shipmentDetail.notFound')
  } else if (result.error?.status === 401) {
    await auth.logout()
    await router.replace('/login')
    return
  } else if (result.outcome === 'REQUEST_NOT_SENT') {
    errorMessage.value = t('delay.offline')
  } else {
    errorMessage.value = result.error?.message || t('common.error')
  }
  loading.value = false
}

async function recordMilestone(type: DriverMilestoneEventType) {
  actionError.value = ''
  const operationId = createOperationId('milestone', `${shipmentId.value}:${type}`)
  const api = auth.createApi(() => network.online)
  const result = await runOnce(() =>
    api.recordEvent(shipmentId.value, { type, idempotencyKey: operationId }),
  )

  if (result.outcome === 'SUCCESS') {
    await loadShipment()
    return
  }

  if (result.outcome === 'REQUEST_SENT_RESPONSE_UNKNOWN') {
    actionError.value = t('milestone.unknown')
    return
  }

  if (result.outcome === 'REQUEST_NOT_SENT') {
    actionError.value = t('delay.offline')
    return
  }

  actionError.value = result.error?.message || t('milestone.failed')
}

onMounted(loadShipment)
</script>

<template>
  <ion-page>
    <ion-header>
      <ion-toolbar color="primary">
        <ion-buttons slot="start">
          <ion-back-button default-href="/shipments" />
        </ion-buttons>
        <ion-title>{{ t('shipmentDetail.title') }}</ion-title>
      </ion-toolbar>
      <OfflineBanner />
    </ion-header>
    <ion-content class="ion-padding">
      <div v-if="loading" class="center">
        <ion-spinner name="crescent" />
      </div>
      <div v-else-if="errorMessage" class="center">
        <ion-text color="danger"><p>{{ errorMessage }}</p></ion-text>
        <ion-button @click="loadShipment">{{ t('common.retry') }}</ion-button>
      </div>
      <div v-else-if="shipment" class="detail">
        <h1>{{ shipment.shipmentNumber }}</h1>
        <p><strong>{{ t('shipments.status') }}:</strong> {{ shipment.status }}</p>
        <p><strong>{{ t('shipments.pickup') }}:</strong> {{ formatWhen(shipment.plannedPickupAt) }}</p>
        <p><strong>{{ t('shipments.delivery') }}:</strong> {{ formatWhen(shipment.plannedDeliveryAt) }}</p>
        <p v-if="shipment.actualPickupAt">
          <strong>{{ t('shipmentDetail.actualPickup') }}:</strong> {{ formatWhen(shipment.actualPickupAt) }}
        </p>
        <p v-if="shipment.actualDeliveryAt">
          <strong>{{ t('shipmentDetail.actualDelivery') }}:</strong> {{ formatWhen(shipment.actualDeliveryAt) }}
        </p>
        <p v-if="shipment.transportMode">
          <strong>{{ t('shipmentDetail.transportMode') }}:</strong> {{ shipment.transportMode }}
        </p>

        <section v-if="milestoneActions.length" class="section">
          <h2>{{ t('milestone.title') }}</h2>
          <ion-button
            v-for="action in milestoneActions"
            :key="action"
            expand="block"
            size="large"
            color="primary"
            class="action-btn"
            :disabled="submitting"
            @click="recordMilestone(action)"
          >
            {{ milestoneLabel(action) }}
          </ion-button>
          <ion-text v-if="actionError" color="danger">
            <p class="inline-error">{{ actionError }}</p>
          </ion-text>
        </section>

        <ion-button
          v-if="showPodUpload"
          expand="block"
          size="large"
          color="success"
          class="action-btn"
          @click="router.push(`/shipments/${shipmentId}/pod`)"
        >
          {{ t('pod.upload') }}
        </ion-button>

        <ion-button
          expand="block"
          size="large"
          color="warning"
          class="action-btn"
          @click="router.push(`/shipments/${shipmentId}/delay`)"
        >
          {{ t('shipmentDetail.reportDelay') }}
        </ion-button>
        <ion-button
          expand="block"
          size="large"
          color="danger"
          class="action-btn"
          @click="router.push(`/shipments/${shipmentId}/problem`)"
        >
          {{ t('shipmentDetail.reportProblem') }}
        </ion-button>
      </div>
    </ion-content>
  </ion-page>
</template>

<style scoped>
.center {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  margin-top: 48px;
}
.detail h1 {
  font-size: 1.5rem;
  margin-bottom: 12px;
}
.section {
  margin: 20px 0 8px;
}
.section h2 {
  font-size: 1.1rem;
  margin-bottom: 8px;
}
.action-btn {
  margin-top: 12px;
  min-height: 56px;
  font-size: 1.05rem;
}
.inline-error {
  margin-top: 8px;
}
</style>
