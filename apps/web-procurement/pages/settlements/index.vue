<script setup lang="ts">
import type { UserCompanyMembership } from '~/types/company'
import type { FreightSettlement, SettlementActor } from '~/types/settlement'
import { formatMoney } from '~/types/evaluation'
import { resolveSettlementActor, filterSettlementsByRegister } from '~/utils/settlement'
import { isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { listSettlements } = useSettlementApi()
const { getUserCompanies } = useCompanies()
const { currentCompanyId } = useTenantContext()
const authStore = useAuthStore()
const { pushToast } = useToast()
const { t } = useI18n()

const loading = ref(true)
const apiUnavailable = ref(false)
const items = ref<FreightSettlement[]>([])
const total = ref(0)
const memberships = ref<UserCompanyMembership[]>([])
const actor = ref<SettlementActor | null>(null)
const pagination = reactive({ limit: 20, offset: 0 })

const registerFilter = computed(() => {
  const raw = route.query.register
  return typeof raw === 'string' ? raw : null
})

const filteredItems = computed(() => filterSettlementsByRegister(items.value, registerFilter.value))
const hasItems = computed(() => filteredItems.value.length > 0)
const canGoPrev = computed(() => pagination.offset > 0)
const canGoNext = computed(() => pagination.offset + pagination.limit < total.value)
const missingCompany = computed(() => !currentCompanyId.value)
const missingActor = computed(() => !actor.value)
const pageTitle = computed(() =>
  actor.value === 'CARRIER' ? t('settlements.carrierTitle') : t('settlements.buyerTitle'),
)

async function loadMemberships() {
  if (!authStore.user?.id) {
    memberships.value = []
    actor.value = null
    return
  }
  memberships.value = await getUserCompanies(authStore.user.id)
  actor.value = resolveSettlementActor(currentCompanyId.value, memberships.value)
}

async function loadSettlements() {
  loading.value = true
  apiUnavailable.value = false
  try {
    await loadMemberships()
    if (!currentCompanyId.value || !actor.value) {
      items.value = []
      total.value = 0
      return
    }
    const data = await listSettlements(actor.value, {
      limit: pagination.limit,
      offset: pagination.offset,
    })
    items.value = data.items
    total.value = data.total
  } catch (error) {
    items.value = []
    total.value = 0
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('settlements.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function goPrev() {
  pagination.offset = Math.max(0, pagination.offset - pagination.limit)
  loadSettlements()
}

function goNext() {
  pagination.offset += pagination.limit
  loadSettlements()
}

watch(currentCompanyId, () => {
  pagination.offset = 0
  loadSettlements()
}, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="pageTitle">
      <template v-if="actor" #subtitle>
        {{ t(`settlements.actor.${actor}`) }}
      </template>
    </PageHeader>

    <Card v-if="registerFilter">
      <p class="filter-note">
        {{ t('settlements.registerFilter') }}:
        <NuxtLink :to="`/billing-registers`">{{ registerFilter }}</NuxtLink>
        <NuxtLink class="clear-filter" to="/settlements">{{ t('settlements.clearFilter') }}</NuxtLink>
      </p>
    </Card>

    <div v-if="loading" role="status" aria-live="polite">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="missingCompany" :title="t('settlements.missingCompany')" />
    <EmptyState v-else-if="missingActor" :title="t('settlements.missingActor')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('settlements.loadFailed')" />
    <EmptyState v-else-if="!hasItems" :title="t('settlements.empty')" />

    <Card v-else>
      <div class="table-scroll">
        <Table
          :columns="[
            t('settlements.settlementNumber'),
            t('common.status'),
            t('settlements.money.agreedBase'),
            t('settlements.money.additionalApproved'),
            t('settlements.money.totalWithVat'),
            t('settlements.register'),
            t('common.actions'),
          ]"
          :loading="loading"
        >
          <tr v-for="item in filteredItems" :key="item.id">
            <td>
              <NuxtLink :to="`/settlements/${item.id}`" class="link">
                {{ item.settlement_number }}
              </NuxtLink>
            </td>
            <td><Badge :status="item.status" /></td>
            <td>{{ formatMoney(item.base_freight_amount, item.currency_code) }}</td>
            <td>{{ formatMoney(item.approved_accessorial_total, item.currency_code) }}</td>
            <td>{{ formatMoney(item.total_with_vat, item.currency_code) }}</td>
            <td>
              <NuxtLink
                v-if="item.billing_register_id"
                :to="`/settlements?register=${item.billing_register_id}`"
              >
                {{ item.billing_register_id.slice(0, 8) }}…
              </NuxtLink>
              <span v-else aria-hidden="true">—</span>
            </td>
            <td>
              <NuxtLink :to="`/settlements/${item.id}`">{{ t('common.details') }}</NuxtLink>
            </td>
          </tr>
        </Table>
      </div>

      <div v-if="!registerFilter" class="pagination">
        <span class="text-sm text-muted">{{ total }} {{ t('settlements.countLabel') }}</span>
        <div class="pagination__actions">
          <Button size="sm" variant="secondary" :disabled="!canGoPrev" @click="goPrev">←</Button>
          <Button size="sm" variant="secondary" :disabled="!canGoNext" @click="goNext">→</Button>
        </div>
      </div>
    </Card>
  </div>
</template>

<style scoped>
.link {
  font-weight: 500;
  text-decoration: none;
}

.link:hover {
  text-decoration: underline;
}

.filter-note {
  margin: 0;
  font-size: 0.875rem;
}

.clear-filter {
  margin-left: 0.75rem;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--color-border);
}

.pagination__actions {
  display: flex;
  gap: 0.5rem;
}
</style>
