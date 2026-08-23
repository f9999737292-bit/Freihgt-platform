<script setup lang="ts">
import type { FreightCostAnalyticsFreshnessDTO } from '~/types/freightCost'
import {
  dataQualityLabelKey,
  dataQualityTone,
  shouldShowIntelligenceDataQualityBanner,
} from '~/utils/freightCostIntelligence'

const props = defineProps<{
  dataQuality?: string | null
  mixedCurrency?: boolean
  freshness?: FreightCostAnalyticsFreshnessDTO | null
}>()

const { t } = useI18n()

const showBanner = computed(() => shouldShowIntelligenceDataQualityBanner(
  props.dataQuality,
  props.mixedCurrency ?? false,
))

const bannerMessageKey = computed(() => {
  if (props.mixedCurrency) return 'freightCosts.intelligence.banners.mixedCurrency'
  const quality = String(props.dataQuality ?? '').toUpperCase()
  if (quality === 'INSUFFICIENT_SAMPLE') return 'freightCosts.intelligence.banners.insufficientSample'
  if (quality === 'STALE') return 'freightCosts.intelligence.banners.stale'
  if (quality === 'NOT_AVAILABLE') return 'freightCosts.intelligence.banners.notAvailable'
  if (quality === 'PARTIAL') return 'freightCosts.intelligence.banners.partial'
  return 'freightCosts.intelligence.banners.partial'
})

const qualityLabel = computed(() => t(dataQualityLabelKey(props.dataQuality)))
const qualityTone = computed(() => dataQualityTone(props.dataQuality))
</script>

<template>
  <div v-if="showBanner" class="intelligence-banner" role="status">
    <Badge :status="qualityLabel" :tone="qualityTone" />
    <p class="intelligence-banner__message">{{ t(bannerMessageKey) }}</p>
    <p v-if="freshness?.calculated_at" class="intelligence-banner__freshness">
      {{ t('freightCosts.intelligence.freshness.calculatedAt', { at: freshness.calculated_at }) }}
    </p>
  </div>
</template>

<style scoped>
.intelligence-banner {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.75rem 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: #fffbeb;
}

.intelligence-banner__message {
  margin: 0;
  color: var(--color-text);
  font-size: 0.875rem;
}

.intelligence-banner__freshness {
  margin: 0;
  color: var(--color-text-muted);
  font-size: 0.8125rem;
}
</style>
