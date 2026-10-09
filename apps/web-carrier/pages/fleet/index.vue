<script setup lang="ts">
import FleetView from '../../components/FleetView.vue'
import PortalState from '../../components/PortalState.vue'
import { viewKindFromError, type PortalViewKind } from '../../domain/viewState'

const { t } = useI18n()
const api = useCarrierApi()
const office = useCarrierOffice()
const kind = ref<PortalViewKind>('loading')
const detail = ref('')
const drivers = ref<Array<{ id: string; full_name?: string; status?: string }>>([])
const vehicles = ref<Array<{ id: string; plate_number?: string; status?: string }>>([])

const role = computed(() => office.session.value?.user.roles[0] ?? '')

async function load() {
  kind.value = 'loading'
  drivers.value = []
  vehicles.value = []
  try {
    const data = await api.listFleet()
    drivers.value = data.drivers
    vehicles.value = data.vehicles
    kind.value = 'ready'
  } catch (error) {
    drivers.value = []
    vehicles.value = []
    kind.value = viewKindFromError(error)
    detail.value = error instanceof Error ? error.message : ''
  }
}

onMounted(load)

function titleFor(state: PortalViewKind) {
  if (state === 'forbidden') return t('carrier.forbidden')
  if (state === 'not_found') return t('carrier.notFound')
  if (state === 'loading') return t('carrier.loading')
  return t('carrier.unavailable')
}
</script>

<template>
  <PortalState v-if="kind !== 'ready'" :kind="kind" :title="titleFor(kind)" :description="detail" />
  <FleetView v-else :role="role" :drivers="drivers" :vehicles="vehicles" />
</template>
