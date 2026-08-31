<script setup lang="ts">
import { ApiError, formatApiErrorForUser } from '~/composables/useApi'

definePageMeta({ layout: 'auth', middleware: 'guest' })

const config = useRuntimeConfig()
const router = useRouter()
const { login } = useAuth()
const { getLandingRoute } = usePermissions()
const { t } = useI18n()
const { resolveInitialTenantId } = useTenantContext()
const { backendOnline, backendStatus, checkBackendStatus } = useBackendStatus()

const tenantId = ref('')
const email = ref('')
const password = ref('')
const loading = ref(false)
const checkingBackend = ref(false)
const loginError = ref('')

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
  loginError.value = ''

  if (!tenantId.value.trim()) {
    loginError.value = t('tenant.required')
    return
  }

  if (!email.value.trim() || !password.value) {
    loginError.value = t('login.validationRequired')
    return
  }

  loading.value = true
  try {
    await login(tenantId.value, email.value, password.value)
    await router.replace(getLandingRoute())
  } catch (error) {
    if (error instanceof ApiError && (error.status === 401 || error.code === 'UNAUTHORIZED')) {
      loginError.value = t('login.invalidCredentials')
    } else {
      loginError.value = formatApiErrorForUser(error)
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

    <p v-if="loginError" class="login-error" role="alert">{{ loginError }}</p>

    <form class="login-form" novalidate @submit.prevent="onSubmit">
      <UiInput
        id="login-tenant-id"
        v-model="tenantId"
        name="tenant_id"
        autocomplete="organization"
        :label="$t('tenant.tenantId')"
        required
      />
      <UiInput
        id="login-email"
        v-model="email"
        name="email"
        type="email"
        autocomplete="username"
        :label="$t('login.email')"
        required
      />
      <UiInput
        id="login-password"
        v-model="password"
        name="password"
        type="password"
        autocomplete="current-password"
        :label="$t('login.password')"
        required
      />
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

.login-error {
  margin: 0 0 1rem;
  padding: 0.75rem 1rem;
  border-radius: var(--radius-md);
  border: 1px solid #fecaca;
  background: #fef2f2;
  color: #991b1b;
  font-size: 0.875rem;
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
