<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import OfflineBanner from '@/components/OfflineBanner.vue'
import { getPilotTenantId } from '@/config/env'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const { t } = useI18n()

const email = ref('')
const password = ref('')
const loading = ref(false)
const errorMessage = ref('')

async function onSubmit() {
  errorMessage.value = ''
  if (!getPilotTenantId()) {
    errorMessage.value = t('login.tenantNotConfigured')
    return
  }
  loading.value = true
  try {
    await auth.login({ email: email.value, password: password.value })
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/shipments'
    await router.replace(redirect)
  } catch (error) {
    if (error instanceof Error) {
      if (error.message === 'offline') {
        errorMessage.value = t('delay.offline')
      } else if (error.message === 'unknown') {
        errorMessage.value = t('delay.unknown')
      } else {
        errorMessage.value = error.message
      }
    } else {
      errorMessage.value = t('common.error')
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <ion-page>
    <ion-header>
      <ion-toolbar color="primary">
        <ion-title>{{ t('login.title') }}</ion-title>
      </ion-toolbar>
      <OfflineBanner />
    </ion-header>
    <ion-content class="ion-padding form-content">
      <form class="login-form" @submit.prevent="onSubmit">
        <ion-item lines="full">
          <ion-label position="stacked">{{ t('login.email') }}</ion-label>
          <ion-input v-model="email" type="email" autocomplete="username" required />
        </ion-item>
        <ion-item lines="full">
          <ion-label position="stacked">{{ t('login.password') }}</ion-label>
          <ion-input v-model="password" type="password" autocomplete="current-password" required />
        </ion-item>
        <p class="hint">{{ t('login.hint') }}</p>
        <ion-text v-if="errorMessage" color="danger">
          <p class="error">{{ errorMessage }}</p>
        </ion-text>
        <ion-button expand="block" size="large" type="submit" :disabled="loading" class="submit-btn">
          {{ loading ? t('common.loading') : t('common.login') }}
        </ion-button>
      </form>
    </ion-content>
  </ion-page>
</template>

<style scoped>
.login-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: 480px;
  margin: 24px auto 0;
}
.hint {
  color: var(--ion-color-medium);
  font-size: 0.9rem;
  margin: 8px 4px;
}
.error {
  margin: 8px 4px;
}
.submit-btn {
  margin-top: 16px;
  min-height: 52px;
}
</style>
