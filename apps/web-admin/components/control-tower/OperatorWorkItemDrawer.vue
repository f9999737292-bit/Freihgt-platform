<script setup lang="ts">
import type { ControlTowerWorkItem } from '~/types/controlTower'
import { timelineCategory } from '~/composables/useOperatorWorkspace'

const props = defineProps<{
  open: boolean
  item: ControlTowerWorkItem | null
  actionLoading?: boolean
  ownershipLabel: (item: ControlTowerWorkItem) => 'unassigned' | 'mine' | 'other'
}>()

const emit = defineEmits<{
  close: []
  claim: [item: ControlTowerWorkItem]
  openLinkedException: [eventId: string]
  openCase: [caseId: string]
  createCase: [item: ControlTowerWorkItem]
}>()

const { t } = useI18n()

function itemTypeLabel(item: ControlTowerWorkItem): string {
  return item.itemType === 'risk'
    ? t('controlTower.workspace.predictiveRisk')
    : t('controlTower.workspace.actualException')
}

function ownerBadge(item: ControlTowerWorkItem): string {
  const kind = props.ownershipLabel(item)
  if (kind === 'mine') return t('controlTower.workspace.assignedToMe')
  if (kind === 'other') return t('controlTower.workspace.assignedToOther')
  return t('controlTower.workspace.unassigned')
}

function timelineLabel(source: string, actionType: string): string {
  const cat = timelineCategory(source, actionType)
  return t(`controlTower.workspace.timelineCategory.${cat}`)
}
</script>

<template>
  <aside
    v-if="open && item"
    class="work-item-drawer"
    role="dialog"
    :aria-label="$t('controlTower.workspace.detailsTitle')"
  >
    <header class="work-item-drawer__header">
      <h3>{{ item.title }}</h3>
      <button type="button" class="work-item-drawer__close" :aria-label="$t('common.close')" @click="emit('close')">
        ×
      </button>
    </header>

    <p class="work-item-drawer__badge" :data-ownership="ownershipLabel(item)">
      {{ ownerBadge(item) }}
    </p>

    <section v-if="item.activeCase" class="work-item-drawer__case">
      <p>{{ $t('controlTower.cases.activeCaseBadge', { reference: item.activeCase.reference }) }}</p>
      <UiButton size="sm" variant="secondary" @click="emit('openCase', item.activeCase!.caseId)">
        {{ $t('controlTower.cases.openCase') }}
      </UiButton>
    </section>
    <section v-else class="work-item-drawer__case">
      <UiButton size="sm" variant="secondary" @click="emit('createCase', item)">
        {{ $t('controlTower.cases.createCaseFromWorkItem') }}
      </UiButton>
    </section>

    <dl class="work-item-drawer__meta">
      <dt>{{ $t('controlTower.workspace.type') }}</dt>
      <dd>{{ itemTypeLabel(item) }}</dd>
      <dt>{{ $t('controlTower.workspace.shipment') }}</dt>
      <dd>{{ item.shipmentNumber || item.shipmentId }}</dd>
      <dt>{{ $t('controlTower.workspace.urgencyColumn') }}</dt>
      <dd>{{ $t(`controlTower.workspace.urgencyLevels.${item.urgency}`) }}</dd>
      <template v-if="item.itemType === 'exception'">
        <dt>{{ $t('controlTower.workspace.workflowStatus') }}</dt>
        <dd>{{ item.workflowStatus }}</dd>
        <dt v-if="item.priority">{{ $t('controlTower.exceptions.priority') }}</dt>
        <dd v-if="item.priority">{{ item.priority }}</dd>
        <dt v-if="item.exceptionCategory">{{ $t('controlTower.exceptions.category') }}</dt>
        <dd v-if="item.exceptionCategory">{{ item.exceptionCategory }}</dd>
        <dt v-if="item.businessImpact">{{ $t('controlTower.exceptions.businessImpactLabel') }}</dt>
        <dd v-if="item.businessImpact">{{ item.businessImpact }}</dd>
        <dt v-if="item.slaStatus">{{ $t('controlTower.exceptions.sla') }}</dt>
        <dd v-if="item.slaStatus">{{ item.slaStatus }} / {{ item.slaPhase }}</dd>
        <dt v-if="item.escalationLevel">{{ $t('controlTower.exceptions.escalationLabel') }}</dt>
        <dd v-if="item.escalationLevel">{{ item.escalationLevel }}</dd>
      </template>
      <template v-if="item.itemType === 'risk'">
        <dt>{{ $t('controlTower.risk.levelLabel') }}</dt>
        <dd>{{ item.riskLevel }}</dd>
        <dt>{{ $t('controlTower.risk.scoreLabel') }}</dt>
        <dd>{{ item.riskScore }}</dd>
        <dt>{{ $t('controlTower.risk.statusLabel') }}</dt>
        <dd>{{ item.riskStatus }}</dd>
        <dt v-if="item.predictedExceptionType">{{ $t('controlTower.risk.predictedTypeLabel') }}</dt>
        <dd v-if="item.predictedExceptionType">{{ item.predictedExceptionType }}</dd>
      </template>
    </dl>

    <section v-if="item.itemType === 'risk' && item.linkedEventId" class="work-item-drawer__materialized">
      <p>{{ $t('controlTower.workspace.materializedAs') }}</p>
      <UiButton size="sm" variant="secondary" @click="emit('openLinkedException', item.linkedEventId!)">
        {{ $t('controlTower.workspace.openActualException') }}
      </UiButton>
    </section>

    <section v-if="item.availableActions.includes('claim')" class="work-item-drawer__actions">
      <UiButton size="sm" :disabled="actionLoading" @click="emit('claim', item)">
        {{ $t('controlTower.workspace.claim') }}
      </UiButton>
    </section>

    <section v-if="item.timeline?.length" class="work-item-drawer__timeline">
      <h4>{{ $t('controlTower.workspace.timeline') }}</h4>
      <ul>
        <li v-for="(entry, idx) in item.timeline" :key="idx">
          <span class="work-item-drawer__timeline-cat">
            {{ timelineLabel(entry.source, entry.actionType) }}
          </span>
          {{ entry.actionType }}
          <span v-if="entry.actorDisplayName"> — {{ entry.actorDisplayName }}</span>
          <time>{{ entry.occurredAt }}</time>
        </li>
      </ul>
    </section>
  </aside>
</template>

<style scoped>
.work-item-drawer {
  position: fixed;
  top: 0;
  right: 0;
  width: min(440px, 100vw);
  height: 100vh;
  background: var(--color-surface, #fff);
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.1);
  padding: 1rem;
  z-index: 50;
  overflow: auto;
}
.work-item-drawer__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 0.5rem;
}
.work-item-drawer__close {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
}
.work-item-drawer__badge {
  display: inline-block;
  margin: 0.5rem 0 1rem;
  padding: 0.2rem 0.55rem;
  border-radius: 999px;
  font-size: 0.8125rem;
  border: 1px solid var(--color-border, #ccc);
}
.work-item-drawer__meta {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.35rem 0.75rem;
  font-size: 0.875rem;
}
.work-item-drawer__timeline ul {
  list-style: none;
  padding: 0;
  font-size: 0.8125rem;
}
.work-item-drawer__timeline-cat {
  display: inline-block;
  min-width: 5rem;
  font-weight: 600;
  font-size: 0.75rem;
}
.work-item-drawer__materialized {
  margin: 1rem 0;
  padding: 0.75rem;
  background: var(--color-surface-muted, #f8f9fb);
  border-radius: var(--radius-sm, 4px);
}
.work-item-drawer__case {
  margin-bottom: 1rem;
  padding: 0.75rem;
  background: var(--color-surface-muted, #f8f9fb);
  border-radius: var(--radius-sm, 4px);
  font-size: 0.875rem;
}
</style>
