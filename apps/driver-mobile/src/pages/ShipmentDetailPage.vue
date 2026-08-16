<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import OfflineBanner from '@/components/OfflineBanner.vue'
import { useAuthStore } from '@/stores/auth'
import { useNetworkStore } from '@/stores/network'
import type { DriverShipmentDetail } from '@/types/driver'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const network = useNetworkStore()
const { t } = useI18n()

const shipmentId = computed(() => String(route.params.shipmentId))
const loading = ref(true)
const errorMessage = ref('')
const shipment = ref<DriverShipmentDetail | null>(null)

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
        <p v-if="shipment.transportMode">
          <strong>{{ t('shipmentDetail.transportMode') }}:</strong> {{ shipment.transportMode }}
        </p>

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
.action-btn {
  margin-top: 16px;
  min-height: 56px;
  font-size: 1.05rem;
}
</style>
