<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { checkmarkCircle, closeCircle, helpCircle } from 'ionicons/icons'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const kind = computed(() => String(route.query.kind || 'delay'))
const status = computed(() => String(route.query.status || 'success'))
const shipmentId = computed(() => String(route.query.shipmentId || ''))

const title = computed(() => {
  if (status.value === 'success') {
    return kind.value === 'problem' ? t('problem.successTitle') : t('delay.successTitle')
  }
  if (status.value === 'unknown') {
    return t('result.unknown')
  }
  return t('result.failure')
})

const body = computed(() => {
  if (status.value === 'success') {
    return kind.value === 'problem' ? t('problem.successBody') : t('delay.successBody')
  }
  if (status.value === 'unknown') {
    return kind.value === 'problem' ? t('problem.unknown') : t('delay.unknown')
  }
  return kind.value === 'problem' ? t('problem.failed') : t('delay.failed')
})

const icon = computed(() => {
  if (status.value === 'success') return checkmarkCircle
  if (status.value === 'unknown') return helpCircle
  return closeCircle
})

const color = computed(() => {
  if (status.value === 'success') return 'success'
  if (status.value === 'unknown') return 'warning'
  return 'danger'
})
</script>

<template>
  <ion-page>
    <ion-header>
      <ion-toolbar color="primary">
        <ion-title>{{ t('result.title') }}</ion-title>
      </ion-toolbar>
    </ion-header>
    <ion-content class="ion-padding result-content">
      <div class="result-card">
        <ion-icon :icon="icon" :color="color" class="result-icon" />
        <h1>{{ title }}</h1>
        <p>{{ body }}</p>
      </div>
      <ion-button
        v-if="shipmentId"
        expand="block"
        size="large"
        @click="router.replace(`/shipments/${shipmentId}`)"
      >
        {{ t('result.backToShipment') }}
      </ion-button>
      <ion-button expand="block" fill="outline" size="large" @click="router.replace('/shipments')">
        {{ t('result.backToList') }}
      </ion-button>
    </ion-content>
  </ion-page>
</template>

<style scoped>
.result-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.result-card {
  text-align: center;
  margin: 32px 0 24px;
}
.result-icon {
  font-size: 72px;
  margin-bottom: 12px;
}
h1 {
  font-size: 1.4rem;
  margin-bottom: 8px;
}
</style>
