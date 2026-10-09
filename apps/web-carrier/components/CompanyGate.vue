<script setup lang="ts">
import { computed, ref } from 'vue'
import { Button, CustomerPageHeader, EmptyState, Select } from '@freight-platform/ui'

const props = defineProps<{
  companies: Array<{ companyId: string; legalName: string }>
}>()
const emit = defineEmits<{ select: [companyId: string] }>()
const { t } = useI18n()
const companyId = ref(props.companies[0]?.companyId ?? '')

const options = computed(() => props.companies.map((company) => ({
  value: company.companyId,
  label: company.legalName,
})))

function submit() {
  if (!companyId.value) return
  if (!props.companies.some((company) => company.companyId === companyId.value)) return
  emit('select', companyId.value)
}
</script>

<template>
  <div data-testid="company-gate">
    <CustomerPageHeader :title="t('carrier.companyGateTitle')" />
    <EmptyState v-if="companies.length === 0" :title="t('carrier.companyGateEmpty')" />
    <form v-else class="gate-form" @submit.prevent="submit">
      <Select v-model="companyId" name="company" data-testid="company-select" :label="t('carrier.companyLabel')" :options="options" />
      <Button type="submit" data-testid="company-continue">{{ t('carrier.continue') }}</Button>
    </form>
  </div>
</template>

<style scoped>
.gate-form {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  max-width: 24rem;
}
</style>
