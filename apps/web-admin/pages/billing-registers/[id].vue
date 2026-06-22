<script setup lang="ts">
import type { BillingRegister } from '~/types/billing'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { apiGet } = useApi()
const item = ref<BillingRegister | null>(null)

onMounted(async () => {
  try {
    item.value = await apiGet<BillingRegister>(`/api/v1/billing-registers/${route.params.id}`)
  } catch {
    item.value = null
  }
})
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="item?.register_number || $t('billing.title')" />
    <UiCard v-if="item">
      <div class="form-grid form-grid--2">
        <div><span class="text-muted">{{ $t('common.status') }}</span><div><UiBadge :status="item.status" /></div></div>
        <div><span class="text-muted">{{ $t('billing.totalWithVat') }}</span><div>{{ item.total_with_vat ?? '—' }}</div></div>
      </div>
    </UiCard>
    <LowCodeCustomFieldsPanel
      v-if="item"
      entity-type="BILLING_REGISTER"
      :entity-id="item.id"
      :entity-status="item.status"
    />
    <UiEmptyState v-else :title="$t('common.empty')" />
  </div>
</template>
