<script setup lang="ts">
import {
  DEMO_ENTITY_REFS,
  LOW_CODE_ENTITY_TYPES,
  formatJsonValue,
  formatLowCodeDate,
  type CustomFieldValueItem,
  type LowCodeEntityType,
} from '~/types/lowCode'
import { TenantRequiredError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { getCustomFieldValues, resolveDemoEntityId, isApiUnavailableError } = useLowCodeApi()
const { hasTenant } = useTenantContext()
const { pushToast } = useToast()
const { t } = useI18n()

const form = reactive({
  entity_type: 'TRANSPORT_ORDER' as LowCodeEntityType,
  entity_id: '',
})

const items = ref<CustomFieldValueItem[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const loaded = ref(false)
const resolvingDemo = ref(false)

const entityTypeOptions = computed(() =>
  LOW_CODE_ENTITY_TYPES.map((value) => ({ label: value, value })),
)

const demoRefHint = computed(() => DEMO_ENTITY_REFS[form.entity_type])

async function loadValues() {
  if (!hasTenant.value) return
  if (!form.entity_id.trim()) {
    pushToast('error', t('lowCode.entityIdRequired'))
    return
  }

  loading.value = true
  loadFailed.value = false
  loaded.value = false
  try {
    const data = await getCustomFieldValues(form.entity_type, form.entity_id.trim())
    items.value = data.items
    loaded.value = true
  } catch (error) {
    items.value = []
    if (error instanceof TenantRequiredError) return
    loadFailed.value = isApiUnavailableError(error)
    if (!loadFailed.value) {
      pushToast('error', error instanceof Error ? error.message : t('lowCode.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function useDemoEntity() {
  if (!hasTenant.value) return
  resolvingDemo.value = true
  try {
    const id = await resolveDemoEntityId(form.entity_type)
    if (!id) {
      pushToast('error', t('lowCode.demoEntityNotFound', { ref: demoRefHint.value }))
      return
    }
    form.entity_id = id
    await loadValues()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('lowCode.demoResolveFailed'))
  } finally {
    resolvingDemo.value = false
  }
}
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/low-code">{{ $t('lowCode.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ $t('lowCode.customFieldValues') }}</span>
    </nav>

    <UiPageHeader :title="$t('lowCode.customFieldValues')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/low-code')">{{ $t('common.back') }}</UiButton>
      </template>
    </UiPageHeader>

    <div class="low-code-hub__notice low-code-hub__notice--info">
      <strong>{{ $t('lowCode.readOnlyPreview') }}</strong>
      <p>{{ $t('lowCode.customFieldValuesReadOnlyHint') }}</p>
    </div>

    <UiCard>
      <div class="lookup-form">
        <UiSelect
          v-model="form.entity_type"
          :label="$t('lowCode.entityType')"
          :options="entityTypeOptions"
        />
        <UiInput
          v-model="form.entity_id"
          :label="$t('lowCode.entityId')"
          :placeholder="$t('lowCode.entityIdPlaceholder')"
        />
        <div class="lookup-form__actions">
          <UiButton :loading="loading" @click="loadValues">{{ $t('lowCode.loadValues') }}</UiButton>
          <UiButton variant="secondary" :loading="resolvingDemo" @click="useDemoEntity">
            {{ $t('lowCode.useDemoEntity') }}
          </UiButton>
        </div>
      </div>
      <p class="demo-hint">
        {{ $t('lowCode.demoHint', { entityType: form.entity_type, demoRef: demoRefHint }) }}
      </p>
    </UiCard>

    <CommonApiUnavailableState
      v-if="loadFailed"
      :message="$t('lowCode.serviceUnavailable')"
      @retry="loadValues"
    />

    <UiTable
      v-else-if="loaded && (items.length || loading)"
      :columns="[$t('lowCode.fieldCode'), $t('lowCode.value'), $t('lowCode.updatedAt')]"
      :loading="loading"
    >
      <tr v-for="item in items" :key="item.field_id">
        <td><code>{{ item.field_code }}</code></td>
        <td><pre class="value-pre">{{ formatJsonValue(item.value_json) }}</pre></td>
        <td>{{ formatLowCodeDate(item.updated_at) }}</td>
      </tr>
    </UiTable>

    <UiEmptyState v-else-if="loaded && !items.length" :title="$t('lowCode.noCustomFieldValuesFound')" />
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

.low-code-hub__notice {
  padding: 1rem 1.25rem;
  border-radius: var(--radius-lg);
  border: 1px solid transparent;
}

.low-code-hub__notice p {
  margin: 0.375rem 0 0;
  font-size: 0.875rem;
}

.low-code-hub__notice--info {
  background: #eff6ff;
  border-color: #bfdbfe;
  color: #1e3a8a;
}

.lookup-form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem;
  align-items: end;
}

.lookup-form__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.demo-hint {
  margin: 1rem 0 0;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}

.value-pre {
  margin: 0;
  font-size: 0.8125rem;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
