<script setup lang="ts">
import type { ControlTowerWorkItem } from '~/types/controlTower'

const props = defineProps<{
  open: boolean
  item: ControlTowerWorkItem | null
  loading?: boolean
}>()

const emit = defineEmits<{ close: []; created: [caseId: string]; added: [caseId: string] }>()

const { createCaseFromWorkItem, findDuplicateCandidates, addWorkItemToCase, actionLoading } = useOperationalCases()

const title = ref('')
const summary = ref('')
const duplicates = ref<import('~/types/controlTower').ControlTowerCaseDuplicateCandidate[]>([])
const selectedCaseId = ref('')

watch(
  () => props.open,
  async (isOpen) => {
    if (isOpen && props.item) {
      title.value = props.item.title
      summary.value = props.item.summary || ''
      selectedCaseId.value = ''
      duplicates.value = await findDuplicateCandidates(props.item)
    }
  },
)

async function onCreate() {
  if (!props.item || !title.value.trim()) return
  const created = await createCaseFromWorkItem({ ...props.item, title: title.value.trim(), summary: summary.value.trim() })
  if (created) {
    emit('created', created.id)
    emit('close')
  }
}

async function onAddExisting() {
  if (!props.item || !selectedCaseId.value) return
  const ok = await addWorkItemToCase(selectedCaseId.value, props.item)
  if (ok) {
    emit('added', selectedCaseId.value)
    emit('close')
  }
}
</script>

<template>
  <UiModal :open="open && !!item" :title="$t('controlTower.cases.createFromWorkItem')" @close="emit('close')">
    <p v-if="duplicates.length" class="case-create-modal__warn">
      {{ $t('controlTower.cases.duplicateWarning') }}
    </p>
    <ul v-if="duplicates.length" class="case-create-modal__dupes">
      <li v-for="dup in duplicates" :key="dup.id">
        <strong>{{ dup.reference }}</strong> — {{ dup.title }}
        <span class="case-create-modal__dupe-meta">
          · {{ $t(`controlTower.cases.statuses.${dup.status}`) }}
          · {{ $t(`controlTower.cases.severities.${dup.effectiveSeverity}`) }}
          · {{ dup.ownerDisplayName || $t('controlTower.cases.unassigned') }}
        </span>
      </li>
    </ul>

    <template v-if="item?.activeCase">
      <p>{{ $t('controlTower.cases.alreadyLinked') }}: {{ item.activeCase.reference }}</p>
    </template>
    <template v-else>
      <label class="case-create-modal__field">
        {{ $t('controlTower.cases.titleColumn') }}
        <input v-model="title" type="text" maxlength="256" required />
      </label>
      <label class="case-create-modal__field">
        {{ $t('controlTower.cases.summary') }}
        <textarea v-model="summary" rows="3" />
      </label>

      <div v-if="duplicates.length" class="case-create-modal__add-existing">
        <label>
          {{ $t('controlTower.cases.addToCase') }}
          <select v-model="selectedCaseId">
            <option value="">{{ $t('controlTower.cases.selectCase') }}</option>
            <option v-for="dup in duplicates" :key="dup.id" :value="dup.id">
              {{ dup.reference }} — {{ dup.title }}
            </option>
          </select>
        </label>
        <UiButton size="sm" variant="secondary" :disabled="!selectedCaseId || actionLoading" @click="onAddExisting">
          {{ $t('controlTower.cases.addToCase') }}
        </UiButton>
      </div>
    </template>

    <template #footer>
      <UiButton variant="secondary" @click="emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton v-if="!item?.activeCase" :loading="actionLoading || loading" :disabled="!title.trim()" @click="onCreate">
        {{ $t('controlTower.cases.createCase') }}
      </UiButton>
    </template>
  </UiModal>
</template>

<style scoped>
.case-create-modal__field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  margin-bottom: 1rem;
  font-size: 0.875rem;
}
.case-create-modal__warn {
  color: var(--color-warning-text, #9a6700);
  font-size: 0.875rem;
}
.case-create-modal__dupes {
  margin: 0 0 1rem;
  padding-left: 1.25rem;
  font-size: 0.875rem;
}
.case-create-modal__dupe-meta {
  display: block;
  font-size: 0.8125rem;
  color: var(--color-text-muted, #666);
}
.case-create-modal__add-existing {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-top: 1rem;
  font-size: 0.875rem;
}
</style>
