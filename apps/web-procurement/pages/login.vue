<script setup lang="ts">
definePageMeta({ middleware: 'guest' })

const config = useRuntimeConfig()
const route = useRoute()
const router = useRouter()
const { login } = useAuth()
const { t } = useI18n()

const tenantId = ref(config.public.defaultTenantId || '')
const email = ref(import.meta.dev ? 'procurement@bintrans.local' : '')
const password = ref(import.meta.dev ? '123456' : '')
const loading = ref(false)
const errorMessage = ref('')

async function onSubmit() {
  errorMessage.value = ''
  if (!tenantId.value.trim()) {
    errorMessage.value = t('login.tenantRequired')
    return
  }

  loading.value = true
  try {
    await login(tenantId.value, email.value, password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.replace(redirect)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('common.error')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <h2>{{ $t('login.title') }}</h2>
    <p class="login-page__hint">{{ $t('login.hint') }}</p>

    <form class="login-form" @submit.prevent="onSubmit">
      <label class="field">
        <span>{{ $t('login.tenantId') }}</span>
        <input v-model="tenantId" type="text" required autocomplete="organization" />
      </label>
      <label class="field">
        <span>{{ $t('login.email') }}</span>
        <input v-model="email" type="email" required autocomplete="username" />
      </label>
      <label class="field">
        <span>{{ $t('login.password') }}</span>
        <input v-model="password" type="password" required autocomplete="current-password" />
      </label>
      <p v-if="errorMessage" class="login-page__error">{{ errorMessage }}</p>
      <button type="submit" class="btn" :disabled="loading">
        {{ loading ? $t('common.loading') : $t('login.submit') }}
      </button>
    </form>
  </div>
</template>

<style scoped>
.login-page {
  max-width: 28rem;
}

.login-page h2 {
  margin: 0 0 0.5rem;
}

.login-page__hint {
  margin: 0 0 1.25rem;
  color: #6b7280;
  font-size: 0.875rem;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  font-size: 0.875rem;
}

.field input {
  padding: 0.625rem 0.75rem;
  border: 1px solid #d1d5db;
  border-radius: 0.375rem;
  font: inherit;
}

.login-page__error {
  margin: 0;
  color: #b91c1c;
  font-size: 0.875rem;
}

.btn {
  padding: 0.625rem 1rem;
  border: none;
  border-radius: 0.375rem;
  background: #2563eb;
  color: #fff;
  font: inherit;
  cursor: pointer;
}

.btn:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}
</style>
