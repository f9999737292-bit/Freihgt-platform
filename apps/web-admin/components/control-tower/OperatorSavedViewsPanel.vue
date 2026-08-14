<script setup lang="ts">
import type { ControlTowerSavedView } from '~/types/controlTower'

const props = defineProps<{
  views: ControlTowerSavedView[]
  activePreset: string
}>()

const emit = defineEmits<{
  apply: [view: ControlTowerSavedView]
  rename: [view: ControlTowerSavedView]
  update: [view: ControlTowerSavedView]
  duplicate: [view: ControlTowerSavedView]
  delete: [view: ControlTowerSavedView]
  setDefault: [view: ControlTowerSavedView]
  create: []
}>()

const { t } = useI18n()
const renamingId = ref<string | null>(null)
const renameValue = ref('')

function scopeLabel(scope: string): string {
  return scope === 'shared'
    ? t('controlTower.workspace.shared')
    : t('controlTower.workspace.private')
}

function startRename(view: ControlTowerSavedView) {
  renamingId.value = view.id
  renameValue.value = view.name
}

function submitRename(view: ControlTowerSavedView) {
  if (renameValue.value.trim()) {
    emit('rename', { ...view, name: renameValue.value.trim() })
  }
  renamingId.value = null
}
</script>

<template>
  <section class="saved-views" aria-labelledby="saved-views-title">
    <div class="saved-views__header">
      <h3 id="saved-views-title" class="saved-views__title">
        {{ $t('controlTower.workspace.savedViews') }}
      </h3>
      <UiButton size="sm" variant="secondary" @click="emit('create')">
        {{ $t('controlTower.workspace.saveView') }}
      </UiButton>
    </div>
    <p v-if="views.length === 0" class="saved-views__empty">
      {{ $t('controlTower.workspace.noSavedViews') }}
    </p>
    <ul v-else class="saved-views__list">
      <li v-for="view in views" :key="view.id" class="saved-views__item">
        <template v-if="renamingId === view.id">
          <input v-model="renameValue" type="text" maxlength="128" class="saved-views__input" />
          <UiButton size="sm" @click="submitRename(view)">{{ $t('common.save') }}</UiButton>
        </template>
        <template v-else>
          <button type="button" class="saved-views__apply" @click="emit('apply', view)">
            {{ view.name }}
          </button>
          <span class="saved-views__badges">
            <span class="saved-views__badge">{{ scopeLabel(view.scope) }}</span>
            <span v-if="view.isDefault" class="saved-views__badge saved-views__badge--default">
              {{ $t('controlTower.workspace.defaultView') }}
            </span>
          </span>
        </template>
        <div class="saved-views__actions">
          <UiButton size="sm" variant="ghost" @click="startRename(view)">
            {{ $t('controlTower.workspace.rename') }}
          </UiButton>
          <UiButton size="sm" variant="ghost" @click="emit('update', view)">
            {{ $t('controlTower.workspace.updateView') }}
          </UiButton>
          <UiButton size="sm" variant="ghost" @click="emit('duplicate', view)">
            {{ $t('controlTower.workspace.duplicate') }}
          </UiButton>
          <UiButton
            v-if="!view.isDefault"
            size="sm"
            variant="ghost"
            @click="emit('setDefault', view)"
          >
            {{ $t('controlTower.workspace.setAsDefault') }}
          </UiButton>
          <UiButton size="sm" variant="ghost" @click="emit('delete', view)">
            {{ $t('controlTower.workspace.delete') }}
          </UiButton>
        </div>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.saved-views__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.75rem;
}
.saved-views__title {
  margin: 0;
  font-size: 1rem;
}
.saved-views__empty {
  font-size: 0.875rem;
  color: var(--color-text-muted, #666);
}
.saved-views__list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.saved-views__item {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0;
  border-bottom: 1px solid var(--color-border, #eee);
}
.saved-views__apply {
  background: none;
  border: none;
  font-weight: 600;
  cursor: pointer;
  color: var(--color-primary, #3366ff);
}
.saved-views__badges {
  display: inline-flex;
  gap: 0.35rem;
}
.saved-views__badge {
  font-size: 0.75rem;
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
  border: 1px solid var(--color-border, #ddd);
}
.saved-views__badge--default {
  border-color: var(--color-primary, #3366ff);
}
.saved-views__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  margin-left: auto;
}
.saved-views__input {
  padding: 0.35rem 0.5rem;
  border: 1px solid var(--color-border, #ddd);
  border-radius: var(--radius-sm, 4px);
}
</style>
