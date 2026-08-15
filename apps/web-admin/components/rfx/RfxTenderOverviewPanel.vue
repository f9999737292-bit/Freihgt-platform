<script setup lang="ts">
import type { RfxEvent } from '~/types/rfx'
import type { TenderWorkspaceState } from '~/composables/useTenderWorkspaceState'

defineProps<{
  event: RfxEvent
  workspace: TenderWorkspaceState
}>()
</script>

<template>
  <div class="tender-overview">
    <h3>{{ $t('tender.tabs.overview') }}</h3>
    <dl class="tender-overview__grid">
      <div>
        <dt>{{ $t('rfx.rfxNumber') }}</dt>
        <dd>{{ event.rfx_number }}</dd>
      </div>
      <div>
        <dt>{{ $t('common.status') }}</dt>
        <dd>{{ event.status }}</dd>
      </div>
      <div>
        <dt>{{ $t('tender.evaluationId') }}</dt>
        <dd>{{ workspace.evaluationId || '—' }}</dd>
      </div>
      <div>
        <dt>{{ $t('tender.scenarioId') }}</dt>
        <dd>{{ workspace.scenarioId || '—' }}</dd>
      </div>
      <div>
        <dt>{{ $t('tender.proposalId') }}</dt>
        <dd>{{ workspace.proposalId || '—' }}</dd>
      </div>
      <div>
        <dt>{{ $t('tender.proposalStatus') }}</dt>
        <dd>{{ workspace.proposalStatus || '—' }}</dd>
      </div>
      <div>
        <dt>{{ $t('tender.awardId') }}</dt>
        <dd>{{ workspace.awardId || '—' }}</dd>
      </div>
      <div>
        <dt>{{ $t('tender.conversionStatus') }}</dt>
        <dd>{{ workspace.conversion?.status || '—' }}</dd>
      </div>
    </dl>
    <p class="tender-overview__hint">{{ $t('tender.overviewHint') }}</p>
  </div>
</template>

<style scoped>
.tender-overview__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.75rem 1rem;
  margin-top: 1rem;
}

.tender-overview__grid dt {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.tender-overview__grid dd {
  margin: 0.15rem 0 0;
  font-family: ui-monospace, monospace;
  font-size: 0.8125rem;
}

.tender-overview__hint {
  margin-top: 1rem;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}
</style>
