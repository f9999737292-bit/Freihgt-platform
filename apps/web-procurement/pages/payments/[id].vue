<script setup lang="ts">
import type { UserCompanyMembership } from '~/types/company'
import type {
  PaymentActor,
  PaymentAllocationRecord,
  PaymentAuditEventRecord,
  PaymentObligationRecord,
  PaymentRecord,
} from '~/types/payment'
import {
  canShowAllocateAction,
  canShowReconcileAction,
  canShowVoidAllocationAction,
  canShowVoidPaymentAction,
  formatPaymentMoney,
  isAllocationActive,
  isNonEmptyReason,
  isPositiveAmountInput,
  paymentStatusLabelKey,
  resolvePaymentActor,
} from '~/utils/payment'
import {
  appendPageItems,
  billingRegisterLinkFromAllocation,
  fetchAllocationsPage,
  fetchAuditPage,
  fetchEligiblePage,
  fetchPaymentDetailInitial,
  obligationLabelFromAllocation,
  PAYMENT_DETAIL_PAGE_SIZE,
} from '~/utils/paymentWorkspaceFlow'
import { ApiError } from '~/utils/apiClient'
import { formatDateTime } from '~/utils/format'
import { isApiUnavailableError, shouldShowNotFound } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const paymentsApi = usePaymentsApi()
const {
  listAllocations,
  listEligibleObligations,
  allocate,
  voidAllocation,
  voidPayment,
  reconcilePayment,
} = paymentsApi
const { getUserCompanies } = useCompanies()
const { currentCompanyId } = useTenantContext()
const { canWritePayments } = usePermissions()
const authStore = useAuthStore()
const { pushToast } = useToast()
const { t } = useI18n()

const paymentId = computed(() => String(route.params.id))
const payment = ref<PaymentRecord | null>(null)
const allocations = ref<PaymentAllocationRecord[]>([])
const allocationsTotal = ref(0)
const auditEvents = ref<PaymentAuditEventRecord[]>([])
const auditTotal = ref(0)
const memberships = ref<UserCompanyMembership[]>([])
const actor = ref<PaymentActor | null>(null)
const loading = ref(true)
const loadingAllocationsMore = ref(false)
const loadingAuditMore = ref(false)
const loadingEligibleMore = ref(false)
const acting = ref(false)
const notFound = ref(false)
const apiUnavailable = ref(false)

const showAllocateModal = ref(false)
const showVoidAllocationModal = ref(false)
const showVoidPaymentModal = ref(false)
const showReconcileModal = ref(false)
const selectedAllocationId = ref('')
const eligibleObligations = ref<PaymentObligationRecord[]>([])
const eligibleTotal = ref(0)
const allocateForm = reactive({ obligationId: '', amount: '' })
const voidReason = ref('')

const canWrite = computed(() => canWritePayments() && !!actor.value)
const showAllocate = computed(() => canWrite.value && canShowAllocateAction(payment.value))
const showVoidPayment = computed(() => canWrite.value && canShowVoidPaymentAction(payment.value))
const showReconcile = computed(() => canWrite.value && canShowReconcileAction(payment.value))
const hasMoreAllocations = computed(() => allocations.value.length < allocationsTotal.value)
const hasMoreAudit = computed(() => auditEvents.value.length < auditTotal.value)
const hasMoreEligible = computed(() => eligibleObligations.value.length < eligibleTotal.value)

async function loadMemberships() {
  if (!authStore.user?.id) {
    memberships.value = []
    actor.value = null
    return
  }
  memberships.value = await getUserCompanies(authStore.user.id)
  actor.value = resolvePaymentActor(currentCompanyId.value, memberships.value)
}

async function loadDetail() {
  loading.value = true
  apiUnavailable.value = false
  notFound.value = false
  try {
    await loadMemberships()
    if (!currentCompanyId.value || !actor.value) {
      payment.value = null
      return
    }
    const snapshot = await fetchPaymentDetailInitial(paymentsApi, paymentId.value, false)
    payment.value = snapshot.payment
    allocations.value = snapshot.allocations
    allocationsTotal.value = snapshot.allocationsTotal
    auditEvents.value = snapshot.auditEvents
    auditTotal.value = snapshot.auditTotal
    if (showAllocate.value) {
      const eligiblePage = await listEligibleObligations(paymentId.value, {
        limit: PAYMENT_DETAIL_PAGE_SIZE,
        offset: 0,
      })
      eligibleObligations.value = eligiblePage.items
      eligibleTotal.value = eligiblePage.total
    } else {
      eligibleObligations.value = []
      eligibleTotal.value = 0
    }
  } catch (error) {
    payment.value = null
    allocations.value = []
    auditEvents.value = []
    notFound.value = shouldShowNotFound(error)
    apiUnavailable.value = isApiUnavailableError(error)
    if (!notFound.value && !apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('payments.detailLoadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function loadMoreAllocations() {
  if (loadingAllocationsMore.value || !hasMoreAllocations.value) return
  loadingAllocationsMore.value = true
  try {
    const page = await fetchAllocationsPage(paymentsApi, paymentId.value, allocations.value.length)
    allocations.value = appendPageItems(allocations.value, page)
    allocationsTotal.value = page.total
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    loadingAllocationsMore.value = false
  }
}

async function loadMoreAudit() {
  if (loadingAuditMore.value || !hasMoreAudit.value) return
  loadingAuditMore.value = true
  try {
    const page = await fetchAuditPage(paymentsApi, paymentId.value, auditEvents.value.length)
    auditEvents.value = appendPageItems(auditEvents.value, page)
    auditTotal.value = page.total
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    loadingAuditMore.value = false
  }
}

async function loadMoreEligible() {
  if (loadingEligibleMore.value || !hasMoreEligible.value) return
  loadingEligibleMore.value = true
  try {
    const page = await fetchEligiblePage(paymentsApi, paymentId.value, eligibleObligations.value.length)
    eligibleObligations.value = appendPageItems(eligibleObligations.value, page)
    eligibleTotal.value = page.total
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    loadingEligibleMore.value = false
  }
}

async function runAction(action: () => Promise<unknown>, successKey: string) {
  if (acting.value) return
  acting.value = true
  try {
    await action()
    pushToast('success', t(successKey))
    await loadDetail()
  } catch (error) {
    if (error instanceof ApiError && error.status === 409) {
      pushToast('error', error.message)
      await loadDetail()
      return
    }
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    acting.value = false
    showAllocateModal.value = false
    showVoidAllocationModal.value = false
    showVoidPaymentModal.value = false
    showReconcileModal.value = false
    voidReason.value = ''
    allocateForm.obligationId = ''
    allocateForm.amount = ''
  }
}

function openAllocateModal() {
  allocateForm.obligationId = eligibleObligations.value[0]?.id ?? ''
  allocateForm.amount = payment.value?.unallocated_amount ?? ''
  showAllocateModal.value = true
}

function openVoidAllocationModal(allocationId: string) {
  selectedAllocationId.value = allocationId
  voidReason.value = ''
  showVoidAllocationModal.value = true
}

function submitAllocate() {
  if (!allocateForm.obligationId) {
    pushToast('error', t('payments.validation.obligationRequired'))
    return
  }
  if (!isPositiveAmountInput(allocateForm.amount)) {
    pushToast('error', t('payments.validation.amountPositive'))
    return
  }
  runAction(
    () => allocate(paymentId.value, allocateForm.obligationId, allocateForm.amount.trim()),
    'payments.success.allocated',
  )
}

function submitVoidAllocation() {
  if (!isNonEmptyReason(voidReason.value)) {
    pushToast('error', t('payments.validation.reasonRequired'))
    return
  }
  runAction(
    () => voidAllocation(selectedAllocationId.value, voidReason.value.trim()),
    'payments.success.voidedAllocation',
  )
}

function submitVoidPayment() {
  if (!isNonEmptyReason(voidReason.value)) {
    pushToast('error', t('payments.validation.reasonRequired'))
    return
  }
  runAction(
    () => voidPayment(paymentId.value, voidReason.value.trim()),
    'payments.success.voidedPayment',
  )
}

function submitReconcile() {
  runAction(
    () => reconcilePayment(paymentId.value),
    'payments.success.reconciled',
  )
}

function auditLabel(eventType: string) {
  const key = `payments.eventType.${eventType}`
  const translated = t(key)
  return translated === key ? eventType : translated
}

watch(currentCompanyId, loadDetail, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="payment?.payment_number || t('payments.detailTitle')">
      <template #actions>
        <NuxtLink to="/payments">{{ t('common.back') }}</NuxtLink>
      </template>
    </PageHeader>

    <div v-if="loading" role="status" aria-live="polite">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="notFound" :title="t('payments.notFound')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('payments.detailLoadFailed')" />
    <EmptyState v-else-if="!payment" :title="t('payments.missingActor')" />

    <template v-else>
      <div class="actions-row">
        <Button v-if="showAllocate" :disabled="acting" @click="openAllocateModal">
          {{ t('payments.actions.allocate') }}
        </Button>
        <Button v-if="showReconcile" :disabled="acting" @click="showReconcileModal = true">
          {{ t('payments.actions.reconcile') }}
        </Button>
        <Button v-if="showVoidPayment" variant="danger" :disabled="acting" @click="showVoidPaymentModal = true">
          {{ t('payments.actions.voidPayment') }}
        </Button>
      </div>

      <Card>
        <h2>{{ t('payments.summary') }}</h2>
        <dl class="detail-grid">
          <dt>{{ t('common.status') }}</dt>
          <dd><Badge :status="t(paymentStatusLabelKey(payment.status))" /></dd>
          <dt>{{ t('payments.paymentNumber') }}</dt>
          <dd>{{ payment.payment_number }}</dd>
          <dt v-if="payment.external_id">{{ t('payments.externalId') }}</dt>
          <dd v-if="payment.external_id">{{ payment.external_id }}</dd>
          <dt v-if="payment.reference">{{ t('payments.reference') }}</dt>
          <dd v-if="payment.reference">{{ payment.reference }}</dd>
          <dt>{{ t('payments.paymentDate') }}</dt>
          <dd>{{ payment.payment_date }}</dd>
          <dt v-if="payment.reconciled_at">{{ t('payments.reconciledAt') }}</dt>
          <dd v-if="payment.reconciled_at">{{ formatDateTime(payment.reconciled_at) }}</dd>
        </dl>
      </Card>

      <Card>
        <h2>{{ t('payments.financial') }}</h2>
        <dl class="detail-grid">
          <dt>{{ t('payments.amount') }}</dt>
          <dd>{{ formatPaymentMoney(payment.amount, payment.currency_code) }}</dd>
          <dt>{{ t('payments.allocated') }}</dt>
          <dd>{{ formatPaymentMoney(payment.allocated_amount, payment.currency_code) }}</dd>
          <dt>{{ t('payments.unallocated') }}</dt>
          <dd>{{ formatPaymentMoney(payment.unallocated_amount, payment.currency_code) }}</dd>
        </dl>
      </Card>

      <Card>
        <h2>{{ t('payments.allocations') }}</h2>
        <EmptyState v-if="allocations.length === 0" :title="t('payments.empty')" />
        <div v-else class="table-scroll">
          <Table
            :columns="[
              t('payments.obligations'),
              t('payments.amount'),
              t('common.status'),
              t('payments.createdAt'),
              t('common.actions'),
            ]"
          >
            <tr v-for="allocation in allocations" :key="allocation.id">
              <td>{{ obligationLabelFromAllocation(allocation) }}</td>
              <td>{{ formatPaymentMoney(allocation.allocated_amount, allocation.currency_code) }}</td>
              <td>
                <Badge
                  :status="isAllocationActive(allocation) ? t('payments.activeAllocation') : t('payments.voidedAllocation')"
                />
              </td>
              <td>{{ formatDateTime(allocation.created_at) }}</td>
              <td>
                <Button
                  v-if="canWrite && canShowVoidAllocationAction(payment, allocation)"
                  variant="secondary"
                  :disabled="acting"
                  @click="openVoidAllocationModal(allocation.id)"
                >
                  {{ t('payments.actions.voidAllocation') }}
                </Button>
                <div v-if="allocation.voided_at" class="muted">
                  {{ t('payments.voidedAt') }}: {{ formatDateTime(allocation.voided_at) }}
                  <span v-if="allocation.void_reason"> — {{ allocation.void_reason }}</span>
                </div>
              </td>
            </tr>
          </Table>
        </div>
        <div v-if="hasMoreAllocations" class="load-more-row">
          <Button variant="secondary" :disabled="loadingAllocationsMore" @click="loadMoreAllocations">
            {{ loadingAllocationsMore ? t('common.loading') : t('payments.loadMore') }}
          </Button>
        </div>
      </Card>

      <Card>
        <h2>{{ t('payments.audit') }}</h2>
        <EmptyState v-if="auditEvents.length === 0" :title="t('payments.empty')" />
        <ul v-else class="audit-list">
          <li v-for="event in auditEvents" :key="event.id">
            <strong>{{ auditLabel(event.event_type) }}</strong>
            <span class="muted">{{ formatDateTime(event.created_at) }}</span>
          </li>
        </ul>
        <div v-if="hasMoreAudit" class="load-more-row">
          <Button variant="secondary" :disabled="loadingAuditMore" @click="loadMoreAudit">
            {{ loadingAuditMore ? t('common.loading') : t('payments.loadMore') }}
          </Button>
        </div>
      </Card>

      <Card v-if="allocations.some((a) => billingRegisterLinkFromAllocation(a))">
        <h2>{{ t('payments.relatedDocuments') }}</h2>
        <ul>
          <li v-for="allocation in allocations" :key="`link-${allocation.id}`">
            <NuxtLink v-if="billingRegisterLinkFromAllocation(allocation)" :to="billingRegisterLinkFromAllocation(allocation)!">
              {{ t('payments.billingRegister') }} — {{ obligationLabelFromAllocation(allocation) }}
            </NuxtLink>
          </li>
        </ul>
      </Card>
    </template>

    <Modal :open="showAllocateModal" :title="t('payments.allocateModal.title')" @close="showAllocateModal = false">
      <Select
        v-model="allocateForm.obligationId"
        :label="t('payments.allocateModal.obligation')"
        :options="eligibleObligations.map((o) => ({
          label: `${o.obligation_number} (${formatPaymentMoney(o.outstanding_amount, o.currency_code)})`,
          value: o.id,
        }))"
      />
      <Input v-model="allocateForm.amount" :label="t('payments.allocateModal.amount')" />
      <div v-if="hasMoreEligible" class="load-more-row">
        <Button variant="secondary" :disabled="loadingEligibleMore" @click="loadMoreEligible">
          {{ loadingEligibleMore ? t('common.loading') : t('payments.loadMoreEligible') }}
        </Button>
      </div>
      <template #footer>
        <Button variant="secondary" @click="showAllocateModal = false">{{ t('common.cancel') }}</Button>
        <Button :disabled="acting" @click="submitAllocate">{{ t('payments.allocateModal.submit') }}</Button>
      </template>
    </Modal>

    <Modal :open="showVoidAllocationModal" :title="t('payments.voidAllocationModal.title')" @close="showVoidAllocationModal = false">
      <Input v-model="voidReason" :label="t('payments.voidAllocationModal.reason')" />
      <template #footer>
        <Button variant="secondary" @click="showVoidAllocationModal = false">{{ t('common.cancel') }}</Button>
        <Button variant="danger" :disabled="acting" @click="submitVoidAllocation">{{ t('payments.voidAllocationModal.submit') }}</Button>
      </template>
    </Modal>

    <Modal :open="showVoidPaymentModal" :title="t('payments.voidPaymentModal.title')" @close="showVoidPaymentModal = false">
      <Input v-model="voidReason" :label="t('payments.voidPaymentModal.reason')" />
      <template #footer>
        <Button variant="secondary" @click="showVoidPaymentModal = false">{{ t('common.cancel') }}</Button>
        <Button variant="danger" :disabled="acting" @click="submitVoidPayment">{{ t('payments.voidPaymentModal.submit') }}</Button>
      </template>
    </Modal>

    <Modal :open="showReconcileModal" :title="t('payments.reconcileModal.title')" @close="showReconcileModal = false">
      <p>{{ t('payments.reconcileModal.message') }}</p>
      <dl v-if="payment" class="detail-grid">
        <dt>{{ t('payments.amount') }}</dt>
        <dd>{{ formatPaymentMoney(payment.amount, payment.currency_code) }}</dd>
        <dt>{{ t('payments.allocated') }}</dt>
        <dd>{{ formatPaymentMoney(payment.allocated_amount, payment.currency_code) }}</dd>
        <dt>{{ t('payments.unallocated') }}</dt>
        <dd>{{ formatPaymentMoney(payment.unallocated_amount, payment.currency_code) }}</dd>
      </dl>
      <template #footer>
        <Button variant="secondary" @click="showReconcileModal = false">{{ t('common.cancel') }}</Button>
        <Button :disabled="acting" @click="submitReconcile">{{ t('payments.reconcileModal.confirm') }}</Button>
      </template>
    </Modal>
  </div>
</template>

<style scoped>
.actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}
.detail-grid {
  display: grid;
  grid-template-columns: minmax(160px, 220px) 1fr;
  gap: 0.5rem 1rem;
}
.detail-grid dt {
  color: var(--color-text-muted);
}
.audit-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  gap: 0.75rem;
}
.audit-list li {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}
.load-more-row {
  margin-top: 0.75rem;
}
.muted {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}
</style>
