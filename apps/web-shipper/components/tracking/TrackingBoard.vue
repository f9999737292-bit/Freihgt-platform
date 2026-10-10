<script setup lang="ts">
import { ref } from 'vue'
import { Button, Card, Input, Table } from '@freight-platform/ui'
import PortalState from '../PortalState.vue'
import EtaFacts from './EtaFacts.vue'
import EtaHistoryTable from './EtaHistoryTable.vue'
import SlotFacts from './SlotFacts.vue'
import SlotHistoryTable from './SlotHistoryTable.vue'
import {
  displayValue,
  HISTORY_PAGE_SIZE,
  isNotConfigured,
  type ETAHistoryItem,
  type ETASummary,
  type LocationFact,
  type SlotHistoryItem,
  type SlotSummary,
  type TrackingSummary,
} from '../../domain/tracking'
import type { PortalViewKind } from '../../domain/viewState'

defineProps<{
  trackingKind: PortalViewKind
  tracking: TrackingSummary | null
  trackingTitle: string
  trackingDetail?: string
  locationsKind: PortalViewKind
  locations: LocationFact[]
  locationsTitle: string
  locationsDetail?: string
  locationsTotal: number
  locationsOffset: number
  etaKind: PortalViewKind
  eta: ETASummary | null
  etaTitle: string
  etaDetail?: string
  etaHistoryKind: PortalViewKind
  etaHistoryTitle: string
  etaHistoryDetail?: string
  pickupEtaHistory: ETAHistoryItem[]
  deliveryEtaHistory: ETAHistoryItem[]
  slotsKind: PortalViewKind
  slots: SlotSummary | null
  slotsTitle: string
  slotsDetail?: string
  slotHistoryKind: PortalViewKind
  slotHistoryTitle: string
  slotHistoryDetail?: string
  pickupSlotHistory: SlotHistoryItem[]
  deliverySlotHistory: SlotHistoryItem[]
}>()

const emit = defineEmits<{
  'locations-page': [offset: number]
  'locations-filter': [range: { from: string; to: string }]
}>()

const { t } = useI18n()
const locationFrom = ref('')
const locationTo = ref('')

function applyLocationRange() {
  emit('locations-filter', { from: locationFrom.value, to: locationTo.value })
}
</script>

<template>
  <div class="board" data-testid="tracking-board">
    <section class="section" data-testid="tracking-section" aria-labelledby="tracking-heading">
      <h2 id="tracking-heading">{{ t('shipper.trackingSection') }}</h2>
      <PortalState v-if="trackingKind !== 'ready'" :kind="trackingKind" :title="trackingTitle" :description="trackingDetail" />
      <Card v-else-if="tracking">
        <p v-if="isNotConfigured(tracking.trackingStatus)" data-testid="tracking-not-configured">
          {{ t('shipper.trackingNotConfigured') }}
        </p>
        <dl class="facts">
          <div>
            <dt>{{ t('shipper.trackingStatus') }}</dt>
            <dd data-testid="tracking-status">{{ displayValue(tracking.trackingStatus) }}</dd>
          </div>
          <div>
            <dt>{{ t('shipper.freshness') }}</dt>
            <dd data-testid="tracking-freshness">{{ displayValue(tracking.freshness?.status) }}</dd>
          </div>
          <div v-if="tracking.freshness?.ageSeconds !== undefined">
            <dt>{{ t('shipper.ageSeconds') }}</dt>
            <dd>{{ displayValue(tracking.freshness.ageSeconds) }}</dd>
          </div>
          <div>
            <dt>{{ t('shipper.quality') }}</dt>
            <dd data-testid="tracking-quality">{{ displayValue(tracking.quality?.status) }}</dd>
          </div>
          <div v-if="tracking.speedKph !== undefined">
            <dt>{{ t('shipper.speed') }}</dt>
            <dd>{{ displayValue(tracking.speedKph) }}</dd>
          </div>
          <div v-if="tracking.headingDegrees !== undefined">
            <dt>{{ t('shipper.heading') }}</dt>
            <dd>{{ displayValue(tracking.headingDegrees) }}</dd>
          </div>
          <div v-if="tracking.lastRecordedAt">
            <dt>{{ t('shipper.lastRecordedAt') }}</dt>
            <dd>{{ tracking.lastRecordedAt }}</dd>
          </div>
          <div v-if="tracking.lastReceivedAt">
            <dt>{{ t('shipper.lastReceivedAt') }}</dt>
            <dd>{{ tracking.lastReceivedAt }}</dd>
          </div>
          <div v-if="tracking.deliveryDelaySeconds !== undefined">
            <dt>{{ t('shipper.deliveryDelay') }}</dt>
            <dd>{{ displayValue(tracking.deliveryDelaySeconds) }}</dd>
          </div>
        </dl>
        <h3>{{ t('shipper.lastKnownPosition') }}</h3>
        <p v-if="!tracking.lastKnownPosition" data-testid="position-absent">{{ t('shipper.noPosition') }}</p>
        <dl v-else class="facts" data-testid="last-known-position">
          <div>
            <dt>{{ t('shipper.latitude') }}</dt>
            <dd data-testid="position-latitude">{{ displayValue(tracking.lastKnownPosition.latitude) }}</dd>
          </div>
          <div>
            <dt>{{ t('shipper.longitude') }}</dt>
            <dd data-testid="position-longitude">{{ displayValue(tracking.lastKnownPosition.longitude) }}</dd>
          </div>
          <div>
            <dt>{{ t('shipper.recordedAt') }}</dt>
            <dd>{{ displayValue(tracking.lastKnownPosition.recordedAt) }}</dd>
          </div>
          <div>
            <dt>{{ t('shipper.ageSeconds') }}</dt>
            <dd>{{ displayValue(tracking.lastKnownPosition.ageSeconds) }}</dd>
          </div>
        </dl>
      </Card>
    </section>

    <section class="section" data-testid="location-history" aria-labelledby="locations-heading">
      <h2 id="locations-heading">{{ t('shipper.locationHistory') }}</h2>
      <form class="filters" @submit.prevent="applyLocationRange">
        <Input v-model="locationFrom" name="from" data-testid="locations-from" :label="t('shipper.periodFrom')" />
        <Input v-model="locationTo" name="to" data-testid="locations-to" :label="t('shipper.periodTo')" />
        <Button type="submit" variant="ghost">{{ t('shipper.apply') }}</Button>
      </form>
      <PortalState v-if="locationsKind !== 'ready'" :kind="locationsKind" :title="locationsTitle" :description="locationsDetail" />
      <Card v-else>
        <p v-if="locations.length === 0" data-testid="locations-empty">{{ t('shipper.noLocations') }}</p>
        <Table
          v-else
          :columns="[
            t('shipper.recordedAt'),
            t('shipper.latitude'),
            t('shipper.longitude'),
            t('shipper.speed'),
            t('shipper.heading'),
            t('shipper.accuracy'),
            t('shipper.quality'),
          ]"
        >
          <tr v-for="(item, index) in locations" :key="`${item.recordedAt ?? ''}-${index}`" data-testid="location-row">
            <td>{{ displayValue(item.recordedAt) }}</td>
            <td>{{ displayValue(item.latitude) }}</td>
            <td>{{ displayValue(item.longitude) }}</td>
            <td>{{ displayValue(item.speedKph) }}</td>
            <td>{{ displayValue(item.headingDegrees) }}</td>
            <td>{{ displayValue(item.accuracyMeters) }}</td>
            <td>{{ displayValue(item.quality?.status) }}</td>
          </tr>
        </Table>
        <div class="pager">
          <Button
            v-if="locationsOffset > 0"
            type="button"
            variant="ghost"
            data-testid="locations-prev"
            @click="emit('locations-page', Math.max(0, locationsOffset - HISTORY_PAGE_SIZE))"
          >
            {{ t('shipper.previousPage') }}
          </Button>
          <Button
            v-if="locationsOffset + locations.length < locationsTotal"
            type="button"
            variant="ghost"
            data-testid="locations-next"
            @click="emit('locations-page', locationsOffset + locations.length)"
          >
            {{ t('shipper.nextPage') }}
          </Button>
        </div>
      </Card>
    </section>

    <section class="section" data-testid="eta-section" aria-labelledby="eta-heading">
      <h2 id="eta-heading">{{ t('shipper.etaSection') }}</h2>
      <PortalState v-if="etaKind !== 'ready'" :kind="etaKind" :title="etaTitle" :description="etaDetail" />
      <div v-else class="pair">
        <Card data-testid="eta-pickup">
          <h3>{{ t('shipper.pickupForecast') }}</h3>
          <p v-if="!eta?.pickup" data-testid="eta-pickup-absent">{{ t('shipper.forecastAbsent') }}</p>
          <EtaFacts v-else :target="eta.pickup" />
        </Card>
        <Card data-testid="eta-delivery">
          <h3>{{ t('shipper.deliveryForecast') }}</h3>
          <p v-if="!eta?.delivery" data-testid="eta-delivery-absent">{{ t('shipper.forecastAbsent') }}</p>
          <EtaFacts v-else :target="eta.delivery" />
        </Card>
      </div>
      <h3>{{ t('shipper.etaHistory') }}</h3>
      <PortalState v-if="etaHistoryKind !== 'ready'" :kind="etaHistoryKind" :title="etaHistoryTitle" :description="etaHistoryDetail" />
      <div v-else class="pair">
        <EtaHistoryTable
          test-id="eta-history-pickup"
          :title="t('shipper.pickupForecast')"
          :items="pickupEtaHistory"
          :empty="t('shipper.noEtaHistory')"
        />
        <EtaHistoryTable
          test-id="eta-history-delivery"
          :title="t('shipper.deliveryForecast')"
          :items="deliveryEtaHistory"
          :empty="t('shipper.noEtaHistory')"
        />
      </div>
    </section>

    <section class="section" data-testid="slots-section" aria-labelledby="slots-heading">
      <h2 id="slots-heading">{{ t('shipper.slotsSection') }}</h2>
      <PortalState v-if="slotsKind !== 'ready'" :kind="slotsKind" :title="slotsTitle" :description="slotsDetail" />
      <div v-else class="pair">
        <Card data-testid="slot-pickup">
          <h3>{{ t('shipper.pickupWindow') }}</h3>
          <p v-if="!slots?.pickup" data-testid="slot-pickup-absent">{{ t('shipper.windowAbsent') }}</p>
          <SlotFacts v-else :target="slots.pickup" />
        </Card>
        <Card data-testid="slot-delivery">
          <h3>{{ t('shipper.deliveryWindow') }}</h3>
          <p v-if="!slots?.delivery" data-testid="slot-delivery-absent">{{ t('shipper.windowAbsent') }}</p>
          <SlotFacts v-else :target="slots.delivery" />
        </Card>
      </div>
      <h3>{{ t('shipper.slotHistory') }}</h3>
      <PortalState v-if="slotHistoryKind !== 'ready'" :kind="slotHistoryKind" :title="slotHistoryTitle" :description="slotHistoryDetail" />
      <div v-else class="pair">
        <SlotHistoryTable
          test-id="slot-history-pickup"
          :title="t('shipper.pickupWindow')"
          :items="pickupSlotHistory"
          :empty="t('shipper.noSlotHistory')"
        />
        <SlotHistoryTable
          test-id="slot-history-delivery"
          :title="t('shipper.deliveryWindow')"
          :items="deliverySlotHistory"
          :empty="t('shipper.noSlotHistory')"
        />
      </div>
    </section>
  </div>
</template>

<style scoped>
.board {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  min-width: 0;
  max-width: 100%;
}
.section {
  min-width: 0;
  max-width: 100%;
}
.pair,
.facts {
  display: grid;
  gap: 0.75rem;
  min-width: 0;
}
.pair {
  grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
}
.facts {
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  margin: 0;
}
.facts dt {
  color: #6b7280;
  font-size: 0.75rem;
  font-weight: 600;
}
.facts dd {
  margin: 0.15rem 0 0;
}
.filters,
.pager {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.75rem;
}
.filters {
  margin-bottom: 0.75rem;
}
.board :deep(a:focus-visible),
.board :deep(button:focus-visible),
.board :deep(input:focus-visible),
.board :deep(select:focus-visible) {
  outline: 2px solid #0f3d4c;
  outline-offset: 2px;
}
</style>
