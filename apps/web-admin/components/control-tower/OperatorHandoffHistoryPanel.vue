<script setup lang="ts">
import type { ControlTowerHandoff } from '~/types/controlTower'
import type { User } from '~/types/user'

defineProps<{
  handoffs: ControlTowerHandoff[]
  users: User[]
  loading?: boolean
}>()

const emit = defineEmits<{
  open: [handoffId: string]
}>()

const { t } = useI18n()

function userName(users: User[], userId: string): string {
  const user = users.find((u) => u.id === userId)
  return user?.full_name || user?.email || userId
}

function transferredCount(handoff: ControlTowerHandoff): number {
  return handoff.items.filter((i) => i.outcome === 'transferred').length
}

function failedCount(handoff: ControlTowerHandoff): number {
  return handoff.items.filter((i) => i.outcome === 'failed').length
}
</script>

<template>
  <section class="handoff-history" aria-labelledby="handoff-history-title">
    <h3 id="handoff-history-title" class="handoff-history__title">
      {{ $t('controlTower.workspace.recentHandoffs') }}
    </h3>
    <p v-if="loading" class="handoff-history__empty">{{ $t('common.loading') }}</p>
    <p v-else-if="handoffs.length === 0" class="handoff-history__empty">
      {{ $t('controlTower.workspace.emptyHandoffs') }}
    </p>
    <ul v-else class="handoff-history__list">
      <li v-for="handoff in handoffs" :key="handoff.id" class="handoff-history__item">
        <button type="button" class="handoff-history__link" @click="emit('open', handoff.id)">
          <span class="handoff-history__ref">{{ handoff.id.slice(0, 8) }}</span>
          <span>{{ userName(users, handoff.fromUserId) }} → {{ handoff.toUserId ? userName(users, handoff.toUserId) : '—' }}</span>
          <span class="handoff-history__meta">
            {{ $t('controlTower.workspace.transferred', { count: transferredCount(handoff) }) }}
            <template v-if="failedCount(handoff) > 0">
              · {{ $t('controlTower.workspace.failed', { count: failedCount(handoff) }) }}
            </template>
          </span>
          <span v-if="handoff.note" class="handoff-history__note">{{ handoff.note }}</span>
        </button>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.handoff-history__title {
  margin: 0 0 0.75rem;
  font-size: 1rem;
}
.handoff-history__empty {
  color: var(--color-text-muted, #666);
  font-size: 0.875rem;
}
.handoff-history__list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.handoff-history__item + .handoff-history__item {
  margin-top: 0.5rem;
}
.handoff-history__link {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.15rem;
  width: 100%;
  text-align: left;
  background: var(--color-surface-muted, #f8f9fb);
  border: 1px solid var(--color-border, #eee);
  border-radius: var(--radius-sm, 4px);
  padding: 0.5rem 0.75rem;
  cursor: pointer;
  font-size: 0.875rem;
}
.handoff-history__ref {
  font-family: monospace;
  font-size: 0.75rem;
}
.handoff-history__meta {
  color: var(--color-text-muted, #666);
}
.handoff-history__note {
  font-style: italic;
  color: var(--color-text-muted, #666);
}
</style>
