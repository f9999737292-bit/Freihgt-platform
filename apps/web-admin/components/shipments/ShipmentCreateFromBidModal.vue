<script setup lang="ts">
import {
  emptyShipmentFromBidForm,
  hasFormErrors,
  toRFC3339,
  validateShipmentFromBidForm,
  type ShipmentFormErrors,
} from '~/types/shipment'
import type { TransportOrder } from '~/types/transportOrder'

const props = defineProps<{
  open: boolean
  initialBidId?: string
  initialTransportOrderId?: string
}>()
const emit = defineEmits<{ close: []; created: [id: string] }>()

const { createShipmentFromBid } = useShipmentsApi()
const tenantStore = useTenantStore()
const { apiGet } = useApi()
const { pushToast } = useToast()
const { t } = useI18n()
const router = useRouter()

const saving = ref(false)
const errorMessage = ref('')
const transportOrders = ref<TransportOrder[]>([])
const form = reactive(emptyShipmentFromBidForm())
const errors = reactive<ShipmentFormErrors>({})

const orderOptions = computed(() =>
  transportOrders.value.map((o) => ({
    label: `${o.order_number || o.id.slice(0, 8)} (${o.status})`,
    value: o.id,
  })),
)

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    Object.assign(
      form,
      emptyShipmentFromBidForm(props.initialBidId || '', props.initialTransportOrderId || ''),
    )
    Object.keys(errors).forEach((k) => delete errors[k as keyof ShipmentFormErrors])
    errorMessage.value = ''
    try {
      const data = await apiGet<{ items: TransportOrder[] }>('/api/v1/transport-orders', {
        query: {
          tenant_id: tenantStore.tenantId,
          limit: 100,
          offset: 0,
        },
      })
      transportOrders.value = data.items ?? []
    } catch {
      transportOrders.value = []
    }
  },
)

function fieldError(field: keyof ShipmentFormErrors) {
  const code = errors[field]
  if (!code) return ''
  if (code === 'range') return t('shipments.validation.deliveryRange')
  return t('rfx.validation.required')
}

async function submit() {
  Object.assign(errors, validateShipmentFromBidForm(form))
  if (hasFormErrors(errors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    const shipment = await createShipmentFromBid({
      bid_id: form.bid_id,
      transport_order_id: form.transport_order_id,
      shipment_number: form.shipment_number,
      planned_pickup_at: toRFC3339(form.planned_pickup_at),
      planned_delivery_at: toRFC3339(form.planned_delivery_at),
    })
    pushToast('success', t('shipments.shipmentCreated'))
    emit('created', shipment.id)
    emit('close')
    await router.push(`/shipments/${shipment.id}`)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('shipments.createFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('shipments.createFromBid')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiInput v-model="form.bid_id" :label="$t('shipments.bidId')" required />
      <p v-if="errors.bid_id" class="field-error">{{ fieldError('bid_id') }}</p>

      <UiSelect
        v-if="orderOptions.length"
        v-model="form.transport_order_id"
        :label="$t('freightRequests.transportOrder')"
        :options="orderOptions"
        required
      />
      <UiInput
        v-else
        v-model="form.transport_order_id"
        :label="$t('freightRequests.transportOrder')"
        required
      />
      <p v-if="errors.transport_order_id" class="field-error">{{ fieldError('transport_order_id') }}</p>

      <UiInput v-model="form.shipment_number" :label="$t('shipments.shipmentNumber')" required />
      <p v-if="errors.shipment_number" class="field-error">{{ fieldError('shipment_number') }}</p>

      <UiInput
        v-model="form.planned_pickup_at"
        type="datetime-local"
        :label="$t('shipments.plannedPickup')"
        required
      />
      <p v-if="errors.planned_pickup_at" class="field-error">{{ fieldError('planned_pickup_at') }}</p>

      <UiInput
        v-model="form.planned_delivery_at"
        type="datetime-local"
        :label="$t('shipments.plannedDelivery')"
        required
      />
      <p v-if="errors.planned_delivery_at" class="field-error">{{ fieldError('planned_delivery_at') }}</p>
    </div>
    <template #footer>
      <UiButton variant="secondary" @click="$emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="saving" :disabled="saving" @click="submit">{{ $t('common.save') }}</UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.modal-error {
  margin: 0 0 1rem;
  padding: 0.75rem;
  border-radius: var(--radius-sm);
  background: #fee2e2;
  color: #991b1b;
  font-size: 0.875rem;
}

.field-error {
  margin: -0.5rem 0 0;
  font-size: 0.8125rem;
  color: #b91c1c;
}
</style>
