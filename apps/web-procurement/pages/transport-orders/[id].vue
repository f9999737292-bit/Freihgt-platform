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

async function loadExecution() {
  loading.value = true
  notFound.value = false
  apiUnavailable.value = false
  try {
    if (!currentCompanyId.value) throw new Error(t('orderExecution.missingCompany'))
    execution.value = await getOrderExecution(orderId.value, currentCompanyId.value, 'BUYER')
  } catch (error) {
    execution.value = null
    if (shouldShowNotFound(error)) notFound.value = true
    else {
      apiUnavailable.value = isApiUnavailableError(error)
      if (!apiUnavailable.value) {
        pushToast('error', error instanceof Error ? error.message : t('orderExecution.loadFailed'))
      }
    }
  } finally {
    loading.value = false
  }
}

watch(orderId, loadExecution, { immediate: true })
watch(currentCompanyId, loadExecution)
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/transport-orders">{{ t('orderExecution.buyerTitle') }}</NuxtLink>
      <span class="breadcrumbs__sep" aria-hidden="true">/</span>
      <NuxtLink v-if="execution" :to="`/tenders/${execution.provenance.rfx_event_id}/evaluation`">
        {{ t('orderExecution.backToTender') }}
      </NuxtLink>
      <span v-if="execution" class="breadcrumbs__sep" aria-hidden="true">/</span>
      <span aria-current="page">{{ t('orderExecution.title') }}</span>
    </nav>

    <PageHeader :title="t('orderExecution.title')" :subtitle="execution?.transport_order_number">
      <template #actions>
        <Button variant="secondary" @click="$router.push('/transport-orders')">
          {{ t('orderExecution.backToList') }}
        </Button>
      </template>
    </PageHeader>

    <div v-if="loading" role="status" aria-live="polite">{{ t('common.loading') }}</div>
    <EmptyState v-else-if="notFound" :title="t('orderExecution.notFound')" />
    <EmptyState v-else-if="apiUnavailable" :title="t('orderExecution.loadFailed')" />

    <template v-else-if="execution">
      <Card>
        <dl class="detail-grid">
          <div>
            <dt>{{ t('orderExecution.orderNumber') }}</dt>
            <dd>{{ execution.transport_order_number }}</dd>
          </div>
          <div>
            <dt>{{ t('orderExecution.orderStatus') }}</dt>
            <dd><Badge :status="execution.transport_order_status" /></dd>
          </div>
          <div>
            <dt>{{ t('orderExecution.sourceTender') }}</dt>
            <dd>{{ execution.provenance.rfx_event_id }}</dd>
          </div>
          <div>
            <dt>{{ t('orderExecution.awardAmount') }}</dt>
            <dd>{{ formatMoney(execution.provenance.amount, execution.provenance.currency_code) }}</dd>
          </div>
        </dl>
      </Card>

      <Card v-if="execution.shipment">
        <template #header><h3>{{ t('orderExecution.shipment') }}</h3></template>
        <p>
          {{ execution.shipment.shipment_number }} —
          <Badge :status="execution.shipment.status" />
        </p>
        <p class="muted">
          {{ t('orderExecution.readiness') }}:
          {{
            execution.readiness.ready_to_start
              ? t('orderExecution.readyToStart')
              : execution.readiness.missing_requirements.join(', ')
          }}
        </p>
      </Card>

      <ExecutionTrackingPanel :execution="execution" />
    </template>
  </div>
</template>

<style scoped>
.detail-grid {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.detail-grid dt {
  font-weight: 600;
  margin-bottom: 0.25rem;
}

.muted {
  color: var(--color-text-muted, #64748b);
}
</style>
