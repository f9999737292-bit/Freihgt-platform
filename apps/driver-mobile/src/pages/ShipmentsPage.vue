<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import ShipmentCard from '@/components/ShipmentCard.vue'
import OfflineBanner from '@/components/OfflineBanner.vue'
import { useAuthStore } from '@/stores/auth'
import { useNetworkStore } from '@/stores/network'
import type { DriverShipmentSummary } from '@/types/driver'

const auth = useAuthStore()
const network = useNetworkStore()
const router = useRouter()
const { t } = useI18n()

const loading = ref(true)
const errorMessage = ref('')
const items = ref<DriverShipmentSummary[]>([])

async function loadShipments() {
  loading.value = true
  errorMessage.value = ''
  const api = auth.createApi(() => network.online)
  const result = await api.getMyShipments({ limit: 50, offset: 0 })

  if (result.outcome === 'SUCCESS' && result.data) {
    items.value = result.data.items ?? []
    loading.value = false
    return
  }

  if (result.error?.status === 401) {
    await auth.logout()
    await router.replace('/login')
    return
  }

  if (result.outcome === 'REQUEST_NOT_SENT') {
    errorMessage.value = t('delay.offline')
  } else if (result.error) {
    errorMessage.value = result.error.message
  } else {
    errorMessage.value = t('shipments.loadError')
  }
  loading.value = false
}

async function logout() {
  await auth.logout()
  await router.replace('/login')
}

onMounted(loadShipments)
</script>

<template>
  <ion-page>
    <ion-header>
      <ion-toolbar color="primary">
        <ion-title>{{ t('shipments.title') }}</ion-title>
        <ion-buttons slot="end">
          <ion-button @click="logout">{{ t('common.logout') }}</ion-button>
        </ion-buttons>
      </ion-toolbar>
      <OfflineBanner />
    </ion-header>
    <ion-content class="ion-padding">
      <div v-if="loading" class="center">
        <ion-spinner name="crescent" />
        <p>{{ t('common.loading') }}</p>
      </div>
      <div v-else-if="errorMessage" class="center">
        <ion-text color="danger"><p>{{ errorMessage }}</p></ion-text>
        <ion-button expand="block" size="large" @click="loadShipments">{{ t('common.retry') }}</ion-button>
      </div>
      <div v-else-if="items.length === 0" class="center">
        <p>{{ t('shipments.empty') }}</p>
        <ion-button expand="block" size="large" @click="loadShipments">{{ t('common.refresh') }}</ion-button>
      </div>
      <div v-else>
        <ShipmentCard
          v-for="shipment in items"
          :key="shipment.id"
          :shipment="shipment"
          @open="router.push(`/shipments/${shipment.id}`)"
        />
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
</style>
