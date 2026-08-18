<script setup lang="ts">
import type { UserCompanyMembership } from '~/types/company'
import type { BillingRegister, SettlementActor } from '~/types/settlement'
import { formatMoney } from '~/types/evaluation'
import { resolveSettlementActor } from '~/utils/settlement'
import { isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { listBillingRegisters } = useSettlementApi()
const { getUserCompanies } = useCompanies()
const { currentCompanyId, tenantId } = useTenantContext()
const authStore = useAuthStore()
const { pushToast } = useToast()
const { t } = useI18n()

const loading = ref(true)
const apiUnavailable = ref(false)
const items = ref<BillingRegister[]>([])
const total = ref(0)
const actor = ref<SettlementActor | null>(null)

const hasItems = computed(() => items.value.length > 0)
const missingTenant = computed(() => !tenantId.value)

async function loadRegisters() {
  loading.value = true
  apiUnavailable.value = false
  try {
    if (!authStore.user?.id || !currentCompanyId.value) {
      items.value = []
      total.value = 0
      return
    }
    const memberships: UserCompanyMembership[] = await getUserCompanies(authStore.user.id)
    actor.value = resolveSettlementActor(currentCompanyId.value, memberships)
    if (!actor.value) {
      items.value = []
      total.value = 0
      return
    }
    const filters =
      actor.value === 'BUYER'
        ? { customerCompanyId: currentCompanyId.value }
        : { contractorCompanyId: currentCompanyId.value }
    const data = await listBillingRegisters(filters)
    items.value = data.items
    total.value = data.total
  } catch (error) {
    items.value = []
    total.value = 0
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('settlements.registersLoadFailed'))
    }
  } finally {
    loading.value = false
  }
}

watch([currentCompanyId, tenantId], loadRegisters, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('settlements.registersTitle')" />

    <div v-if="loading" role="status" aria-live="polite">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="missingTenant" :title="t('tenant.required')" />
    <EmptyState v-else-if="!currentCompanyId" :title="t('settlements.missingCompany')" />
    <EmptyState v-else-if="!actor" :title="t('settlements.missingActor')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('settlements.registersLoadFailed')" />
    <EmptyState v-else-if="!hasItems" :title="t('settlements.registersEmpty')" />

    <Card v-else>
      <div class="table-scroll">
        <Table
          :columns="[
            t('settlements.registerNumber'),
            t('common.status'),
            t('settlements.period'),
            t('settlements.money.totalWithVat'),
            t('common.actions'),
          ]"
        >
          <tr v-for="item in items" :key="item.id">
            <td>{{ item.register_number }}</td>
            <td><Badge :status="item.status" /></td>
            <td>{{ item.period_from }} — {{ item.period_to }}</td>
            <td>{{ formatMoney(item.total_with_vat, item.currency_code) }}</td>
            <td>
              <NuxtLink :to="`/settlements?register=${item.id}`">
                {{ t('settlements.viewSettlements') }}
              </NuxtLink>
            </td>
          </tr>
        </Table>
      </div>
      <p class="table-footer">{{ total }} {{ t('settlements.registersCountLabel') }}</p>
    </Card>
  </div>
</template>

<style scoped>
.table-footer {
  margin: 0;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--color-border);
  font-size: 0.875rem;
  color: var(--color-text-muted, #64748b);
}
</style>
