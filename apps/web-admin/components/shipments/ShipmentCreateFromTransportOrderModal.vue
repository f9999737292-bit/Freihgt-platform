<script setup lang="ts">
import {
  emptyShipmentFromOrderForm,
  hasFormErrors,
  toRFC3339,
  validateShipmentFromOrderForm,
  type ShipmentFormErrors,
} from '~/types/shipment'
import type { Company } from '~/types/company'
import type { TransportOrder } from '~/types/transportOrder'

const props = defineProps<{
  open: boolean
  initialTransportOrderId?: string
}>()
const emit = defineEmits<{ close: []; created: [id: string] }>()

const { createShipmentFromTransportOrder } = useShipmentsApi()
const { listCompanies } = useCompanies()
const tenantStore = useTenantStore()
const { apiGet } = useApi()
const { pushToast } = useToast()
const { t } = useI18n()
const router = useRouter()

const saving = ref(false)
const errorMessage = ref('')
const companies = ref<Company[]>([])
const transportOrders = ref<TransportOrder[]>([])
const form = reactive(emptyShipmentFromOrderForm())
const errors = reactive<ShipmentFormErrors>({})

const carrierOptions = computed(() =>
  companies.value
    .filter((c) => c.company_type === 'CARRIER')
    .map((c) => ({ label: c.legal_name, value: c.id })),
)
const forwarderOptions = computed(() => [
  { label: '—', value: '' },
  ...companies.value.map((c) => ({ label: c.legal_name, value: c.id })),
])
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
    Object.assign(form, emptyShipmentFromOrderForm(props.initialTransportOrderId || ''))
    Object.keys(errors).forEach((k) => delete errors[k as keyof ShipmentFormErrors])
    errorMessage.value = ''
    try {
      const [companiesData, ordersData] = await Promise.all([
        listCompanies({ company_type: 'CARRIER', limit: 100 }),
        apiGet<{ items: TransportOrder[] }>('/api/v1/transport-orders', {
          query: {
            tenant_id: tenantStore.tenantId,
            limit: 100,
            offset: 0,
          },
        }),
      ])
      companies.value = companiesData.items.length
        ? companiesData.items
        : (await listCompanies({ limit: 100 })).items
      transportOrders.value = ordersData.items ?? []
    } catch {
      companies.value = []
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
  Object.assign(errors, validateShipmentFromOrderForm(form))
  if (hasFormErrors(errors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    const shipment = await createShipmentFromTransportOrder({
      transport_order_id: form.transport_order_id,
      carrier_company_id: form.carrier_company_id,
      forwarder_company_id: form.forwarder_company_id || null,
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
  <UiModal :open="open" :title="$t('shipments.createFromTransportOrder')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
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

      <UiSelect
        v-model="form.carrier_company_id"
        :label="$t('shipments.carrier')"
        :options="carrierOptions"
        required
      />
      <p v-if="errors.carrier_company_id" class="field-error">{{ fieldError('carrier_company_id') }}</p>

      <UiSelect
        v-model="form.forwarder_company_id"
        :label="$t('shipments.forwarder')"
        :options="forwarderOptions"
      />

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
