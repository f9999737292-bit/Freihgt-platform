<script setup lang="ts">
import { Button, Card, CustomerPageHeader, ErrorState, Input } from '@freight-platform/ui'

definePageMeta({ layout: 'public' })

const office = useCarrierOffice()
const { t } = useI18n()
const tenantId = ref('')
const email = ref('')
const password = ref('')
const denied = ref(false)
const failure = ref('')
const pending = ref(false)

async function submit() {
  denied.value = false
  failure.value = ''
  pending.value = true
  try {
    const result = await office.login({
      tenantId: tenantId.value.trim(),
      email: email.value.trim(),
      password: password.value,
    })
    if (!result.ok) {
      denied.value = true
      return
    }
    await navigateTo('/')
  } catch (error) {
    office.session.value = null
    if (import.meta.client) window.sessionStorage.removeItem('freight_carrier_tab_session')
    failure.value = error instanceof Error ? error.message : t('carrier.unavailable')
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <Card>
    <form class="login-form" data-testid="login-form" @submit.prevent="submit">
      <CustomerPageHeader :title="t('carrier.loginTitle')" />
      <ErrorState v-if="denied" :title="t('carrier.deniedRole')" />
      <ErrorState v-else-if="failure" :title="t('carrier.unavailable')" :description="failure" />
      <Input v-model="tenantId" name="tenant_id" data-testid="tenant-id" :label="t('carrier.tenantId')" required />
      <Input v-model="email" name="email" type="email" data-testid="email" :label="t('carrier.email')" required />
      <Input v-model="password" name="password" type="password" data-testid="password" :label="t('carrier.password')" required />
      <Button type="submit" data-testid="sign-in" :disabled="pending">{{ t('carrier.signIn') }}</Button>
    </form>
  </Card>
</template>

<style scoped>
.login-form {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  width: min(24rem, 100%);
}
</style>
