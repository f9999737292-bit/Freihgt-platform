<script setup lang="ts">
import {
  EQUIPMENT_TYPES,
  VEHICLE_TYPES,
  emptyVehicleCreateForm,
  hasFormErrors,
  validateVehicleCreateForm,
  type VehicleFormErrors,
} from '~/types/shipment'

const props = defineProps<{ open: boolean; carrierCompanyId: string }>()
const emit = defineEmits<{ close: []; created: [id: string] }>()

const { createVehicle } = useVehiclesApi()
const { pushToast } = useToast()
const { t } = useI18n()

const saving = ref(false)
const errorMessage = ref('')
const form = reactive(emptyVehicleCreateForm())
const errors = reactive<VehicleFormErrors>({})

const vehicleTypeOptions = computed(() => VEHICLE_TYPES.map((v) => ({ label: v, value: v })))
const equipmentOptions = computed(() => EQUIPMENT_TYPES.map((v) => ({ label: v, value: v })))

watch(
  () => props.open,
  (open) => {
    if (!open) return
    Object.assign(form, emptyVehicleCreateForm(props.carrierCompanyId))
    Object.keys(errors).forEach((k) => delete errors[k as keyof VehicleFormErrors])
    errorMessage.value = ''
  },
)

function fieldError(field: keyof VehicleFormErrors) {
  const code = errors[field]
  if (!code) return ''
  if (code === 'countryCode') return t('companies.validation.countryCode')
  return t('rfx.validation.required')
}

async function submit() {
  Object.assign(errors, validateVehicleCreateForm(form))
  if (hasFormErrors(errors)) return

  saving.value = true
  errorMessage.value = ''
  try {
    const vehicle = await createVehicle({
      carrier_company_id: form.carrier_company_id,
      plate_number: form.plate_number,
      vehicle_type: form.vehicle_type,
      equipment_type: form.equipment_type,
      capacity_weight: Number(form.capacity_weight) || undefined,
      capacity_volume: Number(form.capacity_volume) || undefined,
      registration_country: form.registration_country,
    })
    pushToast('success', t('shipments.vehicleCreated'))
    emit('created', vehicle.id)
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('shipments.assignVehicleFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('shipments.createVehicle')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div class="form-grid form-grid--2">
      <UiInput v-model="form.plate_number" :label="$t('shipments.plateNumber')" required />
      <p v-if="errors.plate_number" class="field-error">{{ fieldError('plate_number') }}</p>

      <UiSelect v-model="form.vehicle_type" :label="$t('shipments.vehicleType')" :options="vehicleTypeOptions" />
      <UiSelect v-model="form.equipment_type" :label="$t('transportOrders.equipmentType')" :options="equipmentOptions" />

      <UiInput v-model="form.capacity_weight" type="number" :label="$t('shipments.capacityWeight')" />
      <UiInput v-model="form.capacity_volume" type="number" :label="$t('shipments.capacityVolume')" />
      <UiInput v-model="form.registration_country" :label="$t('companies.countryCode')" required />
      <p v-if="errors.registration_country" class="field-error">{{ fieldError('registration_country') }}</p>
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
