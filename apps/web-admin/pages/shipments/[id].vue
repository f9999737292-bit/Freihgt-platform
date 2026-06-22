<script setup lang="ts">
import type { Company } from '~/types/company'
import type {
  CargoSummary,
  Driver,
  LocationSummary,
  Shipment,
  Vehicle,
} from '~/types/shipment'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const router = useRouter()
const tenantStore = useTenantStore()
const { getShipment, isApiUnavailableError } = useShipmentsApi()
const { getDriver } = useDriversApi()
const { getVehicle } = useVehiclesApi()
const { listCompanies } = useCompanies()
const { apiGet } = useApi()
const { pushToast } = useToast()
const { t } = useI18n()

const shipment = ref<Shipment | null>(null)
const companies = ref<Company[]>([])
const driver = ref<Driver | null>(null)
const vehicle = ref<Vehicle | null>(null)
const origin = ref<LocationSummary | null>(null)
const destination = ref<LocationSummary | null>(null)
const cargo = ref<CargoSummary | null>(null)
const loading = ref(true)
const loadingRelated = ref(false)
const apiUnavailable = ref(false)
const showCancelModal = ref(false)
const showDriverSelect = ref(false)
const showVehicleSelect = ref(false)

const shipmentId = computed(() => String(route.params.id))

const showDocumentsBlock = computed(() => {
  const status = shipment.value?.status
  return !!status && ['DELIVERED', 'DELIVERY_CONFIRMED', 'DOCUMENTS_COMPLETED'].includes(status)
})

function createPod() {
  router.push(`/documents?shipment_id=${shipmentId.value}&document_type=POD`)
}

function createEtrn() {
  router.push(`/documents?shipment_id=${shipmentId.value}&document_type=ETRN`)
}

function companyName(id?: string | null) {
  if (!id) return '—'
  return companies.value.find((c) => c.id === id)?.legal_name || id
}

async function loadRelated() {
  if (!shipment.value) return

  loadingRelated.value = true
  driver.value = null
  vehicle.value = null
  origin.value = null
  destination.value = null
  cargo.value = null

  const tenantQuery = { tenant_id: tenantStore.tenantId }
  const tasks: Promise<void>[] = []

  if (shipment.value.driver_id) {
    tasks.push(
      getDriver(shipment.value.driver_id)
        .then((data) => {
          driver.value = data
        })
        .catch(() => {
          driver.value = null
        }),
    )
  }

  if (shipment.value.vehicle_id) {
    tasks.push(
      getVehicle(shipment.value.vehicle_id)
        .then((data) => {
          vehicle.value = data
        })
        .catch(() => {
          vehicle.value = null
        }),
    )
  }

  if (shipment.value.origin_location_id) {
    tasks.push(
      apiGet<LocationSummary>(`/api/v1/locations/${shipment.value.origin_location_id}`, {
        query: tenantQuery,
      })
        .then((data) => {
          origin.value = data
        })
        .catch(() => {
          origin.value = { id: shipment.value!.origin_location_id! }
        }),
    )
  }

  if (shipment.value.destination_location_id) {
    tasks.push(
      apiGet<LocationSummary>(`/api/v1/locations/${shipment.value.destination_location_id}`, {
        query: tenantQuery,
      })
        .then((data) => {
          destination.value = data
        })
        .catch(() => {
          destination.value = { id: shipment.value!.destination_location_id! }
        }),
    )
  }

  if (shipment.value.cargo_id) {
    tasks.push(
      apiGet<CargoSummary>(`/api/v1/cargoes/${shipment.value.cargo_id}`, { query: tenantQuery })
        .then((data) => {
          cargo.value = data
        })
        .catch(() => {
          cargo.value = { id: shipment.value!.cargo_id! }
        }),
    )
  }

  await Promise.all(tasks)
  loadingRelated.value = false
}

async function loadShipment() {
  loading.value = true
  apiUnavailable.value = false
  try {
    shipment.value = await getShipment(shipmentId.value)
    await loadRelated()
  } catch (error) {
    shipment.value = null
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('shipments.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

watch(shipmentId, loadShipment, { immediate: true })

onMounted(async () => {
  try {
    companies.value = (await listCompanies({ limit: 100 })).items
  } catch {
    companies.value = []
  }
})
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/shipments">{{ $t('shipments.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('shipments.details') }}</span>
    </nav>

    <UiPageHeader :title="shipment?.shipment_number || $t('shipments.details')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/shipments')">
          {{ $t('common.back') }}
        </UiButton>
      </template>
    </UiPageHeader>

    <div v-if="loading" class="loading-block">{{ $t('common.loading') }}</div>
    <UiEmptyState v-else-if="apiUnavailable" :title="$t('shipments.loadFailed')" />
    <UiEmptyState v-else-if="!shipment" :title="$t('shipments.noShipmentsFound')" />

    <template v-else>
      <ShipmentsShipmentActions
        :shipment="shipment"
        @updated="loadShipment"
        @cancel="showCancelModal = true"
      />

      <ShipmentsShipmentDetailsCard :shipment="shipment" />
      <ShipmentsShipmentPartiesCard :shipment="shipment" :company-name="companyName" />
      <ShipmentsShipmentRouteCard
        :origin="origin"
        :destination="destination"
        :loading="loadingRelated"
      />
      <ShipmentsShipmentCargoCard :cargo="cargo" :loading="loadingRelated" />
      <ShipmentsShipmentAssignmentCard
        :shipment="shipment"
        :driver="driver"
        :vehicle="vehicle"
        :loading-driver="loadingRelated"
        :loading-vehicle="loadingRelated"
        @assign-driver="showDriverSelect = true"
        @assign-vehicle="showVehicleSelect = true"
      />
      <ShipmentsShipmentStatusTimeline :status="shipment.status" />

      <LowCodeCustomFieldsPanel
        entity-type="SHIPMENT"
        :entity-id="shipment.id"
        :entity-status="shipment.status"
      />

      <UiCard v-if="showDocumentsBlock">
        <template #header>
          <h3 class="card-title">{{ $t('documents.shipmentDocuments') }}</h3>
        </template>
        <p class="text-muted">{{ $t('documents.shipmentDocumentsHintShort') }}</p>
        <div class="actions-row">
          <UiButton @click="createPod">{{ $t('documents.createPod') }}</UiButton>
          <UiButton variant="secondary" @click="createEtrn">{{ $t('documents.createEtrn') }}</UiButton>
        </div>
      </UiCard>
    </template>

    <ShipmentsShipmentCancelModal
      :open="showCancelModal"
      :shipment-id="shipmentId"
      @close="showCancelModal = false"
      @cancelled="loadShipment"
    />

    <DriversDriverSelect
      v-if="shipment?.carrier_company_id"
      :open="showDriverSelect"
      :shipment-id="shipmentId"
      :carrier-company-id="shipment.carrier_company_id"
      @close="showDriverSelect = false"
      @assigned="loadShipment"
    />

    <VehiclesVehicleSelect
      v-if="shipment?.carrier_company_id"
      :open="showVehicleSelect"
      :shipment-id="shipmentId"
      :carrier-company-id="shipment.carrier_company_id"
      @close="showVehicleSelect = false"
      @assigned="loadShipment"
    />
  </div>
</template>

<style scoped>
.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.breadcrumbs__sep {
  opacity: 0.5;
}

.loading-block {
  padding: 2rem;
  text-align: center;
  color: var(--color-text-muted);
}

.card-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
}

.actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-top: 0.75rem;
}
</style>
