<script setup lang="ts">
definePageMeta({ layout: 'auth', middleware: 'guest' })

const config = useRuntimeConfig()
const { login } = useAuth()
const { pushToast } = useToast()
const { t } = useI18n()
const { resolveInitialTenantId } = useTenantContext()
const { backendOnline, backendStatus, checkBackendStatus } = useBackendStatus()

const tenantId = ref('')
const email = ref('demo@7rights.local')
const password = ref('123456')
const loading = ref(false)
const checkingBackend = ref(false)

onMounted(async () => {
  tenantId.value = resolveInitialTenantId()
  checkingBackend.value = true
  try {
    await checkBackendStatus()
  } finally {
    checkingBackend.value = false
  }
})

async function onRefreshBackendStatus() {
  checkingBackend.value = true
  try {
    await checkBackendStatus()
  } finally {
    checkingBackend.value = false
  }
}

async function onSubmit() {
  if (!tenantId.value.trim()) {
    pushToast('error', t('tenant.required'))
    return
  }

  loading.value = true
  try {
    await login(tenantId.value, email.value, password.value)
  } catch (error) {
    if (error instanceof Error && error.message !== t('tenant.required')) {
      pushToast('error', error.message)
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div>
    <h2>{{ $t('login.title') }}</h2>

    <div class="login-backend-status" :class="backendOnline ? 'login-backend-status--online' : 'login-backend-status--offline'">
      <div class="login-backend-status__header">
        <strong>{{ $t('backendStatus.title') }}</strong>
        <UiButton size="sm" variant="ghost" :loading="checkingBackend" @click="onRefreshBackendStatus">
          {{ $t('backendStatus.refresh') }}
        </UiButton>
      </div>
      <p class="login-backend-status__value">
        {{
          checkingBackend || backendStatus === 'checking'
            ? $t('common.loading')
            : backendOnline
              ? $t('backendStatus.online')
              : $t('backendStatus.offline')
        }}
      </p>
      <p v-if="!backendOnline && !checkingBackend" class="login-backend-status__hint">
        {{ $t('login.backendOfflineHint') }}
      </p>
      <pre v-if="!backendOnline && !checkingBackend" class="login-backend-status__commands">{{ $t('backendStatus.startBackendCommands') }}</pre>
      <p v-if="config.public.mockAuth" class="login-backend-status__mock">
        {{ $t('backendStatus.mockModeActive') }} — {{ $t('backendStatus.mockModeHint') }}
      </p>
    </div>

    <form class="login-form" @submit.prevent="onSubmit">
      <UiInput v-model="tenantId" :label="$t('tenant.tenantId')" required />
      <UiInput v-model="email" type="email" :label="$t('login.email')" required />
      <UiInput v-model="password" type="password" :label="$t('login.password')" required />
      <p v-if="config.public.mockAuth" class="text-sm text-muted">{{ $t('login.hint') }}</p>
      <UiButton type="submit" :loading="loading" style="width: 100%">{{ $t('common.login') }}</UiButton>
    </form>
  </div>
</template>

<style scoped>
.login-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.login-backend-status {
  margin-bottom: 1.25rem;
  padding: 0.875rem 1rem;
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border);
  font-size: 0.875rem;
}

.login-backend-status--online {
  background: #ecfdf5;
  border-color: #a7f3d0;
  color: #065f46;
}

.login-backend-status--offline {
  background: #fffbeb;
  border-color: #fde68a;
  color: #92400e;
}

.login-backend-status__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.35rem;
}

.login-backend-status__value {
  margin: 0;
  font-weight: 600;
}

.login-backend-status__hint,
.login-backend-status__mock {
  margin: 0.5rem 0 0;
  font-size: 0.8125rem;
}

.login-backend-status__commands {
  margin: 0.5rem 0 0;
  padding: 0.5rem 0.75rem;
  border-radius: var(--radius-sm);
  background: rgba(0, 0, 0, 0.06);
  font-size: 0.75rem;
  line-height: 1.5;
  white-space: pre-wrap;
}
</style>
