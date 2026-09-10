<script setup lang="ts">
import type { RfxCompareVersionsResponse } from '~/types/rfx-version-lifecycle'
import {
  compareItemLabel,
  filterVisibleCompareItems,
  groupCompareItemsByChange,
  isCompareEmpty,
  mergeCompareSections,
} from '~/utils/rfxVersionCompare'

const props = defineProps<{
  compare: RfxCompareVersionsResponse | null
  showUnchanged?: boolean
}>()

const { t } = useI18n()

const summary = computed(() => props.compare?.summary)
const isEmpty = computed(() => (summary.value ? isCompareEmpty(summary.value) : true))

const visibleItems = computed(() => {
  if (!props.compare) return []
  const merged = mergeCompareSections(props.compare)
  return props.showUnchanged ? merged : filterVisibleCompareItems(merged)
})

const grouped = computed(() => groupCompareItemsByChange(visibleItems.value))

function changeLabel(change: string) {
  return t(`rfx.compare.change.${change}`)
}
</script>

<template>
  <div v-if="!compare" class="compare-view compare-view--empty">
    {{ $t('rfx.compare.noData') }}
  </div>

  <div v-else class="compare-view">
    <header class="compare-view__header">
      <p class="compare-view__direction">
        {{ $t('rfx.compare.direction', {
          from: compare.source_version_number,
          to: compare.target_version_number,
        }) }}
      </p>
      <p v-if="compare.canonical_diff_hash" class="compare-view__hash">
        {{ $t('rfx.compare.diffHash') }}: <code>{{ compare.canonical_diff_hash }}</code>
      </p>
    </header>

    <p v-if="isEmpty" class="compare-view__no-diff">{{ $t('rfx.compare.noDifferences') }}</p>

    <div v-else class="compare-view__summary">
      <span>{{ $t('rfx.compare.added') }}: {{ summary?.added_count ?? 0 }}</span>
      <span>{{ $t('rfx.compare.removed') }}: {{ summary?.removed_count ?? 0 }}</span>
      <span>{{ $t('rfx.compare.changed') }}: {{ summary?.changed_count ?? 0 }}</span>
      <span>{{ $t('rfx.compare.reordered') }}: {{ summary?.reordered_count ?? 0 }}</span>
    </div>

    <section
      v-for="changeType in ['ADDED', 'REMOVED', 'CHANGED', 'REORDERED', 'UNCHANGED']"
      :key="changeType"
      class="compare-view__group"
    >
      <template v-if="grouped[changeType as keyof typeof grouped]?.length">
        <h3>{{ changeLabel(changeType) }}</h3>
        <ul>
          <li v-for="(item, idx) in grouped[changeType as keyof typeof grouped]" :key="`${changeType}-${idx}`">
            <strong>{{ item.entity_type }}</strong>
            — {{ compareItemLabel(item) }}
            <span v-if="item.fields?.length" class="compare-view__fields">
              ({{ item.fields.join(', ') }})
            </span>
          </li>
        </ul>
      </template>
    </section>
  </div>
</template>

<style scoped>
.compare-view { display: flex; flex-direction: column; gap: 1rem; }
.compare-view__header { border-bottom: 1px solid var(--color-border); padding-bottom: 0.75rem; }
.compare-view__direction { font-weight: 600; margin: 0; }
.compare-view__hash { font-size: 0.875rem; color: var(--color-text-muted); margin: 0.25rem 0 0; }
.compare-view__summary { display: flex; flex-wrap: wrap; gap: 1rem; font-size: 0.875rem; }
.compare-view__group h3 { margin: 0 0 0.5rem; font-size: 0.9375rem; }
.compare-view__group ul { margin: 0; padding-left: 1.25rem; }
.compare-view__fields { color: var(--color-text-muted); font-size: 0.8125rem; }
.compare-view__no-diff { font-style: italic; color: var(--color-text-muted); }
</style>
