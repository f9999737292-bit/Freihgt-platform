<script setup lang="ts">
import type { UserCompanyMembership } from '~/types/company'
import type { CarrierTransportOrderItem, OrderExecutionView } from '~/types/orderExecution'
import { formatMoney } from '~/types/evaluation'
import {
  filterCarrierMemberships,
  membershipSelectOptions,
  selectDefaultCarrierCompany,
} from '~/utils/companyMembership'
import { isApiUnavailableError, shouldShowNotFound } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const {
  getOrderExecution,
  executeTransportOrder,
  startExecution,
  assignDriver,
  assignVehicle,
} = useOrderExecutionApi()
const { getUserCompanies } = useCompanies()
const authStore = useAuthStore()
const { pushToast } = useToast()
const { t } = useI18n()

const orderId = computed(() => String(route.params.id))
const memberships = ref<UserCompanyMembership[]>([])
const selectedCarrierId = ref('')
const execution = ref<OrderExecutionView | null>(null)
const loading = ref(true)
const acting = ref(false)
const notFound = ref(false)
const apiUnavailable = ref(false)
const driverId = ref('')
const vehicleId = ref('')

function formatWhen(value?: string) {
  if (!value) return '—'
  try {
    return new Intl.DateTimeFormat(undefined, {
      day: '2-digit',
      month: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(value))
  } catch {
    return value
  }
}

const carrierOptions = computed(() => membershipSelectOptions(filterCarrierMemberships(memberships.value)))

async function loadExecution() {
  loading.value = true
  notFound.value = false
  apiUnavailable.value = false
  try {
    if (!selectedCarrierId.value) return
    execution.value = await getOrderExecution(orderId.value, selectedCarrierId.value, 'CARRIER')
  } catch (error) {
    execution.value = null
    if (shouldShowNotFound(error)) notFound.value = true
    else apiUnavailable.value = isApiUnavailableError(error)
  } finally {
    loading.value = false
  }
}

async function handleExecute() {
  if (!selectedCarrierId.value || !confirm(t('orderExecution.executeConfirm'))) return
  acting.value = true
  try {
    const result = await executeTransportOrder(
      orderId.value,
      selectedCarrierId.value,
      `SHP-${orderId.value.slice(0, 8)}`,
    )
    pushToast('success', result.created ? t('orderExecution.executeSuccess') : t('orderExecution.executeExisting'))
    await loadExecution()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    acting.value = false
  }
}

async function handleAssignDriver() {
  if (!execution.value?.shipment?.id || !driverId.value.trim()) return
  acting.value = true
  try {
    await assignDriver(execution.value.shipment.id, driverId.value.trim())
    await loadExecution()
    pushToast('success', t('orderExecution.assignDriver'))
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    acting.value = false
  }
}

async function handleAssignVehicle() {
  if (!execution.value?.shipment?.id || !vehicleId.value.trim()) return
  acting.value = true
  try {
    await assignVehicle(execution.value.shipment.id, vehicleId.value.trim())
    await loadExecution()
    pushToast('success', t('orderExecution.assignVehicle'))
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    acting.value = false
  }
}

async function handleStart() {
  if (!selectedCarrierId.value || !confirm(t('orderExecution.startConfirm'))) return
  acting.value = true
  try {
    await startExecution(orderId.value, selectedCarrierId.value)
    await loadExecution()
    pushToast('success', t('orderExecution.startSuccess'))
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    acting.value = false
  }
}

onMounted(async () => {
  memberships.value = await getUserCompanies(authStore.user?.id || '')
  selectedCarrierId.value = selectDefaultCarrierCompany(filterCarrierMemberships(memberships.value))
  await loadExecution()
})

watch(selectedCarrierId, loadExecution)
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/carrier/transport-orders">{{ t('orderExecution.carrierTitle') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ execution?.transport_order_number || orderId }}</span>
    </nav>

    <PageHeader :title="t('orderExecution.title')" :subtitle="execution?.transport_order_number">
      <template #actions>
        <Button variant="secondary" @click="$router.push('/carrier/transport-orders')">{{ t('orderExecution.backToList') }}</Button>
      </template>
    </PageHeader>

    <Card>
      <label class="field-label" for="carrier-select">{{ t('orderExecution.winningCarrier') }}</label>
      <select id="carrier-select" v-model="selectedCarrierId" class="input">
        <option v-for="opt in carrierOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
      </select>
    </Card>

    <div v-if="loading" role="status">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="notFound" :title="t('orderExecution.notFound')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('orderExecution.loadFailed')" />

    <template v-else-if="execution">
      <Card>
        <dl class="detail-grid">
          <div><dt>{{ t('orderExecution.orderStatus') }}</dt><dd><Badge :status="execution.transport_order_status" /></dd></div>
          <div><dt>{{ t('orderExecution.awardAmount') }}</dt><dd>{{ formatMoney(execution.provenance.amount, execution.provenance.currency_code) }}</dd></div>
          <div><dt>{{ t('orderExecution.readiness') }}</dt><dd>{{ execution.readiness.ready_to_start ? t('orderExecution.readyToStart') : execution.readiness.missing_requirements.join(', ') }}</dd></div>
        </dl>
        <div class="actions-row">
          <Button v-if="!execution.shipment" :loading="acting" @click="handleExecute">{{ t('orderExecution.execute') }}</Button>
          <Button v-if="execution.readiness.ready_to_start" :loading="acting" @click="handleStart">{{ t('orderExecution.startExecution') }}</Button>
        </div>
      </Card>

      <Card v-if="execution.shipment">
        <template #header><h3>{{ t('orderExecution.shipment') }}</h3></template>
        <p>{{ execution.shipment.shipment_number }} — <Badge :status="execution.shipment.status" /></p>
        <dl class="detail-grid">
          <div><dt>{{ t('orderExecution.plannedPickup') }}</dt><dd>{{ formatWhen(execution.shipment.planned_pickup_at) }}</dd></div>
          <div><dt>{{ t('orderExecution.plannedDelivery') }}</dt><dd>{{ formatWhen(execution.shipment.planned_delivery_at) }}</dd></div>
          <div><dt>{{ t('orderExecution.actualPickup') }}</dt><dd>{{ formatWhen(execution.shipment.actual_pickup_at) }}</dd></div>
          <div><dt>{{ t('orderExecution.actualDelivery') }}</dt><dd>{{ formatWhen(execution.shipment.actual_delivery_at) }}</dd></div>
        </dl>
        <div class="form-row">
          <label for="driver-id">{{ t('orderExecution.driverId') }}</label>
          <input id="driver-id" v-model="driverId" class="input" type="text" />
          <Button size="sm" :loading="acting" @click="handleAssignDriver">{{ t('orderExecution.assignDriver') }}</Button>
        </div>
        <div class="form-row">
          <label for="vehicle-id">{{ t('orderExecution.vehicleId') }}</label>
          <input id="vehicle-id" v-model="vehicleId" class="input" type="text" />
          <Button size="sm" :loading="acting" @click="handleAssignVehicle">{{ t('orderExecution.assignVehicle') }}</Button>
        </div>
      </Card>

      <Card v-if="execution.sla_signals?.length">
        <template #header><h3>{{ t('orderExecution.slaSignals') }}</h3></template>
        <ul class="signal-list">
          <li v-for="signal in execution.sla_signals" :key="`${signal.code}-${signal.at}`">
            <Badge :status="signal.severity" /> {{ signal.message }}
          </li>
        </ul>
      </Card>

      <Card v-if="execution.milestones?.length">
        <template #header><h3>{{ t('orderExecution.milestones') }}</h3></template>
        <ol class="timeline">
          <li v-for="m in execution.milestones" :key="m.id">
            <strong>{{ m.to_status }}</strong>
            <span class="muted">{{ formatWhen(m.occurred_at) }}</span>
          </li>
        </ol>
      </Card>

      <Card v-if="execution.pod_documents?.length">
        <template #header><h3>{{ t('orderExecution.podDocuments') }}</h3></template>
        <ul class="pod-list">
          <li v-for="doc in execution.pod_documents" :key="doc.id">
            {{ doc.document_number }} — <Badge :status="doc.status" />
          </li>
        </ul>
      </Card>
    </template>
  </div>
</template>

<style scoped>
.detail-grid { display: grid; gap: 1rem; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
.detail-grid dt { font-weight: 600; margin-bottom: 0.25rem; }
.actions-row, .form-row { display: flex; flex-wrap: wrap; gap: 0.75rem; align-items: end; margin-top: 1rem; }
.form-row label { min-width: 120px; }
.signal-list, .pod-list, .timeline { margin: 0; padding-left: 1.25rem; }
.timeline li, .signal-list li, .pod-list li { margin-bottom: 0.5rem; }
.muted { color: var(--color-muted, #666); }
</style>
