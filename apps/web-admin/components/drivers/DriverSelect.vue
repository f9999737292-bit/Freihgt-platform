<script setup lang="ts">
import type { Driver } from '~/types/shipment'

const props = defineProps<{
  open: boolean
  carrierCompanyId: string
  shipmentId: string
}>()
const emit = defineEmits<{ close: []; assigned: [] }>()

const { listDrivers } = useDriversApi()
const { assignDriver } = useShipmentsApi()
const { pushToast } = useToast()
const { t } = useI18n()

const loading = ref(false)
const assigning = ref(false)
const errorMessage = ref('')
const selectedDriverId = ref('')
const showCreateModal = ref(false)
const drivers = ref<Driver[]>([])

const driverOptions = computed(() =>
  drivers.value.map((d) => ({
    label: `${d.full_name}${d.phone ? ` (${d.phone})` : ''}`,
    value: d.id,
  })),
)

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    selectedDriverId.value = ''
    errorMessage.value = ''
    await loadDrivers()
  },
)

async function loadDrivers() {
  if (!props.carrierCompanyId) {
    drivers.value = []
    return
  }
  loading.value = true
  try {
    const data = await listDrivers({ carrier_company_id: props.carrierCompanyId, limit: 100 })
    drivers.value = data.items
  } catch {
    drivers.value = []
  } finally {
    loading.value = false
  }
}

async function onDriverCreated(id: string) {
  await loadDrivers()
  selectedDriverId.value = id
}

async function assign() {
  if (!selectedDriverId.value) {
    errorMessage.value = t('rfx.validation.required')
    return
  }

  assigning.value = true
  errorMessage.value = ''
  try {
    await assignDriver(props.shipmentId, { driver_id: selectedDriverId.value })
    pushToast('success', t('shipments.driverAssigned'))
    emit('assigned')
    emit('close')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : t('shipments.assignDriverFailed')
  } finally {
    assigning.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('shipments.assignDriver')" @close="$emit('close')">
    <p v-if="errorMessage" class="modal-error">{{ errorMessage }}</p>
    <div v-if="loading" class="text-muted">{{ $t('common.loading') }}</div>
    <div v-else class="form-grid">
      <UiSelect
        v-model="selectedDriverId"
        :label="$t('shipments.driver')"
        :options="driverOptions"
        required
      />
      <UiButton variant="secondary" @click="showCreateModal = true">{{ $t('shipments.createDriver') }}</UiButton>
    </div>
    <template #footer>
      <UiButton variant="secondary" @click="$emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="assigning" :disabled="assigning" @click="assign">{{ $t('shipments.assignDriver') }}</UiButton>
    </template>
  </UiModal>

  <DriversDriverCreateModal
    :open="showCreateModal"
    :carrier-company-id="carrierCompanyId"
    @close="showCreateModal = false"
    @created="onDriverCreated"
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
