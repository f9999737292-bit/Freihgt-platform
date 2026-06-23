<script setup lang="ts">
import {
  formatAuditFieldList,
  formatJsonValue,
  formatLowCodeDate,
  parseMigrationAuditPayload,
  type AuditEventItem,
} from '~/types/lowCode'

const props = withDefaults(
  defineProps<{
    event: AuditEventItem
    compact?: boolean
  }>(),
  { compact: false },
)

const { t } = useI18n()

const payload = computed(() => parseMigrationAuditPayload(props.event))
const showRaw = ref(false)
const badgeLabel = computed(() => t('lowCode.auditMigrations'))

const emptyLabel = computed(() => t('lowCode.migrationNone'))

function formatFields(fields: string[]) {
  return formatAuditFieldList(fields, emptyLabel.value)
}
</script>

<template>
  <article class="migration-audit-card" :class="{ 'migration-audit-card--compact': compact }">
    <header class="migration-audit-card__header">
      <div class="migration-audit-card__title-row">
        <UiBadge :status="badgeLabel" tone="info" />
        <h4 class="migration-audit-card__title">{{ $t('lowCode.auditMigratedToActiveTemplate') }}</h4>
      </div>
      <time class="migration-audit-card__time">{{ formatLowCodeDate(event.created_at) }}</time>
    </header>

    <dl class="migration-audit-card__grid">
      <div>
        <dt>{{ $t('lowCode.entityType') }}</dt>
        <dd>{{ event.entity_type }}</dd>
      </div>
      <div>
        <dt>{{ $t('lowCode.entityId') }}</dt>
        <dd class="mono">{{ event.entity_id }}</dd>
      </div>
      <div v-if="event.actor">
        <dt>{{ $t('lowCode.auditActor') }}</dt>
        <dd class="mono">{{ event.actor }}</dd>
      </div>
      <div v-if="event.request_id">
        <dt>{{ $t('lowCode.auditRequestId') }}</dt>
        <dd class="mono">{{ event.request_id }}</dd>
      </div>
      <div>
        <dt>{{ $t('lowCode.auditSourceTemplate') }}</dt>
        <dd class="mono">{{ payload.sourceTemplateId || emptyLabel }}</dd>
      </div>
      <div>
        <dt>{{ $t('lowCode.auditTargetTemplate') }}</dt>
        <dd class="mono">{{ payload.targetTemplateId || emptyLabel }}</dd>
      </div>
      <div v-if="payload.status">
        <dt>{{ $t('lowCode.migrationStatus') }}</dt>
        <dd>{{ payload.status }}</dd>
      </div>
      <div v-if="payload.allowWarnings != null">
        <dt>{{ $t('lowCode.auditAllowWarnings') }}</dt>
        <dd>{{ payload.allowWarnings ? $t('lowCode.yes') : $t('lowCode.no') }}</dd>
      </div>
    </dl>

    <div v-if="!compact" class="migration-audit-card__sections">
      <section>
        <h5>{{ $t('lowCode.migrationCopiedFields') }}</h5>
        <p>{{ formatFields(payload.copiedFields) }}</p>
      </section>
      <section>
        <h5>{{ $t('lowCode.migrationLegacyFields') }}</h5>
        <p>{{ formatFields(payload.legacyFields) }}</p>
      </section>
      <section>
        <h5>{{ $t('lowCode.migrationMissingRequiredFields') }}</h5>
        <p>{{ formatFields(payload.missingRequiredFields) }}</p>
      </section>
      <section>
        <h5>{{ $t('lowCode.migrationIncompatibleFields') }}</h5>
        <p v-if="payload.incompatibleFields.length === 0">{{ emptyLabel }}</p>
        <ul v-else>
          <li v-for="field in payload.incompatibleFields" :key="field.field_code">
            <code>{{ field.field_code }}</code> — {{ field.reason }}
          </li>
        </ul>
      </section>
      <section>
        <h5>{{ $t('lowCode.migrationWarningsSection') }}</h5>
        <p v-if="payload.warnings.length === 0">{{ $t('lowCode.migrationNoIssues') }}</p>
        <ul v-else>
          <li v-for="(warning, index) in payload.warnings" :key="`${warning}-${index}`">
            {{ warning }}
          </li>
        </ul>
      </section>
    </div>

    <div v-else class="migration-audit-card__compact-summary">
      <p>
        <span class="migration-audit-card__compact-label">{{ $t('lowCode.migrationCopiedFields') }}:</span>
        {{ formatFields(payload.copiedFields) }}
      </p>
      <p v-if="payload.legacyFields.length">
        <span class="migration-audit-card__compact-label">{{ $t('lowCode.migrationLegacyFields') }}:</span>
        {{ formatFields(payload.legacyFields) }}
      </p>
    </div>

    <details class="migration-audit-card__raw" :open="showRaw" @toggle="showRaw = ($event.target as HTMLDetailsElement).open">
      <summary>{{ showRaw ? $t('lowCode.auditHideDetails') : $t('lowCode.auditShowDetails') }}</summary>
      <p class="migration-audit-card__raw-label">{{ $t('lowCode.auditRawEvent') }}</p>
      <pre>{{ formatJsonValue(event) }}</pre>
    </details>
  </article>
</template>

<style scoped>
.migration-audit-card {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
  padding: 1rem 1.125rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}

.migration-audit-card--compact {
  padding: 0.875rem 1rem;
  gap: 0.75rem;
}

.migration-audit-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.migration-audit-card__title-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.migration-audit-card__title {
  margin: 0;
  font-size: 0.9375rem;
  font-weight: 600;
}

.migration-audit-card__time {
  font-size: 0.8125rem;
  color: var(--color-text-muted);
  white-space: nowrap;
}

.migration-audit-card__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.75rem 1rem;
  margin: 0;
}

.migration-audit-card__grid dt {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.migration-audit-card__grid dd {
  margin: 0.125rem 0 0;
  font-size: 0.875rem;
  word-break: break-word;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.8125rem;
}

.migration-audit-card__sections {
  display: grid;
  gap: 0.75rem;
}

.migration-audit-card__sections h5 {
  margin: 0 0 0.25rem;
  font-size: 0.8125rem;
  font-weight: 600;
}

.migration-audit-card__sections p,
.migration-audit-card__sections ul {
  margin: 0;
  font-size: 0.875rem;
}

.migration-audit-card__sections ul {
  padding-left: 1.25rem;
}

.migration-audit-card__compact-summary {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  font-size: 0.875rem;
}

.migration-audit-card__compact-label {
  color: var(--color-text-muted);
}

.migration-audit-card__raw summary {
  cursor: pointer;
  color: var(--color-primary);
  font-size: 0.875rem;
}

.migration-audit-card__raw-label {
  margin: 0.5rem 0 0.25rem;
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.migration-audit-card__raw pre {
  margin: 0;
  max-height: 240px;
  overflow: auto;
  padding: 0.75rem;
  background: var(--color-surface-muted, #f8fafc);
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
