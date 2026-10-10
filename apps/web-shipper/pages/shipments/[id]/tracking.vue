<script setup lang="ts">
import { CustomerPageHeader } from '@freight-platform/ui'
import TrackingBoard from '../../../components/tracking/TrackingBoard.vue'
import { HISTORY_PAGE_SIZE, type ETAHistoryItem, type ETASummary, type LocationFact, type SlotHistoryItem, type SlotSummary, type TrackingSummary } from '../../../domain/tracking'
import { viewKindFromError, type PortalViewKind } from '../../../domain/viewState'

const route = useRoute()
const { t } = useI18n()
const api = useShipperApi()
const shipmentId = computed(() => String(route.params.id))

const trackingKind = ref<PortalViewKind>('loading')
const tracking = ref<TrackingSummary | null>(null)
const trackingDetail = ref('')

const locationsKind = ref<PortalViewKind>('loading')
const locations = ref<LocationFact[]>([])
const locationsDetail = ref('')
const locationsTotal = ref(0)
const locationsOffset = ref(0)
const locationFrom = ref('')
const locationTo = ref('')

const etaKind = ref<PortalViewKind>('loading')
const eta = ref<ETASummary | null>(null)
const etaDetail = ref('')

const etaHistoryKind = ref<PortalViewKind>('loading')
const etaHistoryDetail = ref('')
const pickupEtaHistory = ref<ETAHistoryItem[]>([])
const deliveryEtaHistory = ref<ETAHistoryItem[]>([])

const slotsKind = ref<PortalViewKind>('loading')
const slots = ref<SlotSummary | null>(null)
const slotsDetail = ref('')

const slotHistoryKind = ref<PortalViewKind>('loading')
const slotHistoryDetail = ref('')
const pickupSlotHistory = ref<SlotHistoryItem[]>([])
const deliverySlotHistory = ref<SlotHistoryItem[]>([])

function titleFor(state: PortalViewKind, emptyKey: string) {
  if (state === 'empty') return t(emptyKey)
  if (state === 'forbidden') return t('shipper.forbidden')
  if (state === 'not_found') return t('shipper.trackingNotFound')
  if (state === 'unauthorized') return t('shipper.forbidden')
  if (state === 'loading') return t('shipper.loading')
  return t('shipper.unavailable')
}

async function loadTracking() {
  trackingKind.value = 'loading'
  tracking.value = null
  trackingDetail.value = ''
  try {
    tracking.value = await api.getTracking(shipmentId.value)
    trackingKind.value = 'ready'
  } catch (error) {
    tracking.value = null
    trackingKind.value = viewKindFromError(error)
    trackingDetail.value = error instanceof Error ? error.message : ''
  }
}

async function loadLocations(offset = locationsOffset.value) {
  locationsKind.value = 'loading'
  locations.value = []
  locationsDetail.value = ''
  locationsOffset.value = offset
  try {
    const page = await api.listTrackingLocations(shipmentId.value, {
      from: locationFrom.value,
      to: locationTo.value,
      limit: HISTORY_PAGE_SIZE,
      offset,
    })
    locations.value = page.items ?? []
    locationsTotal.value = page.total ?? locations.value.length
    locationsKind.value = 'ready'
  } catch (error) {
    locations.value = []
    locationsKind.value = viewKindFromError(error)
    locationsDetail.value = error instanceof Error ? error.message : ''
  }
}

async function loadEta() {
  etaKind.value = 'loading'
  eta.value = null
  etaDetail.value = ''
  try {
    eta.value = await api.getETA(shipmentId.value)
    etaKind.value = 'ready'
  } catch (error) {
    eta.value = null
    etaKind.value = viewKindFromError(error)
    etaDetail.value = error instanceof Error ? error.message : ''
  }
}

async function loadEtaHistory() {
  etaHistoryKind.value = 'loading'
  pickupEtaHistory.value = []
  deliveryEtaHistory.value = []
  etaHistoryDetail.value = ''
  try {
    const [pickup, delivery] = await Promise.all([
      api.listETAHistory(shipmentId.value, { targetType: 'pickup', limit: HISTORY_PAGE_SIZE, offset: 0 }),
      api.listETAHistory(shipmentId.value, { targetType: 'delivery', limit: HISTORY_PAGE_SIZE, offset: 0 }),
    ])
    pickupEtaHistory.value = pickup.items ?? []
    deliveryEtaHistory.value = delivery.items ?? []
    etaHistoryKind.value = 'ready'
  } catch (error) {
    pickupEtaHistory.value = []
    deliveryEtaHistory.value = []
    etaHistoryKind.value = viewKindFromError(error)
    etaHistoryDetail.value = error instanceof Error ? error.message : ''
  }
}

async function loadSlots() {
  slotsKind.value = 'loading'
  slots.value = null
  slotsDetail.value = ''
  try {
    slots.value = await api.getSlots(shipmentId.value)
    slotsKind.value = 'ready'
  } catch (error) {
    slots.value = null
    slotsKind.value = viewKindFromError(error)
    slotsDetail.value = error instanceof Error ? error.message : ''
  }
}

async function loadSlotHistory() {
  slotHistoryKind.value = 'loading'
  pickupSlotHistory.value = []
  deliverySlotHistory.value = []
  slotHistoryDetail.value = ''
  try {
    const [pickup, delivery] = await Promise.all([
      api.listSlotHistory(shipmentId.value, { slotType: 'pickup', limit: HISTORY_PAGE_SIZE, offset: 0 }),
      api.listSlotHistory(shipmentId.value, { slotType: 'delivery', limit: HISTORY_PAGE_SIZE, offset: 0 }),
    ])
    pickupSlotHistory.value = pickup.items ?? []
    deliverySlotHistory.value = delivery.items ?? []
    slotHistoryKind.value = 'ready'
  } catch (error) {
    pickupSlotHistory.value = []
    deliverySlotHistory.value = []
    slotHistoryKind.value = viewKindFromError(error)
    slotHistoryDetail.value = error instanceof Error ? error.message : ''
  }
}

function onLocationFilter(range: { from: string; to: string }) {
  locationFrom.value = range.from
  locationTo.value = range.to
  void loadLocations(0)
}

onMounted(() => {
  void loadTracking()
  void loadLocations(0)
  void loadEta()
  void loadEtaHistory()
  void loadSlots()
  void loadSlotHistory()
})
</script>

<template>
  <section data-testid="shipment-tracking">
    <CustomerPageHeader :title="t('shipper.trackingPageTitle')" />
    <TrackingBoard
      :tracking-kind="trackingKind"
      :tracking="tracking"
      :tracking-title="titleFor(trackingKind, 'shipper.trackingNotConfigured')"
      :tracking-detail="trackingDetail"
      :locations-kind="locationsKind"
      :locations="locations"
      :locations-title="titleFor(locationsKind, 'shipper.noLocations')"
      :locations-detail="locationsDetail"
      :locations-total="locationsTotal"
      :locations-offset="locationsOffset"
      :eta-kind="etaKind"
      :eta="eta"
      :eta-title="titleFor(etaKind, 'shipper.forecastAbsent')"
      :eta-detail="etaDetail"
      :eta-history-kind="etaHistoryKind"
      :eta-history-title="titleFor(etaHistoryKind, 'shipper.noEtaHistory')"
      :eta-history-detail="etaHistoryDetail"
      :pickup-eta-history="pickupEtaHistory"
      :delivery-eta-history="deliveryEtaHistory"
      :slots-kind="slotsKind"
      :slots="slots"
      :slots-title="titleFor(slotsKind, 'shipper.windowAbsent')"
      :slots-detail="slotsDetail"
      :slot-history-kind="slotHistoryKind"
      :slot-history-title="titleFor(slotHistoryKind, 'shipper.noSlotHistory')"
      :slot-history-detail="slotHistoryDetail"
      :pickup-slot-history="pickupSlotHistory"
      :delivery-slot-history="deliverySlotHistory"
      @locations-page="loadLocations"
      @locations-filter="onLocationFilter"
    />
  </section>
</template>
