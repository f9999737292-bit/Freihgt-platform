<script setup lang="ts">
import type { UserCompanyMembership } from '~/types/company'
import type { FreightSettlementDetail, SettlementActor } from '~/types/settlement'
import { formatMoney } from '~/types/evaluation'
import { computeSettlementMoneySummary, resolveSettlementActor } from '~/utils/settlement'
import { shouldShowNotFound, isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const {
  getSettlement,
  proposeAccessorial,
  approveAccessorial,
  rejectAccessorial,
  raiseDispute,
  resolveDispute,
  submitForReview,
  approveSettlement,
  markDocumentsReady,
  markReadyForPayment,
  includeInRegister,
} = useSettlementApi()
const { getUserCompanies } = useCompanies()
const { currentCompanyId } = useTenantContext()
const authStore = useAuthStore()
const { pushToast } = useToast()
const { t } = useI18n()

const settlementId = computed(() => String(route.params.id))
const detail = ref<FreightSettlementDetail | null>(null)
const memberships = ref<UserCompanyMembership[]>([])
const actor = ref<SettlementActor | null>(null)
const loading = ref(true)
const acting = ref(false)
const notFound = ref(false)
const apiUnavailable = ref(false)

const money = computed(() => (detail.value ? computeSettlementMoneySummary(detail.value) : null))

const showProposeAccessorial = computed(() =>
  detail.value?.status === 'DRAFT' || detail.value?.status === 'UNDER_REVIEW',
)
const showSubmitForReview = computed(() => detail.value?.status === 'DRAFT')
const showApproveSettlement = computed(() =>
  detail.value?.status === 'UNDER_REVIEW' || detail.value?.status === 'DISPUTED',
)
const showMarkDocumentsReady = computed(() => detail.value?.status === 'APPROVED')
const showMarkReadyForPayment = computed(() => detail.value?.status === 'DOCUMENTS_READY')
const showIncludeInRegister = computed(() =>
  Boolean(detail.value?.billing_register_id == null && detail.value?.status === 'APPROVED'),
)

const accessorialForm = reactive({
  charge_code: '',
  description: '',
  amount: '',
})
const disputeForm = reactive({
  accessorial_id: '',
  reason: '',
})
const resolveForm = reactive({
  disputeId: '',
  resolution_note: '',
})
const registerNumber = ref('')

async function onIncludeInRegister() {
  if (!actor.value || !registerNumber.value.trim()) return
  await runAction(
    () => includeInRegister(settlementId.value, actor.value!, { register_number: registerNumber.value.trim() }),
    'settlements.includedInRegister',
  )
  registerNumber.value = ''
}

async function loadActor() {
  if (!authStore.user?.id) {
    actor.value = null
    return
  }
  memberships.value = await getUserCompanies(authStore.user.id)
  actor.value = resolveSettlementActor(currentCompanyId.value, memberships.value)
}

async function loadDetail() {
  loading.value = true
  notFound.value = false
  apiUnavailable.value = false
  try {
    await loadActor()
    if (!currentCompanyId.value || !actor.value) {
      detail.value = null
      return
    }
    detail.value = await getSettlement(settlementId.value, actor.value)
  } catch (error) {
    detail.value = null
    if (shouldShowNotFound(error)) notFound.value = true
    else {
      apiUnavailable.value = isApiUnavailableError(error)
      if (!apiUnavailable.value) {
        pushToast('error', error instanceof Error ? error.message : t('settlements.loadFailed'))
      }
    }
  } finally {
    loading.value = false
  }
}

async function runAction(action: () => Promise<unknown>, successKey: string) {
  if (!actor.value) return
  acting.value = true
  try {
    const result = await action()
    if (result && typeof result === 'object' && 'accessorials' in result) {
      detail.value = result as FreightSettlementDetail
    } else {
      await loadDetail()
    }
    pushToast('success', t(successKey))
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('settlements.actionFailed'))
  } finally {
    acting.value = false
  }
}

async function onProposeAccessorial() {
  if (!actor.value || !accessorialForm.charge_code.trim() || !accessorialForm.amount) return
  await runAction(
    () =>
      proposeAccessorial(settlementId.value, actor.value!, {
        charge_code: accessorialForm.charge_code.trim(),
        description: accessorialForm.description.trim() || undefined,
        amount: Number(accessorialForm.amount),
      }).then(() => getSettlement(settlementId.value, actor.value!)),
    'settlements.accessorialProposed',
  )
  accessorialForm.charge_code = ''
  accessorialForm.description = ''
  accessorialForm.amount = ''
}

async function onApproveAccessorial(accessorialId: string) {
  if (!actor.value) return
  await runAction(
    () => approveAccessorial(settlementId.value, accessorialId, actor.value!),
    'settlements.accessorialApproved',
  )
}

async function onRejectAccessorial(accessorialId: string) {
  if (!actor.value) return
  await runAction(
    () => rejectAccessorial(settlementId.value, accessorialId, actor.value!),
    'settlements.accessorialRejected',
  )
}

async function onRaiseDispute() {
  if (!actor.value || !disputeForm.reason.trim()) return
  await runAction(
    () =>
      raiseDispute(settlementId.value, actor.value!, {
        reason: disputeForm.reason.trim(),
        accessorial_id: disputeForm.accessorial_id.trim() || undefined,
      }).then(() => getSettlement(settlementId.value, actor.value!)),
    'settlements.disputeRaised',
  )
  disputeForm.reason = ''
  disputeForm.accessorial_id = ''
}

async function onResolveDispute() {
  if (!actor.value || !resolveForm.disputeId || !resolveForm.resolution_note.trim()) return
  await runAction(
    () =>
      resolveDispute(settlementId.value, resolveForm.disputeId, actor.value!, {
        resolution_note: resolveForm.resolution_note.trim(),
      }),
    'settlements.disputeResolved',
  )
  resolveForm.disputeId = ''
  resolveForm.resolution_note = ''
}

watch(settlementId, loadDetail, { immediate: true })
watch(currentCompanyId, loadDetail)
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/settlements">{{ t('settlements.title') }}</NuxtLink>
      <span class="breadcrumbs__sep" aria-hidden="true">/</span>
      <span aria-current="page">{{ detail?.settlement_number ?? settlementId }}</span>
    </nav>

    <PageHeader
      :title="detail?.settlement_number ?? t('settlements.title')"
      :subtitle="detail ? t(`settlements.actor.${actor}`) : undefined"
    >
      <template #actions>
        <Button variant="secondary" @click="$router.push('/settlements')">
          {{ t('settlements.backToList') }}
        </Button>
      </template>
    </PageHeader>

    <div v-if="loading" role="status" aria-live="polite">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="notFound" :title="t('settlements.notFound')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('settlements.loadFailed')" />
    <EmptyState v-else-if="!detail" :title="t('settlements.missingActor')" />

    <template v-else>
      <Card>
        <dl class="detail-grid">
          <div>
            <dt>{{ t('common.status') }}</dt>
            <dd><Badge :status="detail.status" /></dd>
          </div>
          <div>
            <dt>{{ t('settlements.shipment') }}</dt>
            <dd>{{ detail.shipment_id }}</dd>
          </div>
          <div>
            <dt>{{ t('settlements.transportOrder') }}</dt>
            <dd>
              <NuxtLink :to="`/transport-orders/${detail.transport_order_id}`">
                {{ detail.transport_order_id }}
              </NuxtLink>
            </dd>
          </div>
          <div>
            <dt>{{ t('settlements.register') }}</dt>
            <dd>
              <NuxtLink
                v-if="detail.billing_register_id"
                :to="`/settlements?register=${detail.billing_register_id}`"
              >
                {{ detail.billing_register_id }}
              </NuxtLink>
              <span v-else>{{ t('settlements.registerNotLinked') }}</span>
            </dd>
          </div>
          <div>
            <dt>{{ t('settlements.documentsStatus') }}</dt>
            <dd>{{ detail.status === 'DOCUMENTS_READY' || detail.status === 'READY_FOR_PAYMENT' ? t('settlements.documentsReady') : t('settlements.documentsPending') }}</dd>
          </div>
        </dl>
      </Card>

      <Card v-if="money">
        <template #header><h3>{{ t('settlements.reconciliation') }}</h3></template>
        <dl class="money-grid">
          <div>
            <dt>{{ t('settlements.money.agreedBase') }}</dt>
            <dd>{{ formatMoney(money.agreedBase, money.currencyCode) }}</dd>
          </div>
          <div>
            <dt>{{ t('settlements.money.additionalProposed') }}</dt>
            <dd>{{ formatMoney(money.additionalProposed, money.currencyCode) }}</dd>
          </div>
          <div>
            <dt>{{ t('settlements.money.additionalApproved') }}</dt>
            <dd>{{ formatMoney(money.additionalApproved, money.currencyCode) }}</dd>
          </div>
          <div>
            <dt>{{ t('settlements.money.additionalDisputed') }}</dt>
            <dd>{{ formatMoney(money.additionalDisputed, money.currencyCode) }}</dd>
          </div>
          <div>
            <dt>{{ t('settlements.money.totalWithoutVat') }}</dt>
            <dd>{{ formatMoney(money.totalWithoutVat, money.currencyCode) }}</dd>
          </div>
          <div>
            <dt>{{ t('settlements.money.vat') }}</dt>
            <dd>{{ formatMoney(money.vatAmount, money.currencyCode) }}</dd>
          </div>
          <div class="money-grid__total">
            <dt>{{ t('settlements.money.totalWithVat') }}</dt>
            <dd>{{ formatMoney(money.totalWithVat, money.currencyCode) }}</dd>
          </div>
        </dl>
      </Card>

      <Card>
        <template #header><h3>{{ t('settlements.accessorials') }}</h3></template>
        <EmptyState v-if="detail.accessorials.length === 0" :title="t('settlements.noAccessorials')" />
        <div v-else class="table-scroll">
          <Table
            :columns="[
              t('settlements.chargeCode'),
              t('settlements.description'),
              t('common.status'),
              t('settlements.amount'),
              t('common.actions'),
            ]"
          >
            <tr v-for="item in detail.accessorials" :key="item.id">
              <td>{{ item.charge_code }}</td>
              <td>{{ item.description ?? '—' }}</td>
              <td><Badge :status="item.status" /></td>
              <td>{{ formatMoney(item.amount, item.currency_code) }}</td>
              <td class="actions-cell">
                <Button
                  v-if="item.status === 'PROPOSED'"
                  size="sm"
                  variant="secondary"
                  :disabled="acting"
                  @click="onApproveAccessorial(item.id)"
                >
                  {{ t('settlements.approve') }}
                </Button>
                <Button
                  v-if="item.status === 'PROPOSED'"
                  size="sm"
                  variant="secondary"
                  :disabled="acting"
                  @click="onRejectAccessorial(item.id)"
                >
                  {{ t('settlements.reject') }}
                </Button>
              </td>
            </tr>
          </Table>
        </div>

        <form v-if="showProposeAccessorial" class="inline-form" @submit.prevent="onProposeAccessorial">
          <h4>{{ t('settlements.proposeAccessorial') }}</h4>
          <div class="form-row">
            <Input v-model="accessorialForm.charge_code" :label="t('settlements.chargeCode')" required />
            <Input v-model="accessorialForm.amount" :label="t('settlements.amount')" type="number" required />
          </div>
          <Input v-model="accessorialForm.description" :label="t('settlements.description')" />
          <Button type="submit" :disabled="acting">{{ t('settlements.propose') }}</Button>
        </form>
      </Card>

      <Card>
        <template #header><h3>{{ t('settlements.disputes') }}</h3></template>
        <EmptyState v-if="detail.disputes.length === 0" :title="t('settlements.noDisputes')" />
        <div v-else class="table-scroll">
          <Table
            :columns="[
              t('settlements.reason'),
              t('common.status'),
              t('settlements.accessorial'),
              t('common.actions'),
            ]"
          >
            <tr v-for="item in detail.disputes" :key="item.id">
              <td>{{ item.reason }}</td>
              <td><Badge :status="item.status" /></td>
              <td>{{ item.accessorial_id ?? '—' }}</td>
              <td>
                <Button
                  v-if="item.status === 'OPEN'"
                  size="sm"
                  variant="secondary"
                  :disabled="acting"
                  @click="resolveForm.disputeId = item.id"
                >
                  {{ t('settlements.resolve') }}
                </Button>
              </td>
            </tr>
          </Table>
        </div>

        <form class="inline-form" @submit.prevent="onRaiseDispute">
          <h4>{{ t('settlements.raiseDispute') }}</h4>
          <Input v-model="disputeForm.accessorial_id" :label="t('settlements.accessorialIdOptional')" />
          <Input v-model="disputeForm.reason" :label="t('settlements.reason')" required />
          <Button type="submit" :disabled="acting">{{ t('settlements.raise') }}</Button>
        </form>

        <form v-if="resolveForm.disputeId" class="inline-form" @submit.prevent="onResolveDispute">
          <h4>{{ t('settlements.resolveDispute') }}</h4>
          <Input v-model="resolveForm.resolution_note" :label="t('settlements.resolutionNote')" required />
          <div class="form-actions">
            <Button type="submit" :disabled="acting">{{ t('settlements.resolve') }}</Button>
            <Button type="button" variant="secondary" @click="resolveForm.disputeId = ''">
              {{ t('common.cancel') }}
            </Button>
          </div>
        </form>
      </Card>

      <Card>
        <template #header><h3>{{ t('settlements.lifecycle') }}</h3></template>
        <div class="lifecycle-actions">
          <Button
            v-if="showSubmitForReview"
            :disabled="acting"
            @click="runAction(() => submitForReview(settlementId, actor!), 'settlements.submittedForReview')"
          >
            {{ t('settlements.submitForReview') }}
          </Button>
          <Button
            v-if="showApproveSettlement"
            :disabled="acting"
            @click="runAction(() => approveSettlement(settlementId, actor!), 'settlements.settlementApproved')"
          >
            {{ t('settlements.approveSettlement') }}
          </Button>
          <Button
            v-if="showMarkDocumentsReady"
            :disabled="acting"
            @click="runAction(() => markDocumentsReady(settlementId, actor!), 'settlements.documentsMarkedReady')"
          >
            {{ t('settlements.markDocumentsReady') }}
          </Button>
          <Button
            v-if="showMarkReadyForPayment"
            :disabled="acting"
            @click="runAction(() => markReadyForPayment(settlementId, actor!), 'settlements.markedReadyForPayment')"
          >
            {{ t('settlements.markReadyForPayment') }}
          </Button>
        </div>

        <form v-if="showIncludeInRegister" class="inline-form" @submit.prevent="onIncludeInRegister">
          <h4>{{ t('settlements.includeInRegister') }}</h4>
          <Input v-model="registerNumber" :label="t('settlements.registerNumber')" required />
          <Button type="submit" :disabled="acting || !registerNumber.trim()">{{ t('settlements.include') }}</Button>
        </form>
      </Card>
    </template>
  </div>
</template>

<style scoped>
.detail-grid,
.money-grid {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.detail-grid dt,
.money-grid dt {
  font-weight: 600;
  margin-bottom: 0.25rem;
  color: var(--color-text-muted, #64748b);
  font-size: 0.875rem;
}

.money-grid__total dd {
  font-weight: 700;
  font-size: 1.125rem;
}

.actions-cell {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.inline-form {
  margin-top: 1.25rem;
  padding-top: 1.25rem;
  border-top: 1px solid var(--color-border);
  display: grid;
  gap: 0.75rem;
}

.inline-form h4 {
  margin: 0;
  font-size: 0.9375rem;
}

.form-row {
  display: grid;
  gap: 0.75rem;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
}

.lifecycle-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.form-actions {
  display: flex;
  gap: 0.5rem;
}
</style>
