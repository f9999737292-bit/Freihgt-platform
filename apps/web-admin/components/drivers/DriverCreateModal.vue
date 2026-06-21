<script setup lang="ts">
import {
  emptyDriverCreateForm,
  hasFormErrors,
  validateDriverCreateForm,
  type DriverFormErrors,
} from '~/types/shipment'

const props = defineProps<{ open: boolean; carrierCompanyId: string }>()
const emit = defineEmits<{ close: []; created: [id: string] }>()

const { createDriver } = useDriversApi()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const errorMessage = ref('')
const form = reactive(emptyDriverCreateForm())
const errors = reactive<DriverFormErrors>({})

watch(
  () => props.open,
  (open) => {
    if (!open) return
    Object.assign(form, emptyDriverCreateForm(props.carrierCompanyId))
    Object.keys(errors).forEach((k) => delete errors[k as keyof DriverFormErrors])
    errorMessage.value = ''
  },
)

function fieldError(field: keyof DriverFormErrors) {
  const code = errors[field]
  if (!code) return ''
  if (code === 'countryCode') return t('companies.validation.countryCode')
  return t('rfx.validation.required')
}

async function submit() {
  Object.assign(errors, validateDriverCreateForm(form))
  if (hasFormErrors(errors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    const driver = await createDriver({
      carrier_company_id: form.carrier_company_id,
      full_name: form.full_name,
      phone: form.phone,
      license_number: form.license_number,
      license_country: form.license_country,
      preferred_locale: form.preferred_locale,
    })
    pushToast('success', t('shipments.driverCreated'))
    emit('created', driver.id)
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('shipments.assignDriverFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('shipments.createDriver')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiInput v-model="form.full_name" :label="$t('users.fullName')" required />
      <p v-if="errors.full_name" class="field-error">{{ fieldError('full_name') }}</p>

      <UiInput v-model="form.phone" :label="$t('users.phone')" />
      <UiInput v-model="form.license_number" :label="$t('shipments.licenseNumber')" />
      <UiInput v-model="form.license_country" :label="$t('companies.countryCode')" required />
      <p v-if="errors.license_country" class="field-error">{{ fieldError('license_country') }}</p>

      <UiInput v-model="form.preferred_locale" :label="$t('companies.preferredLocale')" required />
      <p v-if="errors.preferred_locale" class="field-error">{{ fieldError('preferred_locale') }}</p>
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
