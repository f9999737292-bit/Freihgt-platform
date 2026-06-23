<script setup lang="ts">
import {
  formatJsonValue,
  formatLowCodeDate,
  isMigrationAuditEvent,
  type AuditEventItem,
} from '~/types/lowCode'

defineProps<{
  event: AuditEventItem
}>()

const { t } = useI18n()

function formatChangedFields(fields: string[]) {
  return fields?.length ? fields.join(', ') : t('lowCode.migrationNone')
}
</script>

<template>
  <LowCodeMigrationAuditCard v-if="isMigrationAuditEvent(event)" :event="event" />

  <article v-else class="audit-event-card">
    <header class="audit-event-card__header">
      <div>
        <h4 class="audit-event-card__action">{{ event.action }}</h4>
        <time class="audit-event-card__time">{{ formatLowCodeDate(event.created_at) }}</time>
      </div>
    </header>

    <dl class="audit-event-card__grid">
      <div>
        <dt>{{ $t('lowCode.entityType') }}</dt>
        <dd>{{ event.entity_type }}</dd>
      </div>
      <div>
        <dt>{{ $t('lowCode.entityId') }}</dt>
        <dd class="mono">{{ event.entity_id }}</dd>
      </div>
      <div>
        <dt>{{ $t('lowCode.auditActor') }}</dt>
        <dd class="mono">{{ event.actor || t('lowCode.migrationNone') }}</dd>
      </div>
      <div v-if="event.request_id">
        <dt>{{ $t('lowCode.auditRequestId') }}</dt>
        <dd class="mono">{{ event.request_id }}</dd>
      </div>
      <div class="audit-event-card__wide">
        <dt>{{ $t('lowCode.changedFields') }}</dt>
        <dd>{{ formatChangedFields(event.changed_fields) }}</dd>
      </div>
    </dl>

    <details class="audit-event-card__raw">
      <summary>{{ $t('lowCode.auditShowDetails') }}</summary>
      <div class="audit-event-card__raw-blocks">
        <div>
          <p class="audit-event-card__raw-label">{{ $t('lowCode.oldValues') }}</p>
          <pre>{{ formatJsonValue(event.old_values) }}</pre>
        </div>
        <div>
          <p class="audit-event-card__raw-label">{{ $t('lowCode.newValues') }}</p>
          <pre>{{ formatJsonValue(event.new_values) }}</pre>
        </div>
        <div>
          <p class="audit-event-card__raw-label">{{ $t('lowCode.auditRawEvent') }}</p>
          <pre>{{ formatJsonValue(event) }}</pre>
        </div>
      </div>
    </details>
  </article>
</template>

<style scoped>
.audit-event-card {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
  padding: 1rem 1.125rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}

.audit-event-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.audit-event-card__action {
  margin: 0;
  font-size: 0.9375rem;
  font-weight: 600;
  word-break: break-word;
}

.audit-event-card__time {
  display: block;
  margin-top: 0.25rem;
  font-size: 0.8125rem;
  color: var(--color-text-muted);
}

.audit-event-card__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.75rem 1rem;
  margin: 0;
}

.audit-event-card__grid dt {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.audit-event-card__grid dd {
  margin: 0.125rem 0 0;
  font-size: 0.875rem;
  word-break: break-word;
}

.audit-event-card__wide {
  grid-column: 1 / -1;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.8125rem;
}

.audit-event-card__raw summary {
  cursor: pointer;
  color: var(--color-primary);
  font-size: 0.875rem;
}

.audit-event-card__raw-blocks {
  display: grid;
  gap: 0.75rem;
  margin-top: 0.5rem;
}

.audit-event-card__raw-label {
  margin: 0 0 0.25rem;
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.audit-event-card__raw pre {
  margin: 0;
  max-height: 200px;
  overflow: auto;
  padding: 0.75rem;
  background: var(--color-surface-muted, #f8fafc);
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
