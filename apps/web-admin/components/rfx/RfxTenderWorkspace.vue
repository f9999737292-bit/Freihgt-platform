<script setup lang="ts">
import type { RfxEvent } from '~/types/rfx'
import {
  createDefaultTenderWorkspaceState,
  provideTenderWorkspace,
} from '~/composables/useTenderWorkspaceState'

defineProps<{
  event: RfxEvent
  companyName?: (id: string) => string
}>()

const { t } = useI18n()

const tabs = [
  'overview',
  'participants',
  'bids',
  'evaluation',
  'allocation',
  'quota',
  'award',
] as const

type TenderTab = (typeof tabs)[number]

const activeTab = ref<TenderTab>('overview')
const workspace = ref(createDefaultTenderWorkspaceState())

provideTenderWorkspace(workspace)

function tabLabel(tab: TenderTab) {
  return t(`tender.tabs.${tab}`)
}
</script>

<template>
  <section class="tender-workspace">
    <nav class="tender-workspace__tabs" role="tablist" :aria-label="t('tender.workspaceTitle')">
      <button
        v-for="tab in tabs"
        :key="tab"
        type="button"
        role="tab"
        class="tender-workspace__tab"
        :class="{ 'tender-workspace__tab--active': activeTab === tab }"
        :aria-selected="activeTab === tab"
        @click="activeTab = tab"
      >
        {{ tabLabel(tab) }}
      </button>
    </nav>

    <div class="tender-workspace__panel" role="tabpanel">
      <RfxTenderOverviewPanel
        v-if="activeTab === 'overview'"
        :event="event"
        :workspace="workspace"
      />
      <RfxRfxParticipantsTable
        v-else-if="activeTab === 'participants'"
        :rfx-event-id="event.id"
        :company-name="companyName"
      />
      <RfxTenderBidsPanel
        v-else-if="activeTab === 'bids'"
        :rfx-event-id="event.id"
        :company-name="companyName"
      />
      <RfxEvaluationPanel
        v-else-if="activeTab === 'evaluation'"
        :rfx-event-id="event.id"
        :company-name="companyName"
      />
      <RfxTenderAllocationPanel
        v-else-if="activeTab === 'allocation'"
        :rfx-event-id="event.id"
        :company-name="companyName"
      />
      <RfxTenderQuotaPanel
        v-else-if="activeTab === 'quota'"
        :company-name="companyName"
      />
      <RfxTenderAwardPanel
        v-else-if="activeTab === 'award'"
        :rfx-event-id="event.id"
        :company-name="companyName"
      />
    </div>
  </section>
</template>

<style scoped>
.tender-workspace {
  margin-top: 1.5rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.tender-workspace__tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  padding: 0.75rem;
  border-bottom: 1px solid var(--color-border);
  background: var(--color-surface-muted, #f8fafc);
}

.tender-workspace__tab {
  border: 1px solid transparent;
  background: transparent;
  border-radius: var(--radius-sm);
  padding: 0.45rem 0.75rem;
  font-size: 0.875rem;
  cursor: pointer;
  color: var(--color-text-muted);
}

.tender-workspace__tab--active {
  background: var(--color-surface, #fff);
  border-color: var(--color-border);
  color: var(--color-text);
}

.tender-workspace__panel {
  padding: 1rem;
}
</style>
