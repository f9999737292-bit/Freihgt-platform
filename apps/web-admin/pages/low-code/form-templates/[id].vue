<script setup lang="ts">
import { formatJsonValue, formatLowCodeDate, formTemplateDetailToPreview, type FormTemplateDetail } from '~/types/lowCode'

definePageMeta({ middleware: 'auth', layout: 'default' })

const route = useRoute()
const { getFormTemplate, isApiUnavailableError } = useLowCodeApi()
const { pushToast } = useToast()
const { t } = useI18n()

const template = ref<FormTemplateDetail | null>(null)
const loading = ref(true)
const apiUnavailable = ref(false)

const templateId = computed(() => String(route.params.id))

const activeView = ref<'details' | 'preview'>('details')

async function loadTemplate() {
  loading.value = true
  apiUnavailable.value = false
  try {
    template.value = await getFormTemplate(templateId.value)
  } catch (error) {
    template.value = null
    apiUnavailable.value = isApiUnavailableError(error)
    if (!apiUnavailable.value) {
      pushToast('error', error instanceof Error ? error.message : t('lowCode.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function boolLabel(value: boolean) {
  return value ? t('lowCode.yes') : t('lowCode.no')
}

const previewModel = computed(() => (template.value ? formTemplateDetailToPreview(template.value) : null))

watch(templateId, loadTemplate, { immediate: true })
</script>

<template>
  <div class="page-stack">
    <nav class="breadcrumbs" aria-label="Breadcrumb">
      <NuxtLink to="/low-code">{{ $t('lowCode.title') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <NuxtLink to="/low-code/form-templates">{{ $t('lowCode.formTemplates') }}</NuxtLink>
      <span class="breadcrumbs__sep">/</span>
      <span>{{ template?.name || $t('lowCode.templateDetails') }}</span>
    </nav>

    <UiPageHeader :title="template?.name || $t('lowCode.templateDetails')">
      <template #actions>
        <UiButton variant="secondary" @click="$router.push('/low-code/form-templates')">
          {{ $t('common.back') }}
        </UiButton>
      </template>
    </UiPageHeader>

    <div v-if="loading" class="loading-block">{{ $t('common.loading') }}</div>

    <CommonApiUnavailableState
      v-else-if="apiUnavailable"
      :message="$t('lowCode.serviceUnavailable')"
      @retry="loadTemplate"
    />

    <UiEmptyState v-else-if="!template" :title="$t('lowCode.noTemplatesFound')" />

    <template v-else>
      <UiCard>
        <template #header>{{ $t('lowCode.templateMetadata') }}</template>
        <div class="details-grid">
          <div class="details-item">
            <span class="details-item__label">{{ $t('lowCode.entityType') }}</span>
            <span>{{ template.entity_type }}</span>
          </div>
          <div class="details-item">
            <span class="details-item__label">{{ $t('lowCode.code') }}</span>
            <span><code>{{ template.code }}</code></span>
          </div>
          <div class="details-item">
            <span class="details-item__label">{{ $t('common.status') }}</span>
            <UiBadge :status="template.status" />
          </div>
          <div class="details-item">
            <span class="details-item__label">{{ $t('lowCode.version') }}</span>
            <span>{{ template.version }}</span>
          </div>
          <div class="details-item">
            <span class="details-item__label">{{ $t('lowCode.publishedAt') }}</span>
            <span>{{ formatLowCodeDate(template.published_at) }}</span>
          </div>
          <div class="details-item">
            <span class="details-item__label">ID</span>
            <span><code>{{ template.id }}</code></span>
          </div>
        </div>
      </UiCard>

      <div class="view-tabs">
        <button
          type="button"
          class="view-tabs__btn"
          :class="{ 'view-tabs__btn--active': activeView === 'details' }"
          @click="activeView = 'details'"
        >
          {{ $t('common.details') }}
        </button>
        <button
          type="button"
          class="view-tabs__btn"
          :class="{ 'view-tabs__btn--active': activeView === 'preview' }"
          @click="activeView = 'preview'"
        >
          {{ $t('lowCode.preview') }}
        </button>
      </div>

      <LowCodeFormTemplatePreview
        v-if="activeView === 'preview'"
        :template="previewModel"
        :title="$t('lowCode.formPreview')"
      />

      <template v-if="activeView === 'details'">
      <UiCard v-for="section in template.sections" :key="section.id">
        <template #header>
          {{ section.title }}
          <span class="text-muted">({{ section.code }})</span>
        </template>

        <UiTable
          :columns="[
            $t('lowCode.fieldCode'),
            $t('lowCode.label'),
            $t('lowCode.fieldType'),
            $t('lowCode.required'),
            $t('lowCode.readOnly'),
            $t('lowCode.systemField'),
          ]"
        >
          <tr v-for="field in section.fields" :key="field.id">
            <td><code>{{ field.code }}</code></td>
            <td>{{ field.label }}</td>
            <td>{{ field.field_type }}</td>
            <td>{{ boolLabel(field.required) }}</td>
            <td>{{ boolLabel(field.read_only) }}</td>
            <td>{{ boolLabel(field.system_field) }}</td>
          </tr>
        </UiTable>

        <div v-for="field in section.fields" :key="`${field.id}-json`" class="field-json-block">
          <h4>{{ field.code }}</h4>
          <details v-if="field.options_json" class="json-details">
            <summary>{{ $t('lowCode.optionsJson') }}</summary>
            <pre>{{ formatJsonValue(field.options_json) }}</pre>
          </details>
          <details v-if="field.validation_rule_json" class="json-details">
            <summary>{{ $t('lowCode.validationRuleJson') }}</summary>
            <pre>{{ formatJsonValue(field.validation_rule_json) }}</pre>
          </details>
          <details v-if="field.visibility_rule_json" class="json-details">
            <summary>{{ $t('lowCode.visibilityRuleJson') }}</summary>
            <pre>{{ formatJsonValue(field.visibility_rule_json) }}</pre>
          </details>
        </div>
      </UiCard>
      </template>
    </template>
  </div>
</template>

<style scoped>
.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-muted);
  flex-wrap: wrap;
}

.breadcrumbs__sep {
  opacity: 0.5;
}

.loading-block {
  padding: 2rem;
  text-align: center;
  color: var(--color-text-muted);
}

.view-tabs {
  display: flex;
  gap: 0.5rem;
}

.view-tabs__btn {
  padding: 0.5rem 0.875rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  font: inherit;
  cursor: pointer;
}

.view-tabs__btn--active {
  border-color: var(--color-primary);
  background: #eff6ff;
  color: var(--color-primary);
}

.details-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem;
}

.details-item {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.details-item__label {
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.field-json-block {
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--color-border);
}

.field-json-block h4 {
  margin: 0 0 0.5rem;
  font-size: 0.875rem;
}

.json-details {
  margin-bottom: 0.5rem;
}

.json-details summary {
  cursor: pointer;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.json-details pre {
  margin: 0.5rem 0 0;
  padding: 0.75rem;
  border-radius: var(--radius-md);
  background: #f8fafc;
  border: 1px solid var(--color-border);
  font-size: 0.75rem;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
