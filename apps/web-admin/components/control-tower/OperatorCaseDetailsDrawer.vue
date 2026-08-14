<script setup lang="ts">
import type { ControlTowerOperationalCase, ControlTowerCaseResolutionCode } from '~/types/controlTower'
import { CONTROL_TOWER_CASE_RESOLUTION_CODES } from '~/types/controlTower'

const props = defineProps<{
  open: boolean
  caseItem: ControlTowerOperationalCase | null
  actionLoading?: boolean
  tenantUsers?: import('~/types/user').User[]
}>()

const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()
const {
  claimCase,
  resolveCase,
  closeCase,
  reopenCase,
  addNote,
  createActionItem,
  completeActionItem,
  recordDecision,
  isActionOverdue,
} = useOperationalCases()

const noteBody = ref('')
const actionTitle = ref('')
const decisionText = ref('')
const decisionRationale = ref('')
const resolveCode = ref<ControlTowerCaseResolutionCode>('operational_issue_resolved')
const resolveSummary = ref('')
const showResolve = ref(false)

async function onAddNote() {
  if (!props.caseItem || !noteBody.value.trim()) return
  await addNote(props.caseItem.id, noteBody.value.trim())
  noteBody.value = ''
}

async function onAddAction() {
  if (!props.caseItem || !actionTitle.value.trim()) return
  await createActionItem(props.caseItem.id, actionTitle.value.trim())
  actionTitle.value = ''
}

async function onRecordDecision() {
  if (!props.caseItem || !decisionText.value.trim()) return
  await recordDecision(props.caseItem.id, decisionText.value.trim(), decisionRationale.value.trim())
  decisionText.value = ''
  decisionRationale.value = ''
}

async function onResolve() {
  if (!props.caseItem) return
  await resolveCase(props.caseItem.id, resolveCode.value, resolveSummary.value)
  showResolve.value = false
}
</script>

<template>
  <aside
    v-if="open && caseItem"
    class="case-drawer"
    role="dialog"
    :aria-label="$t('controlTower.cases.detailsTitle')"
  >
    <header class="case-drawer__header">
      <div>
        <p class="case-drawer__ref">{{ caseItem.reference }}</p>
        <h3>{{ caseItem.title }}</h3>
      </div>
      <button type="button" class="case-drawer__close" :aria-label="$t('common.close')" @click="emit('close')">×</button>
    </header>

    <section class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.overview') }}</h4>
      <dl class="case-drawer__meta">
        <dt>{{ $t('controlTower.cases.status') }}</dt>
        <dd>{{ $t(`controlTower.cases.statuses.${caseItem.status}`) }}</dd>
        <dt>{{ $t('controlTower.cases.severity') }}</dt>
        <dd>{{ $t(`controlTower.cases.severities.${caseItem.effectiveSeverity}`) }}</dd>
        <dt>{{ $t('controlTower.cases.caseOwner') }}</dt>
        <dd>{{ caseItem.ownerDisplayName || $t('controlTower.cases.unassigned') }}</dd>
        <dt>{{ $t('controlTower.cases.summary') }}</dt>
        <dd>{{ caseItem.summary || '—' }}</dd>
      </dl>
      <div class="case-drawer__actions">
        <UiButton v-if="!caseItem.ownerUserId" size="sm" :disabled="actionLoading" @click="claimCase(caseItem.id)">
          {{ $t('controlTower.cases.claim') }}
        </UiButton>
        <UiButton
          v-if="!['resolved','closed'].includes(caseItem.status)"
          size="sm"
          variant="secondary"
          @click="showResolve = true"
        >
          {{ $t('controlTower.cases.resolve') }}
        </UiButton>
        <UiButton v-if="caseItem.status === 'resolved'" size="sm" variant="secondary" @click="closeCase(caseItem.id)">
          {{ $t('controlTower.cases.close') }}
        </UiButton>
        <UiButton v-if="['resolved','closed'].includes(caseItem.status)" size="sm" variant="ghost" @click="reopenCase(caseItem.id)">
          {{ $t('controlTower.cases.reopen') }}
        </UiButton>
      </div>
    </section>

    <section v-if="showResolve" class="case-drawer__section case-drawer__resolve">
      <h4>{{ $t('controlTower.cases.resolve') }}</h4>
      <p v-if="caseItem.health?.activeWorkItemCount" class="case-drawer__warn">
        {{ $t('controlTower.cases.activeWorkWarning', { count: caseItem.health.activeWorkItemCount }) }}
      </p>
      <select v-model="resolveCode" class="case-drawer__select">
        <option v-for="code in CONTROL_TOWER_CASE_RESOLUTION_CODES" :key="code" :value="code">
          {{ $t(`controlTower.cases.resolutionCodes.${code}`) }}
        </option>
      </select>
      <textarea v-model="resolveSummary" rows="2" :placeholder="$t('controlTower.cases.rationale')" />
      <UiButton size="sm" :loading="actionLoading" @click="onResolve">{{ $t('controlTower.cases.confirmResolve') }}</UiButton>
    </section>

    <section v-if="caseItem.links?.length" class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.linkedWork') }}</h4>
      <ul>
        <li v-for="link in caseItem.links" :key="link.id">
          {{ link.entityType }} · {{ link.entityId }}
        </li>
      </ul>
    </section>

    <section class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.actionItems') }}</h4>
      <ul v-if="caseItem.actionItems?.length">
        <li v-for="action in caseItem.actionItems" :key="action.id" class="case-drawer__action-item">
          <span>{{ action.title }}</span>
          <span :data-overdue="isActionOverdue(action)">
            {{ $t(`controlTower.cases.actionStatuses.${action.status}`) }}
            <template v-if="action.dueAt"> · {{ action.dueAt }}</template>
          </span>
          <UiButton
            v-if="action.status !== 'done'"
            size="sm"
            variant="ghost"
            :disabled="actionLoading"
            @click="completeActionItem(caseItem.id, action.id)"
          >
            {{ $t('controlTower.cases.completeAction') }}
          </UiButton>
        </li>
      </ul>
      <div class="case-drawer__inline-form">
        <input v-model="actionTitle" type="text" :placeholder="$t('controlTower.cases.addAction')" />
        <UiButton size="sm" :disabled="!actionTitle.trim() || actionLoading" @click="onAddAction">
          {{ $t('common.create') }}
        </UiButton>
      </div>
    </section>

    <section class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.notes') }}</h4>
      <ul v-if="caseItem.notes?.length">
        <li v-for="note in caseItem.notes" :key="note.id" class="case-drawer__note">
          <p>{{ note.body }}</p>
          <small>{{ note.createdAt }}<span v-if="note.editedAt"> · {{ $t('controlTower.cases.edited') }}</span></small>
        </li>
      </ul>
      <label class="case-drawer__inline-form">
        <span>{{ $t('controlTower.cases.internalNote') }}</span>
        <textarea v-model="noteBody" rows="3" />
        <UiButton size="sm" :disabled="!noteBody.trim() || actionLoading" @click="onAddNote">
          {{ $t('controlTower.cases.addNote') }}
        </UiButton>
      </label>
    </section>

    <section class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.decisions') }}</h4>
      <ul v-if="caseItem.decisions?.length">
        <li v-for="d in caseItem.decisions" :key="d.id">
          <strong>{{ d.decision }}</strong>
          <p v-if="d.rationale">{{ d.rationale }}</p>
        </li>
      </ul>
      <div class="case-drawer__inline-form">
        <input v-model="decisionText" type="text" :placeholder="$t('controlTower.cases.recordDecision')" />
        <input v-model="decisionRationale" type="text" :placeholder="$t('controlTower.cases.rationale')" />
        <UiButton size="sm" :disabled="!decisionText.trim() || actionLoading" @click="onRecordDecision">
          {{ $t('controlTower.cases.recordDecision') }}
        </UiButton>
      </div>
    </section>
  </aside>
</template>

<style scoped>
.case-drawer {
  position: fixed;
  top: 0;
  right: 0;
  width: min(480px, 100vw);
  height: 100vh;
  background: var(--color-surface, #fff);
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.1);
  padding: 1rem;
  z-index: 55;
  overflow: auto;
}
.case-drawer__header {
  display: flex;
  justify-content: space-between;
  gap: 0.5rem;
}
.case-drawer__ref {
  margin: 0;
  font-family: monospace;
  font-size: 0.8125rem;
  color: var(--color-text-muted, #666);
}
.case-drawer__close {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
}
.case-drawer__section {
  margin-top: 1.25rem;
  padding-top: 1rem;
  border-top: 1px solid var(--color-border, #eee);
}
.case-drawer__meta {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.35rem 0.75rem;
  font-size: 0.875rem;
}
.case-drawer__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-top: 0.75rem;
}
.case-drawer__inline-form {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-top: 0.5rem;
}
.case-drawer__warn {
  color: var(--color-warning-text, #9a6700);
  font-size: 0.875rem;
}
.case-drawer__select,
.case-drawer__inline-form input,
.case-drawer__inline-form textarea {
  width: 100%;
  padding: 0.5rem;
  border: 1px solid var(--color-border, #ddd);
  border-radius: var(--radius-sm, 4px);
}
.case-drawer__note p {
  margin: 0 0 0.25rem;
  white-space: pre-wrap;
}
</style>
