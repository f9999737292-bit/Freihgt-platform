<script setup lang="ts">
import { Button, Card, CustomerPageHeader, ErrorState, Input } from '@freight-platform/ui'
import { SHIPPER_TAB_SESSION_STORAGE_KEY } from '@freight-platform/portal-client'

definePageMeta({ layout: 'public' })

const office = useShipperOffice()
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
    if (import.meta.client) window.sessionStorage.removeItem(SHIPPER_TAB_SESSION_STORAGE_KEY)
    failure.value = error instanceof Error ? error.message : t('shipper.unavailable')
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <Card>
    <form class="login-form" data-testid="login-form" @submit.prevent="submit">
      <CustomerPageHeader :title="t('shipper.loginTitle')" />
      <ErrorState v-if="denied" :title="t('shipper.deniedRole')" />
      <ErrorState v-else-if="failure" :title="t('shipper.unavailable')" :description="failure" />
      <Input v-model="tenantId" name="tenant_id" data-testid="tenant-id" :label="t('shipper.tenantId')" required />
      <Input v-model="email" name="email" type="email" data-testid="email" :label="t('shipper.email')" required />
      <Input v-model="password" name="password" type="password" data-testid="password" :label="t('shipper.password')" required />
      <Button type="submit" data-testid="sign-in" :disabled="pending">{{ t('shipper.signIn') }}</Button>
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
