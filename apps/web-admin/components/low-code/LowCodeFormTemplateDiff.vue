<script setup lang="ts">
import {
  compareFormTemplates,
  adminDetailToCompareInput,
  draftToCompareInput,
  hasFormTemplateCompareChanges,
  type AdminFormTemplateDetail,
  type DraftFormTemplateDraft,
  type FormTemplateCompareChangeType,
  type FormTemplateCompareResult,
  type FormTemplateCompareRow,
} from '~/types/lowCode'

const props = withDefaults(
  defineProps<{
    baseTemplate?: AdminFormTemplateDetail | null
    draftTemplate?: DraftFormTemplateDraft | null
    draftVersion?: number
    compact?: boolean
    showEmptyState?: boolean
  }>(),
  {
    baseTemplate: null,
    draftTemplate: null,
    draftVersion: 0,
    compact: false,
    showEmptyState: true,
  },
)

const { t } = useI18n()

const compareResult = computed<FormTemplateCompareResult | null>(() => {
  if (!props.baseTemplate || !props.draftTemplate) return null
  return compareFormTemplates(
    adminDetailToCompareInput(props.baseTemplate),
    draftToCompareInput(props.draftTemplate, props.draftVersion),
  )
})

const hasChanges = computed(() => (compareResult.value ? hasFormTemplateCompareChanges(compareResult.value) : false))

const groupedRows = computed(() => {
  const result = compareResult.value
  if (!result) return { added: [], removed: [], changed: [] as FormTemplateCompareRow[] }
  return {
    added: result.rows.filter((row) => row.type === 'added'),
    removed: result.rows.filter((row) => row.type === 'removed'),
    changed: result.rows.filter((row) => row.type === 'changed'),
  }
})

function typeLabel(type: FormTemplateCompareChangeType) {
  if (type === 'added') return t('lowCode.compareAdded')
  if (type === 'removed') return t('lowCode.compareRemoved')
  return t('lowCode.compareChanged')
}

function areaLabel(row: FormTemplateCompareRow) {
  if (row.area === 'template') return t('lowCode.templateMetadata')
  if (row.area === 'section') return t('lowCode.section')
  return t('lowCode.field')
}

function rowTitle(row: FormTemplateCompareRow) {
  const parts = [row.code]
  if (row.sectionCode) parts.unshift(row.sectionCode)
  if (row.label) parts.push(`(${row.label})`)
  return parts.join(' / ')
}

defineExpose({ compareResult, hasChanges })
</script>

<template>
  <div class="template-diff" :class="{ 'template-diff--compact': compact }">
    <p v-if="!baseTemplate" class="template-diff__notice">
      {{ $t('lowCode.noPublishedBaseTemplateFound') }}
    </p>

    <template v-else-if="compareResult">
      <div class="template-diff__summary">
        <div class="template-diff__summary-card">
          <span class="template-diff__summary-value">{{ compareResult.summary.addedSections }}</span>
          <span class="template-diff__summary-label">{{ $t('lowCode.addedSections') }}</span>
        </div>
        <div class="template-diff__summary-card">
          <span class="template-diff__summary-value">{{ compareResult.summary.removedSections }}</span>
          <span class="template-diff__summary-label">{{ $t('lowCode.removedSections') }}</span>
        </div>
        <div class="template-diff__summary-card">
          <span class="template-diff__summary-value">{{ compareResult.summary.changedSections }}</span>
          <span class="template-diff__summary-label">{{ $t('lowCode.changedSections') }}</span>
        </div>
        <div class="template-diff__summary-card">
          <span class="template-diff__summary-value">{{ compareResult.summary.addedFields }}</span>
          <span class="template-diff__summary-label">{{ $t('lowCode.addedFields') }}</span>
        </div>
        <div class="template-diff__summary-card">
          <span class="template-diff__summary-value">{{ compareResult.summary.removedFields }}</span>
          <span class="template-diff__summary-label">{{ $t('lowCode.removedFields') }}</span>
        </div>
        <div class="template-diff__summary-card">
          <span class="template-diff__summary-value">{{ compareResult.summary.changedFields }}</span>
          <span class="template-diff__summary-label">{{ $t('lowCode.changedFields') }}</span>
        </div>
      </div>

      <p v-if="showEmptyState && !hasChanges" class="template-diff__empty">
        {{ $t('lowCode.noChangesDetected') }}
      </p>

      <div v-for="group in (['added', 'removed', 'changed'] as const)" :key="group" class="template-diff__group">
        <template v-if="groupedRows[group].length">
          <h3 class="template-diff__group-title">
            {{ typeLabel(group === 'changed' ? 'changed' : group) }}
          </h3>
          <div class="template-diff__table-wrap">
            <table class="template-diff__table">
              <thead>
                <tr>
                  <th>{{ $t('common.type') }}</th>
                  <th>{{ $t('lowCode.compareArea') }}</th>
                  <th>{{ $t('lowCode.code') }}</th>
                  <th v-if="group === 'changed'">{{ $t('lowCode.compareBefore') }}</th>
                  <th v-if="group === 'changed'">{{ $t('lowCode.compareAfter') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, index) in groupedRows[group]" :key="`${group}-${row.code}-${row.attribute ?? ''}-${index}`">
                  <td>
                    <span class="template-diff__badge" :class="`template-diff__badge--${row.type}`">
                      {{ typeLabel(row.type) }}
                    </span>
                  </td>
                  <td>{{ areaLabel(row) }}</td>
                  <td>
                    <div class="template-diff__code">{{ rowTitle(row) }}</div>
                    <div v-if="row.attribute" class="template-diff__attribute">{{ row.attribute }}</div>
                  </td>
                  <td v-if="group === 'changed'">
                    <pre v-if="row.before && (row.attribute?.includes('json') || row.before.includes('\n'))" class="template-diff__json">{{ row.before }}</pre>
                    <span v-else>{{ row.before ?? '—' }}</span>
                  </td>
                  <td v-if="group === 'changed'">
                    <pre v-if="row.after && (row.attribute?.includes('json') || row.after.includes('\n'))" class="template-diff__json">{{ row.after }}</pre>
                    <span v-else>{{ row.after ?? '—' }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>
      </div>
    </template>
  </div>
</template>

<style scoped>
.template-diff {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.template-diff__notice,
.template-diff__empty {
  margin: 0;
  padding: 0.875rem 1rem;
  border-radius: var(--radius-md);
  background: #f8fafc;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.template-diff__summary {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 0.75rem;
}

.template-diff__summary-card {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.875rem 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
}

.template-diff__summary-value {
  font-size: 1.25rem;
  font-weight: 700;
}

.template-diff__summary-label {
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.template-diff__group-title {
  margin: 0;
  font-size: 0.9375rem;
}

.template-diff__table-wrap {
  overflow-x: auto;
}

.template-diff__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.template-diff__table th,
.template-diff__table td {
  padding: 0.625rem 0.75rem;
  border-bottom: 1px solid var(--color-border);
  text-align: left;
  vertical-align: top;
}

.template-diff__table th {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.template-diff__badge {
  display: inline-flex;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}

.template-diff__badge--added {
  background: #dcfce7;
  color: #166534;
}

.template-diff__badge--removed {
  background: #fee2e2;
  color: #991b1b;
}

.template-diff__badge--changed {
  background: #fef3c7;
  color: #92400e;
}

.template-diff__code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.8125rem;
}

.template-diff__attribute {
  margin-top: 0.25rem;
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.template-diff__json {
  margin: 0;
  max-width: 280px;
  max-height: 160px;
  overflow: auto;
  padding: 0.5rem;
  border-radius: var(--radius-sm);
  background: #f8fafc;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
  white-space: pre-wrap;
  word-break: break-word;
}

.template-diff--compact .template-diff__summary {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.template-diff--compact .template-diff__table-wrap {
  max-height: 240px;
  overflow: auto;
}
</style>
