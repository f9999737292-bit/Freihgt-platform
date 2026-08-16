<script setup lang="ts">
import { useNetworkStore } from '@/stores/network'
import { useI18n } from 'vue-i18n'

const network = useNetworkStore()
const { t } = useI18n()
</script>

<template>
  <ion-toolbar v-if="!network.online" color="warning" class="offline-banner">
    <ion-title>{{ t('network.offlineBanner') }}</ion-title>
  </ion-toolbar>
</template>

<style scoped>
.offline-banner {
  --min-height: 44px;
}
ion-title {
  font-size: 0.95rem;
  font-weight: 600;
}
</style>
