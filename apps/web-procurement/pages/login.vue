<script setup lang="ts">
definePageMeta({ middleware: 'guest', layout: false })

const config = useRuntimeConfig()
const route = useRoute()
const { login } = useAuth()
const { resolveInitialTenantId } = useTenantContext()
const { t } = useI18n()

const tenantId = ref(resolveInitialTenantId())
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
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/tenders'
    await navigateTo(redirect, { replace: true })
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
      <Input v-model="tenantId" :label="$t('login.tenantId')" required />
      <Input v-model="email" type="email" :label="$t('login.email')" required />
      <Input v-model="password" type="password" :label="$t('login.password')" required />
      <p v-if="errorMessage" class="login-page__error">{{ errorMessage }}</p>
      <Button type="submit" :loading="loading" :disabled="loading">
        {{ loading ? $t('common.loading') : $t('login.submit') }}
      </Button>
    </form>
  </div>
</template>

<style scoped>
.login-page {
  max-width: 28rem;
  margin: 4rem auto;
  padding: 0 1rem;
}

.login-page h2 {
  margin: 0 0 0.5rem;
}

.login-page__hint {
  margin: 0 0 1.25rem;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.login-page__error {
  margin: 0;
  color: #b91c1c;
  font-size: 0.875rem;
}
</style>
