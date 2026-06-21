<script setup lang="ts">
import {
  FREIGHT_REQUEST_TYPES,
  emptyCreateFreightRequestForm,
  hasFormErrors,
  toRFC3339,
  validateFreightRequestForm,
  type FreightRequestFormErrors,
} from '~/types/rfx'
import type { Company } from '~/types/company'
import type { TransportOrder } from '~/types/transportOrder'

const props = defineProps<{
  open: boolean
  initialTransportOrderId?: string
  initialShipperCompanyId?: string
}>()
const emit = defineEmits<{ close: []; created: [id: string] }>()

const { createFreightRequestFromTransportOrder } = useFreightRequestsApi()
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
const form = reactive(emptyCreateFreightRequestForm())
const errors = reactive<FreightRequestFormErrors>({})

const typeOptions = computed(() => FREIGHT_REQUEST_TYPES.map((v) => ({ label: v, value: v })))
const companyOptions = computed(() =>
  companies.value.map((c) => ({ label: c.legal_name, value: c.id })),
)
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
      emptyCreateFreightRequestForm(
        props.initialTransportOrderId || '',
        props.initialShipperCompanyId || '',
      ),
    )
    Object.keys(errors).forEach((k) => delete errors[k as keyof FreightRequestFormErrors])
    errorMessage.value = ''
    try {
      const [companiesData, ordersData] = await Promise.all([
        listCompanies({ limit: 100 }),
        apiGet<{ items: TransportOrder[] }>('/api/v1/transport-orders', {
          query: {
            tenant_id: tenantStore.tenantId,
            status: 'READY_FOR_SOURCING',
            limit: 100,
            offset: 0,
          },
        }),
      ])
      companies.value = companiesData.items
      transportOrders.value = ordersData.items ?? []
    } catch {
      companies.value = []
      transportOrders.value = []
    }
  },
)

watch(
  () => form.transport_order_id,
  (id) => {
    const order = transportOrders.value.find((o) => o.id === id)
    if (order?.shipper_company_id && !props.initialShipperCompanyId) {
      form.shipper_company_id = order.shipper_company_id
    }
  },
)

function fieldError(field: keyof FreightRequestFormErrors) {
  return errors[field] ? t('rfx.validation.required') : ''
}

async function submit() {
  Object.assign(errors, validateFreightRequestForm(form))
  if (hasFormErrors(errors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    const fr = await createFreightRequestFromTransportOrder({
      ...form,
      response_deadline: toRFC3339(form.response_deadline || ''),
    })
    pushToast('success', t('freightRequests.createdSuccess'))
    emit('created', fr.id)
    emit('close')
    await router.push(`/freight-requests/${fr.id}`)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('freightRequests.createFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('freightRequests.createFromOrder')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiSelect
        v-model="form.transport_order_id"
        :label="$t('freightRequests.transportOrder')"
        :options="orderOptions"
        required
      />
      <p v-if="errors.transport_order_id" class="field-error">{{ fieldError('transport_order_id') }}</p>

      <UiInput
        v-model="form.freight_request_number"
        :label="$t('freightRequests.number')"
        required
      />
      <p v-if="errors.freight_request_number" class="field-error">
        {{ fieldError('freight_request_number') }}
      </p>

      <UiSelect
        v-model="form.request_type"
        :label="$t('freightRequests.requestType')"
        :options="typeOptions"
      />

      <UiSelect
        v-model="form.shipper_company_id"
        :label="$t('freightRequests.shipper')"
        :options="companyOptions"
        required
      />
      <p v-if="errors.shipper_company_id" class="field-error">{{ fieldError('shipper_company_id') }}</p>

      <UiInput v-model="form.currency_code" :label="$t('rfx.currency')" />
      <UiInput
        v-model="form.response_deadline"
        type="datetime-local"
        :label="$t('rfx.responseDeadline')"
        required
      />
      <p v-if="errors.response_deadline" class="field-error">{{ fieldError('response_deadline') }}</p>
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
  color: #b91c1b;
}
</style>
