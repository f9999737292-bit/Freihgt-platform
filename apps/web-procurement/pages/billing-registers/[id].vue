<script setup lang="ts">
import type { UserCompanyMembership } from '~/types/company'
import type { BillingRegisterDetail, FreightSettlement, SettlementActor } from '~/types/settlement'
import { formatMoney } from '~/types/evaluation'
import { resolveSettlementActor } from '~/utils/settlement'
import { isApiUnavailableError, shouldShowNotFound } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const {
  getBillingRegister,
  listSettlements,
  includeSettlementInRegister,
  calculateBillingRegister,
  approveBillingRegister,
  createClosingDocumentPackage,
} = useSettlementApi()
const { getUserCompanies } = useCompanies()
const { currentCompanyId, tenantId } = useTenantContext()
const authStore = useAuthStore()
const { pushToast } = useToast()
const { t } = useI18n()

const registerId = computed(() => String(route.params.id))
const detail = ref<BillingRegisterDetail | null>(null)
const eligibleSettlements = ref<FreightSettlement[]>([])
const actor = ref<SettlementActor | null>(null)
const loading = ref(true)
const acting = ref(false)
const notFound = ref(false)
const apiUnavailable = ref(false)
const selectedSettlementId = ref('')

const isBuyer = computed(() => actor.value === 'BUYER')
const canMutate = computed(() => isBuyer.value && (detail.value?.status === 'DRAFT' || detail.value?.status === 'CALCULATED'))
const canCalculate = computed(() => isBuyer.value && detail.value?.status === 'DRAFT' && (detail.value?.items?.length ?? 0) > 0)
const canApprove = computed(() => isBuyer.value && detail.value?.status === 'CALCULATED')
const canCreateDocuments = computed(() => isBuyer.value && detail.value?.status === 'APPROVED')

async function loadDetail() {
  loading.value = true
  apiUnavailable.value = false
  notFound.value = false
  try {
    if (!authStore.user?.id || !currentCompanyId.value || !actor.value) {
      detail.value = null
      return
    }
    detail.value = await getBillingRegister(registerId.value, actor.value)
    if (isBuyer.value) {
      const eligible = await listSettlements(actor.value, { status: 'APPROVED', limit: 50 })
      eligibleSettlements.value = (eligible.items ?? []).filter(
        (s) => !s.billing_register_id && s.carrier_company_id === detail.value?.contractor_company_id,
      )
    } else {
      eligibleSettlements.value = []
    }
  } catch (error) {
    detail.value = null
    eligibleSettlements.value = []
    notFound.value = shouldShowNotFound(error)
    apiUnavailable.value = isApiUnavailableError(error)
    if (!notFound.value && !apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('settlements.registerLoadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function loadActor() {
  if (!authStore.user?.id || !currentCompanyId.value) {
    actor.value = null
    return
  }
  const memberships: UserCompanyMembership[] = await getUserCompanies(authStore.user.id)
  actor.value = resolveSettlementActor(currentCompanyId.value, memberships)
}

async function runAction(fn: () => Promise<unknown>, successKey: string) {
  if (acting.value) return
  acting.value = true
  try {
    await fn()
    pushToast('success', t(successKey))
    await loadDetail()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('settlements.actionFailed'))
  } finally {
    acting.value = false
  }
}

async function onIncludeSettlement() {
  if (!actor.value || !selectedSettlementId.value) return
  await runAction(
    () => includeSettlementInRegister(registerId.value, actor.value!, selectedSettlementId.value),
    'settlements.includedInRegister',
  )
  selectedSettlementId.value = ''
}

async function onCalculate() {
  if (!actor.value) return
  await runAction(() => calculateBillingRegister(registerId.value, actor.value!), 'settlements.registerCalculated')
}

async function onApprove() {
  if (!actor.value || !authStore.user?.id) return
  await runAction(
    () => approveBillingRegister(registerId.value, actor.value!, authStore.user!.id),
    'settlements.registerApproved',
  )
}

async function onCreateClosingPackage() {
  if (!actor.value || !detail.value) return
  const pkgNumber = `PKG-${detail.value.register_number}`
  await runAction(
    () => createClosingDocumentPackage(registerId.value, actor.value!, pkgNumber),
    'settlements.closingPackageCreated',
  )
}

watch([currentCompanyId, tenantId, registerId], async () => {
  await loadActor()
  await loadDetail()
}, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="detail?.register_number ?? t('settlements.registerDetail')" />

    <div v-if="loading" role="status" aria-live="polite">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="!tenantId" :title="t('tenant.required')" />
    <EmptyState v-else-if="!currentCompanyId" :title="t('settlements.missingCompany')" />
    <EmptyState v-else-if="!actor" :title="t('settlements.missingActor')" />
    <EmptyState v-else-if="notFound" :title="t('settlements.registerNotFound')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('settlements.registerLoadFailed')" />
    <template v-else-if="detail">
      <Card>
        <dl class="detail-grid">
          <div><dt>{{ t('common.status') }}</dt><dd><Badge :status="detail.status" /></dd></div>
          <div><dt>{{ t('settlements.period') }}</dt><dd>{{ detail.period_from }} — {{ detail.period_to }}</dd></div>
          <div><dt>{{ t('settlements.money.totalWithVat') }}</dt><dd>{{ formatMoney(detail.total_with_vat, detail.currency_code) }}</dd></div>
          <div><dt>{{ t('settlements.money.totalWithoutVat') }}</dt><dd>{{ formatMoney(detail.total_without_vat, detail.currency_code) }}</dd></div>
        </dl>
      </Card>

      <Card>
        <h2>{{ t('settlements.registerItems') }}</h2>
        <EmptyState v-if="!(detail.items?.length)" :title="t('settlements.registerItemsEmpty')" />
        <div v-else class="table-scroll">
          <Table
            :columns="[
              t('settlements.settlementNumber'),
              t('settlements.shipment'),
              t('settlements.money.agreedBase'),
              t('settlements.money.additionalApproved'),
              t('settlements.money.totalWithVat'),
            ]"
          >
            <tr v-for="item in detail.items" :key="item.id">
              <td>
                <NuxtLink v-if="item.settlement_id" :to="`/settlements/${item.settlement_id}`">
                  {{ item.settlement_id.slice(0, 8) }}
                </NuxtLink>
                <span v-else>—</span>
              </td>
              <td>{{ item.shipment_id.slice(0, 8) }}</td>
              <td>{{ formatMoney(item.base_amount, detail.currency_code) }}</td>
              <td>{{ formatMoney(item.extra_charges, detail.currency_code) }}</td>
              <td>{{ formatMoney(item.amount_with_vat, detail.currency_code) }}</td>
            </tr>
          </Table>
        </div>
      </Card>

      <Card v-if="isBuyer && canMutate">
        <h2>{{ t('settlements.addSettlement') }}</h2>
        <EmptyState v-if="!eligibleSettlements.length" :title="t('settlements.noEligibleSettlements')" />
        <form v-else class="inline-form" @submit.prevent="onIncludeSettlement">
          <label>
            {{ t('settlements.eligibleSettlement') }}
            <select v-model="selectedSettlementId" required :disabled="acting">
              <option value="" disabled>{{ t('settlements.selectSettlement') }}</option>
              <option v-for="s in eligibleSettlements" :key="s.id" :value="s.id">
                {{ s.settlement_number }} — {{ formatMoney(s.total_with_vat, s.currency_code) }}
              </option>
            </select>
          </label>
          <button type="submit" class="btn primary" :disabled="acting || !selectedSettlementId">
            {{ acting ? t('common.loading') : t('settlements.include') }}
          </button>
        </form>
      </Card>

      <Card v-if="isBuyer">
        <h2>{{ t('settlements.lifecycle') }}</h2>
        <div class="action-row">
          <button v-if="canCalculate" type="button" class="btn" :disabled="acting" @click="onCalculate">
            {{ t('settlements.calculateRegister') }}
          </button>
          <button v-if="canApprove" type="button" class="btn primary" :disabled="acting" @click="onApprove">
            {{ t('settlements.approveRegister') }}
          </button>
          <button v-if="canCreateDocuments" type="button" class="btn" :disabled="acting" @click="onCreateClosingPackage">
            {{ t('settlements.createClosingPackage') }}
          </button>
        </div>
      </Card>

      <Card v-if="detail.invoices?.length || detail.acts?.length || detail.vat_invoices?.length || detail.upd_documents?.length">
        <h2>{{ t('settlements.closingDocuments') }}</h2>
        <ul class="doc-list">
          <li v-for="inv in detail.invoices ?? []" :key="inv.id">
            {{ t('settlements.invoice') }} {{ inv.invoice_number }} — {{ formatMoney(inv.total_amount, detail.currency_code) }}
          </li>
          <li v-for="act in detail.acts ?? []" :key="act.id">
            {{ t('settlements.act') }} {{ act.act_number }} — {{ formatMoney(act.total_amount, detail.currency_code) }}
          </li>
          <li v-for="vat in detail.vat_invoices ?? []" :key="vat.id">
            {{ t('settlements.vatInvoice') }} {{ vat.vat_invoice_number }} — {{ formatMoney(vat.amount_with_vat, detail.currency_code) }}
          </li>
          <li v-for="upd in detail.upd_documents ?? []" :key="upd.id">
            {{ t('settlements.upd') }} {{ upd.upd_number }} — {{ formatMoney(upd.amount_with_vat, detail.currency_code) }}
          </li>
        </ul>
        <p class="edo-note">{{ t('settlements.edoMockNote') }}</p>
      </Card>
    </template>
  </div>
</template>

<style scoped>
.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 1rem;
  margin: 0;
}
.detail-grid dt {
  font-size: 0.75rem;
  color: var(--color-text-muted, #64748b);
}
.detail-grid dd {
  margin: 0.25rem 0 0;
}
.action-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
.doc-list {
  margin: 0;
  padding-left: 1.25rem;
}
.edo-note {
  margin: 1rem 0 0;
  font-size: 0.875rem;
  color: var(--color-text-muted, #64748b);
}
</style>
