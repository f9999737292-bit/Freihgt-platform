<script setup lang="ts">
import type { TransportOrder } from '~/types/transportOrder'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { apiGet, apiPost } = useApi()
const { pushToast } = useToast()
const { t } = useI18n()

const order = ref<TransportOrder | null>(null)
const submitting = ref(false)
const showMiniTenderModal = ref(false)

async function loadOrder() {
  try {
    order.value = await apiGet<TransportOrder>(`/api/v1/transport-orders/${route.params.id}`)
  } catch {
    order.value = null
  }
}

async function submitOrder() {
  if (!order.value) return
  submitting.value = true
  try {
    await apiPost(`/api/v1/transport-orders/${order.value.id}/submit`)
    pushToast('success', t('transportOrders.submitOrder'))
    await loadOrder()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    submitting.value = false
  }
}

onMounted(loadOrder)
</script>

<template>
  <div class="page-stack">
    <UiPageHeader :title="order?.order_number || $t('transportOrders.title')">
      <template #actions>
        <UiButton
          v-if="order?.status === 'READY_FOR_SOURCING'"
          @click="showMiniTenderModal = true"
        >
          {{ $t('freightRequests.createMiniTender') }}
        </UiButton>
        <UiButton
          v-if="order?.status === 'DRAFT'"
          :loading="submitting"
          @click="submitOrder"
        >
          {{ $t('transportOrders.submitOrder') }}
        </UiButton>
      </template>
    </UiPageHeader>

    <UiCard v-if="order">
      <div class="form-grid form-grid--2">
        <div><span class="text-muted">{{ $t('common.status') }}</span><div><UiBadge :status="order.status" /></div></div>
        <div><span class="text-muted">{{ $t('transportOrders.equipmentType') }}</span><div>{{ order.equipment_type || '—' }}</div></div>
        <div><span class="text-muted">{{ $t('transportOrders.pickupDate') }}</span><div>{{ order.pickup_date || order.requested_pickup_date || '—' }}</div></div>
        <div><span class="text-muted">{{ $t('transportOrders.deliveryDate') }}</span><div>{{ order.delivery_date || order.requested_delivery_date || '—' }}</div></div>
      </div>
    </UiCard>
    <LowCodeCustomFieldsPanel
      v-if="order"
      entity-type="TRANSPORT_ORDER"
      :entity-id="order.id"
      :entity-status="order.status"
      editable
      show-full-editor-link
    />
    <UiEmptyState v-else :title="$t('common.empty')" />

    <FreightRequestsFreightRequestCreateFromTransportOrderModal
      :open="showMiniTenderModal"
      :initial-transport-order-id="order?.id"
      :initial-shipper-company-id="order?.shipper_company_id"
      @close="showMiniTenderModal = false"
    />
  </div>
</template>
