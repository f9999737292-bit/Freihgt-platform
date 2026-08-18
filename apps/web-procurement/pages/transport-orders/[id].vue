<script setup lang="ts">
import type { OrderExecutionView } from '~/types/orderExecution'
import { formatMoney } from '~/types/evaluation'
import { shouldShowNotFound, isApiUnavailableError } from '~/utils/apiError'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { getOrderExecution } = useOrderExecutionApi()
const { currentCompanyId } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const orderId = computed(() => String(route.params.id))
const execution = ref<OrderExecutionView | null>(null)
const loading = ref(true)
const notFound = ref(false)
const apiUnavailable = ref(false)

function formatWhen(value?: string) {
  if (!value) return '—'
  try {
    return new Intl.DateTimeFormat(undefined, {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(value))
  } catch {
    return value
  }
}

async function loadExecution() {
  loading.value = true
  notFound.value = false
  apiUnavailable.value = false
  try {
    if (!currentCompanyId.value) throw new Error(t('common.error'))
    execution.value = await getOrderExecution(orderId.value, currentCompanyId.value, 'BUYER')
  } catch (error) {
    execution.value = null
    if (shouldShowNotFound(error)) notFound.value = true
    else {
      apiUnavailable.value = isApiUnavailableError(error)
      if (!apiUnavailable.value) pushToast('error', error instanceof Error ? error.message : t('orderExecution.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

watch(orderId, loadExecution, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/transport-orders">{{ t('orderExecution.buyerTitle') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <NuxtLink v-if="execution" :to="`/tenders/${execution.provenance.rfx_event_id}/evaluation`">
        {{ t('orderExecution.backToTender') }}
      </NuxtLink>
      <span v-if="execution" class="breadcrumbs__sep">/</span>
      <span>{{ t('orderExecution.title') }}</span>
    </nav>

    <PageHeader :title="t('orderExecution.title')" :subtitle="execution?.transport_order_number">
      <template #actions>
        <Button variant="secondary" @click="$router.push('/transport-orders')">{{ t('orderExecution.backToList') }}</Button>
      </template>
    </PageHeader>

    <div v-if="loading" role="status">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="notFound" :title="t('orderExecution.notFound')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('orderExecution.loadFailed')" />

    <template v-else-if="execution">
      <Card>
        <dl class="detail-grid">
          <div><dt>{{ t('orderExecution.orderNumber') }}</dt><dd>{{ execution.transport_order_number }}</dd></div>
          <div><dt>{{ t('orderExecution.orderStatus') }}</dt><dd><Badge :status="execution.transport_order_status" /></dd></div>
          <div><dt>{{ t('orderExecution.sourceTender') }}</dt><dd>{{ execution.provenance.rfx_event_id }}</dd></div>
          <div><dt>{{ t('orderExecution.awardAmount') }}</dt><dd>{{ formatMoney(execution.provenance.amount, execution.provenance.currency_code) }}</dd></div>
        </dl>
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
        <p class="muted">{{ t('orderExecution.readiness') }}:
          {{ execution.readiness.ready_to_start ? t('orderExecution.readyToStart') : execution.readiness.missing_requirements.join(', ') }}
        </p>
      </Card>

      <Card v-if="execution.sla_signals?.length">
        <template #header><h3>{{ t('orderExecution.slaSignals') }}</h3></template>
        <ul class="signal-list">
          <li v-for="signal in execution.sla_signals" :key="`${signal.code}-${signal.at}`">
            <Badge :status="signal.severity" /> {{ signal.message }}
            <span v-if="signal.at" class="muted">({{ formatWhen(signal.at) }})</span>
          </li>
        </ul>
      </Card>

      <Card v-if="execution.milestones?.length">
        <template #header><h3>{{ t('orderExecution.milestones') }}</h3></template>
        <ol class="timeline">
          <li v-for="m in execution.milestones" :key="m.id">
            <strong>{{ m.to_status }}</strong>
            <span class="muted">{{ formatWhen(m.occurred_at) }}</span>
            <span v-if="m.from_status" class="muted">← {{ m.from_status }}</span>
          </li>
        </ol>
      </Card>

      <Card v-if="execution.pod_documents?.length">
        <template #header><h3>{{ t('orderExecution.podDocuments') }}</h3></template>
        <ul class="pod-list">
          <li v-for="doc in execution.pod_documents" :key="doc.id">
            {{ doc.document_number }} — <Badge :status="doc.status" />
            <span class="muted">{{ formatWhen(doc.created_at) }}</span>
          </li>
        </ul>
      </Card>
    </template>
  </div>
</template>

<style scoped>
.detail-grid { display: grid; gap: 1rem; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
.detail-grid dt { font-weight: 600; margin-bottom: 0.25rem; }
.muted { color: var(--color-muted, #666); }
.signal-list, .pod-list, .timeline { margin: 0; padding-left: 1.25rem; }
.timeline li, .signal-list li, .pod-list li { margin-bottom: 0.5rem; }
</style>
