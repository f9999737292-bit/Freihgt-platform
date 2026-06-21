<script setup lang="ts">
import type { Vehicle } from '~/types/shipment'

const props = defineProps<{
  open: boolean
  carrierCompanyId: string
  shipmentId: string
}>()
const emit = defineEmits<{ close: []; assigned: [] }>()

const { listVehicles } = useVehiclesApi()
const { assignVehicle } = useShipmentsApi()
const { pushToast } = useToast()
const { t } = useI18n()

const loading = ref(false)
const assigning = ref(false)
const errorMessage = ref('')
const selectedVehicleId = ref('')
const showCreateModal = ref(false)
const vehicles = ref<Vehicle[]>([])

const vehicleOptions = computed(() =>
  vehicles.value.map((v) => ({
    label: `${v.plate_number} (${v.vehicle_type})`,
    value: v.id,
  })),
)

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    selectedVehicleId.value = ''
    errorMessage.value = ''
    await loadVehicles()
  },
)

async function loadVehicles() {
  if (!props.carrierCompanyId) {
    vehicles.value = []
    return
  }
  loading.value = true
  try {
    const data = await listVehicles({ carrier_company_id: props.carrierCompanyId, limit: 100 })
    vehicles.value = data.items
  } catch {
    vehicles.value = []
  } finally {
    loading.value = false
  }
}

async function onVehicleCreated(id: string) {
  await loadVehicles()
  selectedVehicleId.value = id
}

async function assign() {
  if (!selectedVehicleId.value) {
    errorMessage.value = t('rfx.validation.required')
    return
  }

  assigning.value = true
  errorMessage.value = ''
  try {
    await assignVehicle(props.shipmentId, { vehicle_id: selectedVehicleId.value })
    pushToast('success', t('shipments.vehicleAssigned'))
    emit('assigned')
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('shipments.assignVehicleFailed')
  } finally {
    assigning.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('shipments.assignVehicle')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div v-if="loading" class="text-muted">{{ $t('common.loading') }}</div>
    <div v-else class="form-grid">
      <UiSelect
        v-model="selectedVehicleId"
        :label="$t('shipments.vehicle')"
        :options="vehicleOptions"
        required
      />
      <UiButton variant="secondary" @click="showCreateModal = true">{{ $t('shipments.createVehicle') }}</UiButton>
    </div>
    <template #footer>
      <UiButton variant="secondary" @click="$emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="assigning" :disabled="assigning" @click="assign">{{ $t('shipments.assignVehicle') }}</UiButton>
    </template>
  </UiModal>

  <VehiclesVehicleCreateModal
    :open="showCreateModal"
    :carrier-company-id="carrierCompanyId"
    @close="showCreateModal = false"
    @created="onVehicleCreated"
  />
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
</style>
