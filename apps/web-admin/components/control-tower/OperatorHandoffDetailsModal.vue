<script setup lang="ts">
import type { ControlTowerHandoff } from '~/types/controlTower'
import type { User } from '~/types/user'

defineProps<{
  open: boolean
  handoff: ControlTowerHandoff | null
  users: User[]
  formatReason: (error?: string) => string
}>()

const emit = defineEmits<{
  close: []
  openWorkItem: [itemType: ControlTowerHandoff['items'][number]['itemType'], sourceId: string]
}>()

const { t } = useI18n()

function userName(users: User[], userId: string): string {
  return users.find((u) => u.id === userId)?.full_name || userId
}
</script>

<template>
  <UiModal
    :open="open && !!handoff"
    :title="$t('controlTower.workspace.handoffDetails')"
    @close="emit('close')"
  >
    <template v-if="handoff">
      <dl class="handoff-details__meta">
        <dt>{{ $t('controlTower.workspace.handoffReference') }}</dt>
        <dd>{{ handoff.id }}</dd>
        <dt>{{ $t('controlTower.workspace.fromOperator') }}</dt>
        <dd>{{ userName(users, handoff.fromUserId) }}</dd>
        <dt>{{ $t('controlTower.workspace.recipient') }}</dt>
        <dd>{{ handoff.toUserId ? userName(users, handoff.toUserId) : '—' }}</dd>
        <dt>{{ $t('controlTower.workspace.createdAt') }}</dt>
        <dd>{{ handoff.createdAt }}</dd>
        <template v-if="handoff.note">
          <dt>{{ $t('controlTower.workspace.handoffNote') }}</dt>
          <dd>{{ handoff.note }}</dd>
        </template>
      </dl>

      <h4>{{ $t('controlTower.workspace.transferredItems') }}</h4>
      <ul class="handoff-details__list">
        <li v-for="item in handoff.items" :key="item.id">
          <button
            v-if="item.outcome === 'transferred'"
            type="button"
            class="handoff-details__link"
            @click="emit('openWorkItem', item.itemType, item.sourceId)"
          >
            {{ item.itemType }} · {{ item.sourceId }}
          </button>
          <span v-else>
            {{ item.itemType }} · {{ item.sourceId }} —
            {{ formatReason(item.errorCode) }}
          </span>
        </li>
      </ul>
    </template>

    <template #footer>
      <UiButton variant="secondary" @click="emit('close')">{{ $t('common.close') }}</UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.handoff-details__meta {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.35rem 1rem;
  font-size: 0.875rem;
  margin-bottom: 1rem;
}
.handoff-details__list {
  list-style: none;
  padding: 0;
  margin: 0;
  font-size: 0.875rem;
}
.handoff-details__link {
  background: none;
  border: none;
  color: var(--color-primary, #3366ff);
  cursor: pointer;
  padding: 0;
}
</style>
