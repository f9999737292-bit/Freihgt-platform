<script setup lang="ts">
import {
  formatCustomFieldDisplayValue,
  formatLowCodeDate,
  isCustomFieldComplexValue,
  type LowCodeEntityType,
} from '~/types/lowCode'
import { TenantRequiredError } from '~/composables/useApi'

const props = withDefaults(
  defineProps<{
    entityType: LowCodeEntityType | string
    entityId?: string | null
    title?: string
  }>(),
  {
    entityId: null,
    title: undefined,
  },
)

const { getCustomFieldValues, isApiUnavailableError } = useLowCodeApi()
const { hasTenant } = useTenantContext()
const { t } = useI18n()

const loading = ref(false)
const loaded = ref(false)
const loadFailed = ref(false)
const items = ref<Array<{ field_id: string; field_code: string; value_json: unknown; updated_at: string }>>([])

const panelTitle = computed(() => props.title || t('lowCode.customFields'))
const canLoad = computed(() => hasTenant.value && !!props.entityId?.trim())

async function load() {
  if (!canLoad.value) {
    loaded.value = false
    items.value = []
    return
  }

  loading.value = true
  loadFailed.value = false
  loaded.value = false
  try {
    const data = await getCustomFieldValues(props.entityType, props.entityId!.trim())
    items.value = data.items
    loaded.value = true
  } catch (error) {
    items.value = []
    if (error instanceof TenantRequiredError) return
    loadFailed.value = isApiUnavailableError(error)
    loaded.value = true
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.entityType, props.entityId] as const,
  () => {
    load()
  },
  { immediate: true },
)
</script>

<template>
  <UiCard class="low-code-panel">
    <template #header>
      <div class="low-code-panel__header">
        <h3 class="low-code-panel__title">{{ panelTitle }}</h3>
        <UiBadge status="read-only" tone="neutral">{{ $t('lowCode.readOnly') }}</UiBadge>
      </div>
    </template>

    <p class="low-code-panel__hint">{{ $t('lowCode.lowCodeFieldsHint') }}</p>

    <div v-if="!canLoad" class="low-code-panel__muted">{{ $t('lowCode.entityIdRequired') }}</div>

    <div v-else-if="loading" class="low-code-panel__muted">{{ $t('common.loading') }}</div>

    <CommonApiUnavailableState
      v-else-if="loadFailed"
      :title="$t('lowCode.customFieldsLoadFailed')"
      :message="$t('lowCode.serviceUnavailable')"
      @retry="load"
    />

    <UiEmptyState v-else-if="loaded && !items.length" :title="$t('lowCode.noCustomFieldsFound')" />

    <UiTable
      v-else-if="items.length"
      :columns="[$t('lowCode.field'), $t('lowCode.value'), $t('lowCode.updatedAt')]"
    >
      <tr v-for="item in items" :key="item.field_id">
        <td><code>{{ item.field_code }}</code></td>
        <td>
          <span
            v-if="!isCustomFieldComplexValue(item.value_json)"
            class="low-code-panel__value"
          >
            {{ formatCustomFieldDisplayValue(item.value_json) }}
          </span>
          <pre v-else class="low-code-panel__value-json">{{ formatCustomFieldDisplayValue(item.value_json) }}</pre>
        </td>
        <td>{{ formatLowCodeDate(item.updated_at) }}</td>
      </tr>
    </UiTable>
  </UiCard>
</template>

<style scoped>
.low-code-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.low-code-panel__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
}

.low-code-panel__hint {
  margin: 0 0 1rem;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.low-code-panel__muted {
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.low-code-panel__value {
  font-size: 0.875rem;
}

.low-code-panel__value-json {
  margin: 0;
  font-size: 0.75rem;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
