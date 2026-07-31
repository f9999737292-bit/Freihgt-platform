<script setup lang="ts">
import { CONTROL_TOWER_AUTO_REFRESH_INTERVALS } from '~/types/controlTower'

const props = defineProps<{
  lastUpdatedAt: string | null
  backendOnline: boolean
  loading: boolean
  autoRefreshEnabled: boolean
  autoRefreshIntervalMs: number
}>()

const emit = defineEmits<{
  refresh: []
  'update:autoRefreshEnabled': [value: boolean]
  'update:autoRefreshIntervalMs': [value: number]
}>()

const { t } = useI18n()

const intervalOptions = computed(() =>
  CONTROL_TOWER_AUTO_REFRESH_INTERVALS.map((item) => ({
    label: t(`controlTower.autoRefresh.intervals.${item.key}`),
    value: String(item.ms),
  })),
)

const formattedUpdatedAt = computed(() => {
  if (!props.lastUpdatedAt) return '—'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(props.lastUpdatedAt))
})

function onIntervalChange(value: string) {
  emit('update:autoRefreshIntervalMs', Number(value))
}
</script>

<template>
  <header class="ct-toolbar">
    <div>
      <h1 class="ct-toolbar__title">{{ $t('controlTower.pageTitle') }}</h1>
      <p class="ct-toolbar__meta">
        <span>{{ $t('controlTower.lastUpdated') }}: {{ formattedUpdatedAt }}</span>
        <span class="ct-toolbar__dot">·</span>
        <span>
          {{ $t('controlTower.backendConnection') }}:
          {{
            backendOnline
              ? $t('controlTower.backendOnline')
              : $t('controlTower.backendOfflineShort')
          }}
        </span>
      </p>
    </div>

    <div class="ct-toolbar__actions">
      <label class="ct-toolbar__auto">
        <input
          type="checkbox"
          :checked="autoRefreshEnabled"
          @change="emit('update:autoRefreshEnabled', ($event.target as HTMLInputElement).checked)"
        />
        {{ $t('controlTower.autoRefresh.label') }}
      </label>
      <UiSelect
        v-if="autoRefreshEnabled"
        :model-value="String(autoRefreshIntervalMs)"
        :label="$t('controlTower.autoRefresh.interval')"
        :options="intervalOptions"
        @update:model-value="onIntervalChange"
      />
      <UiButton size="sm" variant="secondary" :disabled="loading" @click="emit('refresh')">
        {{ loading ? $t('common.loading') : $t('controlTower.refresh') }}
      </UiButton>
    </div>
  </header>
</template>

<style scoped>
.ct-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.ct-toolbar__title {
  margin: 0;
  font-size: 1.75rem;
}

.ct-toolbar__meta {
  margin: 0.35rem 0 0;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.ct-toolbar__dot {
  margin: 0 0.35rem;
}

.ct-toolbar__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.75rem;
}

.ct-toolbar__auto {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
  padding-bottom: 0.35rem;
}
</style>
