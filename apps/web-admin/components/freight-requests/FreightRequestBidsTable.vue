<script setup lang="ts">
import { formatMoney, formatRfxDate, type Bid, type FreightRequest } from '~/types/rfx'

const props = defineProps<{
  freightRequestId: string
  request: FreightRequest
  companyName?: (id: string) => string
}>()
const emit = defineEmits<{ updated: [] }>()

const { listFreightRequestBids, isApiUnavailableError } = useFreightRequestsApi()
const { submitBid, acceptBid } = useBidsApi()
const { pushToast } = useToast()
const { t } = useI18n()
const router = useRouter()

const items = ref<Bid[]>([])
const loading = ref(true)
const loadFailed = ref(false)
const showCreateModal = ref(false)
const actionLoading = ref<string | null>(null)
const acceptedBid = ref<Bid | null>(null)

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    items.value = await listFreightRequestBids(props.freightRequestId)
    acceptedBid.value = items.value.find((b) => b.status === 'ACCEPTED') || null
  } catch (error) {
    items.value = []
    loadFailed.value = isApiUnavailableError(error)
    if (!loadFailed.value) {
      pushToast('error', error instanceof Error ? error.message : t('freightRequests.loadBidsFailed'))
    }
  } finally {
    loading.value = false
  }
}

function amountWithoutVat(bid: Bid) {
  return bid.total_amount ?? bid.items?.[0]?.amount_without_vat
}

async function handleSubmit(bid: Bid) {
  actionLoading.value = bid.id
  try {
    await submitBid(bid.id)
    pushToast('success', t('freightRequests.bidSubmitted'))
    await load()
    emit('updated')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    actionLoading.value = null
  }
}

async function handleAccept(bid: Bid) {
  if (!confirm(t('freightRequests.acceptConfirm'))) return
  actionLoading.value = bid.id
  try {
    await acceptBid(bid.id)
    pushToast('success', t('freightRequests.bidAccepted'))
    await load()
    emit('updated')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    actionLoading.value = null
  }
}

function createShipment() {
  const bid = acceptedBid.value
  if (!bid || !props.request.transport_order_id) return
  router.push(
    `/shipments?bid_id=${bid.id}&transport_order_id=${props.request.transport_order_id}`,
  )
}

watch(() => props.freightRequestId, load, { immediate: true })
</script>

<template>
  <UiCard>
    <template #header>
      <div class="section-header">
        <h3>{{ $t('freightRequests.bids') }}</h3>
        <UiButton size="sm" @click="showCreateModal = true">{{ $t('freightRequests.addBid') }}</UiButton>
      </div>
    </template>

    <UiTable
      v-if="items.length || loading"
      :columns="[
        $t('freightRequests.bidNumber'),
        $t('freightRequests.carrier'),
        $t('freightRequests.amountWithoutVat'),
        $t('freightRequests.vatAmount'),
        $t('freightRequests.amountWithVat'),
        $t('rfx.currency'),
        $t('common.status'),
        $t('freightRequests.submittedAt'),
        $t('common.actions'),
      ]"
      :loading="loading"
    >
      <tr v-for="bid in items" :key="bid.id">
        <td>{{ bid.bid_number }}</td>
        <td>{{ companyName?.(bid.carrier_company_id) || bid.carrier_company_id }}</td>
        <td>{{ formatMoney(amountWithoutVat(bid), bid.currency_code) }}</td>
        <td>{{ formatMoney(bid.vat_amount, bid.currency_code) }}</td>
        <td>{{ formatMoney(bid.total_amount_with_vat, bid.currency_code) }}</td>
        <td>{{ bid.currency_code || '—' }}</td>
        <td><FreightRequestsBidStatusBadge :status="bid.status" /></td>
        <td>{{ formatRfxDate(bid.submitted_at) }}</td>
        <td class="actions-cell">
          <UiButton
            v-if="bid.status === 'DRAFT'"
            size="sm"
            variant="secondary"
            :loading="actionLoading === bid.id"
            @click="handleSubmit(bid)"
          >
            {{ $t('freightRequests.submitBid') }}
          </UiButton>
          <UiButton
            v-if="bid.status === 'SUBMITTED'"
            size="sm"
            :loading="actionLoading === bid.id"
            @click="handleAccept(bid)"
          >
            {{ $t('freightRequests.acceptBid') }}
          </UiButton>
        </td>
      </tr>
    </UiTable>

    <UiEmptyState v-else-if="loadFailed" :title="$t('freightRequests.loadBidsFailed')" />
    <UiEmptyState v-else :title="$t('freightRequests.noBids')" />
  </UiCard>

  <UiCard v-if="acceptedBid" class="shipment-hint">
    <p>{{ $t('freightRequests.createShipmentHint') }}</p>
    <UiButton @click="createShipment">{{ $t('freightRequests.createShipment') }}</UiButton>
  </UiCard>

  <FreightRequestsBidCreateModal
    :open="showCreateModal"
    :freight-request-id="freightRequestId"
    @close="showCreateModal = false"
    @created="load"
  />
</template>

<style scoped>
.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.section-header h3 {
  margin: 0;
  font-size: 1rem;
}

.actions-cell {
  white-space: nowrap;
}

.shipment-hint {
  margin-top: 0;
}

.shipment-hint p {
  margin: 0 0 1rem;
}
</style>
