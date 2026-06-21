<script setup lang="ts">
import {
  emptyCreateBidForm,
  hasFormErrors,
  parseBidFormNumbers,
  toRFC3339,
  validateBidForm,
  type BidFormErrors,
} from '~/types/rfx'
import type { Company } from '~/types/company'

const props = defineProps<{ open: boolean; freightRequestId: string }>()
const emit = defineEmits<{ close: []; created: [] }>()

const { createBid } = useFreightRequestsApi()
const { listCompanies } = useCompanies()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const errorMessage = ref('')
const carriers = ref<Company[]>([])
const form = reactive(emptyCreateBidForm())
const errors = reactive<BidFormErrors>({})

const carrierOptions = computed(() =>
  carriers.value.map((c) => ({ label: c.legal_name, value: c.id })),
)

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    Object.assign(form, emptyCreateBidForm())
    Object.keys(errors).forEach((k) => delete errors[k as keyof BidFormErrors])
    errorMessage.value = ''
    try {
      const data = await listCompanies({ company_type: 'CARRIER', limit: 100 })
      carriers.value = data.items.length ? data.items : (await listCompanies({ limit: 100 })).items
    } catch {
      carriers.value = []
    }
  },
)

function fieldError(field: keyof BidFormErrors) {
  const code = errors[field]
  if (!code) return ''
  if (code === 'negative') return t('freightRequests.validation.negative')
  return t('rfx.validation.required')
}

async function submit() {
  Object.assign(errors, validateBidForm(form))
  if (hasFormErrors(errors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    const amounts = parseBidFormNumbers(form)
    await createBid(props.freightRequestId, {
      carrier_company_id: form.carrier_company_id,
      bid_number: form.bid_number,
      currency_code: form.currency_code,
      vat_rate: amounts.vat_rate,
      valid_until: toRFC3339(form.valid_until),
      items: [
        {
          description: 'Тестовая ставка',
          base_amount: amounts.base_amount,
          fuel_surcharge: amounts.fuel_surcharge,
          toll_amount: amounts.toll_amount,
          extra_charges: amounts.extra_charges,
          vat_rate: amounts.vat_rate,
          comment: form.comment,
        },
      ],
    })
    pushToast('success', t('freightRequests.bidCreated'))
    emit('created')
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('freightRequests.bidCreateFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('freightRequests.addBid')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiSelect
        v-model="form.carrier_company_id"
        :label="$t('freightRequests.carrier')"
        :options="carrierOptions"
        required
      />
      <p v-if="errors.carrier_company_id" class="field-error">{{ fieldError('carrier_company_id') }}</p>

      <UiInput v-model="form.bid_number" :label="$t('freightRequests.bidNumber')" required />
      <p v-if="errors.bid_number" class="field-error">{{ fieldError('bid_number') }}</p>

      <UiInput v-model="form.currency_code" :label="$t('rfx.currency')" required />
      <UiInput v-model="form.vat_rate" type="number" :label="$t('freightRequests.vatRate')" />
      <UiInput v-model="form.valid_until" type="datetime-local" :label="$t('freightRequests.validUntil')" required />

      <UiInput v-model="form.base_amount" type="number" :label="$t('freightRequests.baseAmount')" />
      <UiInput v-model="form.fuel_surcharge" type="number" :label="$t('freightRequests.fuelSurcharge')" />
      <UiInput v-model="form.toll_amount" type="number" :label="$t('freightRequests.tollAmount')" />
      <UiInput v-model="form.extra_charges" type="number" :label="$t('freightRequests.extraCharges')" />
      <UiInput v-model="form.comment" :label="$t('freightRequests.comment')" />
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
