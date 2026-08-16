<script setup lang="ts">
import type { ControlTowerCaseResolutionCode, ControlTowerCaseSeverity, ControlTowerOperationalCase } from '~/types/controlTower'
import { CONTROL_TOWER_CASE_RESOLUTION_CODES } from '~/types/controlTower'
import {
  caseSlaDisplayState,
  caseTimelineCategory,
  formatCaseDateTime,
  formatRelativeTime,
} from '~/composables/useCaseDisplay'

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
  updateActionItem,
  completeActionItem,
  recordDecision,
  setSeverityOverride,
  clearSeverityOverride,
  addParticipant,
  updateParticipantRole,
  removeParticipant,
  isActionOverdue,
  timelineEntries,
  timelineHasNext,
  timelineLoading,
  loadOlderTimeline,
} = useOperationalCases()

type DrawerSection = 'overview' | 'linkedWork' | 'shipments' | 'actions' | 'notes' | 'decisions' | 'participants' | 'timeline' | 'playbooks'
const activeSection = ref<DrawerSection>('overview')

const noteBody = ref('')
const actionTitle = ref('')
const actionDueAt = ref('')
const actionAssigneeId = ref('')
const decisionText = ref('')
const decisionRationale = ref('')
const resolveCode = ref<ControlTowerCaseResolutionCode>('operational_issue_resolved')
const resolveSummary = ref('')
const showResolve = ref(false)
const overrideSeverity = ref<ControlTowerCaseSeverity>('medium')
const newParticipantUserId = ref('')
const newParticipantRole = ref<'collaborator' | 'observer'>('collaborator')

const sections: { id: DrawerSection; labelKey: string }[] = [
  { id: 'overview', labelKey: 'controlTower.cases.overview' },
  { id: 'linkedWork', labelKey: 'controlTower.cases.linkedWork' },
  { id: 'shipments', labelKey: 'controlTower.cases.linkedShipments' },
  { id: 'actions', labelKey: 'controlTower.cases.actionItems' },
  { id: 'notes', labelKey: 'controlTower.cases.notes' },
  { id: 'decisions', labelKey: 'controlTower.cases.decisions' },
  { id: 'participants', labelKey: 'controlTower.cases.participants' },
  { id: 'timeline', labelKey: 'controlTower.cases.timeline' },
  { id: 'playbooks', labelKey: 'controlTower.automation.playbooksSection' },
]

const ownerUserId = computed(() => props.caseItem?.ownerUserId)
const collaborators = computed(() =>
  (props.caseItem?.participants ?? []).filter((p) => p.role === 'collaborator'),
)
const observers = computed(() =>
  (props.caseItem?.participants ?? []).filter((p) => p.role === 'observer'),
)
const exceptionLinks = computed(() =>
  (props.caseItem?.links ?? []).filter((l) => l.entityType === 'exception' || l.entityType === 'risk'),
)
const resolveWarnings = computed(() => ({
  activeWork: props.caseItem?.health?.activeWorkItemCount ?? 0,
  openActions: props.caseItem?.health?.openActionCount ?? 0,
  overdueActions: props.caseItem?.health?.overdueActionCount ?? 0,
}))

function dueLabel(iso?: string): string {
  const rel = formatRelativeTime(iso)
  if (!rel) return '—'
  if (rel.startsWith('in_')) {
    const n = rel.replace('in_', '')
    if (rel.includes('m')) return t('controlTower.cases.dueInMinutes', { count: parseInt(n, 10) || 0 })
    if (rel.includes('h')) return t('controlTower.cases.dueInHours', { count: parseInt(n, 10) || 0 })
    if (rel === 'tomorrow') return t('controlTower.cases.dueTomorrow')
    return t('controlTower.cases.dueInDays', { count: parseInt(n, 10) || 0 })
  }
  if (rel.startsWith('overdue_')) {
    const n = rel.replace('overdue_', '')
    if (rel.includes('m')) return t('controlTower.cases.overdueByMinutes', { count: parseInt(n, 10) || 0 })
    if (rel.includes('h')) return t('controlTower.cases.overdueByHours', { count: parseInt(n, 10) || 0 })
    return t('controlTower.cases.overdueByDays', { count: parseInt(n, 10) || 0 })
  }
  return formatCaseDateTime(iso)
}

function timelineLabel(entry: { source: string; actionType: string }): string {
  const key = `controlTower.cases.timelineEvents.${entry.actionType}`
  const translated = t(key)
  if (translated !== key) return translated
  return t(`controlTower.cases.timelineCategories.${caseTimelineCategory(entry.source, entry.actionType)}`)
}

function userName(userId?: string): string {
  if (!userId) return '—'
  return props.tenantUsers?.find((u) => u.id === userId)?.displayName ?? userId
}

async function onAddNote() {
  if (!props.caseItem || !noteBody.value.trim()) return
  await addNote(props.caseItem.id, noteBody.value.trim())
  noteBody.value = ''
}

async function onAddAction() {
  if (!props.caseItem || !actionTitle.value.trim()) return
  await createActionItem(
    props.caseItem.id,
    actionTitle.value.trim(),
    undefined,
    actionAssigneeId.value || undefined,
    actionDueAt.value || undefined,
  )
  actionTitle.value = ''
  actionDueAt.value = ''
  actionAssigneeId.value = ''
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

async function onSetOverride() {
  if (!props.caseItem) return
  await setSeverityOverride(props.caseItem.id, overrideSeverity.value, props.caseItem)
}

async function onClearOverride() {
  if (!props.caseItem) return
  await clearSeverityOverride(props.caseItem.id, props.caseItem)
}

async function onAddParticipant() {
  if (!props.caseItem || !newParticipantUserId.value) return
  const ok = await addParticipant(props.caseItem.id, newParticipantUserId.value, newParticipantRole.value)
  if (ok) {
    newParticipantUserId.value = ''
    newParticipantRole.value = 'collaborator'
  }
}

watch(
  () => props.caseItem?.id,
  () => {
    activeSection.value = 'overview'
    showResolve.value = false
  },
)
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

    <nav class="case-drawer__nav" :aria-label="$t('controlTower.cases.sectionsNav')">
      <button
        v-for="section in sections"
        :key="section.id"
        type="button"
        class="case-drawer__nav-btn"
        :class="{ 'case-drawer__nav-btn--active': activeSection === section.id }"
        @click="activeSection = section.id"
      >
        {{ $t(section.labelKey) }}
      </button>
    </nav>

    <section v-if="activeSection === 'overview'" class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.health.title') }}</h4>
      <dl class="case-drawer__meta">
        <dt>{{ $t('controlTower.cases.health.effectiveSeverity') }}</dt>
        <dd>{{ $t(`controlTower.cases.severities.${caseItem.effectiveSeverity}`) }}</dd>
        <dt>{{ $t('controlTower.cases.health.derivedSeverity') }}</dt>
        <dd>{{ $t(`controlTower.cases.severities.${caseItem.derivedSeverity}`) }}</dd>
        <dt v-if="caseItem.severityOverride">{{ $t('controlTower.cases.health.manualOverride') }}</dt>
        <dd v-if="caseItem.severityOverride">{{ $t('controlTower.cases.health.overrideActive') }}</dd>
        <dt>{{ $t('controlTower.cases.health.sla') }}</dt>
        <dd>{{ $t(`controlTower.cases.slaStates.${caseSlaDisplayState(caseItem.health)}`) }}</dd>
        <dt v-if="caseItem.health?.nearestSlaDueAt">{{ $t('controlTower.cases.health.nearestSla') }}</dt>
        <dd v-if="caseItem.health?.nearestSlaDueAt">{{ formatCaseDateTime(caseItem.health.nearestSlaDueAt) }}</dd>
        <dt v-if="caseItem.health?.highestExceptionPriority">{{ $t('controlTower.cases.health.highestPriority') }}</dt>
        <dd v-if="caseItem.health?.highestExceptionPriority">{{ caseItem.health.highestExceptionPriority.toUpperCase() }}</dd>
        <dt v-if="caseItem.health?.highestRiskLevel">{{ $t('controlTower.cases.health.highestRisk') }}</dt>
        <dd v-if="caseItem.health?.highestRiskLevel">{{ caseItem.health.highestRiskLevel }}</dd>
        <dt>{{ $t('controlTower.cases.health.openActions') }}</dt>
        <dd>{{ caseItem.health?.openActionCount ?? 0 }}</dd>
        <dt>{{ $t('controlTower.cases.health.overdueActions') }}</dt>
        <dd>{{ caseItem.health?.overdueActionCount ?? 0 }}</dd>
        <dt>{{ $t('controlTower.cases.health.activeWork') }}</dt>
        <dd>{{ caseItem.health?.activeWorkItemCount ?? 0 }}</dd>
      </dl>

      <div class="case-drawer__severity-controls">
        <label>
          <span>{{ $t('controlTower.cases.overrideSeverity') }}</span>
          <select v-model="overrideSeverity" class="case-drawer__select">
            <option value="critical">{{ $t('controlTower.cases.severities.critical') }}</option>
            <option value="high">{{ $t('controlTower.cases.severities.high') }}</option>
            <option value="medium">{{ $t('controlTower.cases.severities.medium') }}</option>
            <option value="low">{{ $t('controlTower.cases.severities.low') }}</option>
          </select>
        </label>
        <UiButton size="sm" :disabled="actionLoading" @click="onSetOverride">{{ $t('controlTower.cases.applyOverride') }}</UiButton>
        <UiButton v-if="caseItem.severityOverride" size="sm" variant="secondary" :disabled="actionLoading" @click="onClearOverride">
          {{ $t('controlTower.cases.clearOverride') }}
        </UiButton>
      </div>

      <h4>{{ $t('controlTower.cases.overview') }}</h4>
      <dl class="case-drawer__meta">
        <dt>{{ $t('controlTower.cases.status') }}</dt>
        <dd>{{ $t(`controlTower.cases.statuses.${caseItem.status}`) }}</dd>
        <dt>{{ $t('controlTower.cases.caseOwner') }}</dt>
        <dd>{{ caseItem.ownerDisplayName || $t('controlTower.cases.unassigned') }}</dd>
        <dt>{{ $t('controlTower.cases.summary') }}</dt>
        <dd>{{ caseItem.summary || '—' }}</dd>
      </dl>
      <div class="case-drawer__actions">
        <UiButton v-if="!caseItem.ownerUserId" size="sm" :disabled="actionLoading" @click="claimCase(caseItem.id)">
          {{ $t('controlTower.cases.claim') }}
        </UiButton>
        <UiButton v-if="!['resolved','closed'].includes(caseItem.status)" size="sm" variant="secondary" @click="showResolve = true">
          {{ $t('controlTower.cases.resolve') }}
        </UiButton>
        <UiButton v-if="caseItem.status === 'resolved'" size="sm" variant="secondary" @click="closeCase(caseItem.id)">
          {{ $t('controlTower.cases.close') }}
        </UiButton>
        <UiButton v-if="['resolved','closed'].includes(caseItem.status)" size="sm" variant="ghost" @click="reopenCase(caseItem.id)">
          {{ $t('controlTower.cases.reopen') }}
        </UiButton>
      </div>

      <div v-if="showResolve" class="case-drawer__resolve">
        <h4>{{ $t('controlTower.cases.resolve') }}</h4>
        <ul class="case-drawer__warn-list">
          <li>{{ $t('controlTower.cases.resolveCheck.activeWork', { count: resolveWarnings.activeWork }) }}</li>
          <li>{{ $t('controlTower.cases.resolveCheck.openActions', { count: resolveWarnings.openActions }) }}</li>
          <li>{{ $t('controlTower.cases.resolveCheck.overdueActions', { count: resolveWarnings.overdueActions }) }}</li>
        </ul>
        <select v-model="resolveCode" class="case-drawer__select">
          <option v-for="code in CONTROL_TOWER_CASE_RESOLUTION_CODES" :key="code" :value="code">
            {{ $t(`controlTower.cases.resolutionCodes.${code}`) }}
          </option>
        </select>
        <textarea v-model="resolveSummary" rows="2" :placeholder="$t('controlTower.cases.rationale')" />
        <UiButton size="sm" :loading="actionLoading" @click="onResolve">{{ $t('controlTower.cases.confirmResolve') }}</UiButton>
      </div>
    </section>

    <section v-else-if="activeSection === 'linkedWork'" class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.linkedWork') }}</h4>
      <ul v-if="exceptionLinks.length">
        <li v-for="link in exceptionLinks" :key="link.id">
          <span class="case-drawer__badge">{{ link.entityType }}</span>
          {{ link.entityId }}
        </li>
      </ul>
      <p v-else>{{ $t('controlTower.cases.noLinkedWork') }}</p>
    </section>

    <section v-else-if="activeSection === 'shipments'" class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.linkedShipments') }}</h4>
      <div v-if="caseItem.linkedShipments?.length" class="case-drawer__cards">
        <article v-for="sh in caseItem.linkedShipments" :key="sh.id" class="case-drawer__card">
          <h5>{{ sh.reference || sh.id }}</h5>
          <dl>
            <dt>{{ $t('controlTower.cases.shipment.status') }}</dt><dd>{{ sh.status || '—' }}</dd>
            <dt>{{ $t('controlTower.cases.shipment.plannedPickup') }}</dt><dd>{{ formatCaseDateTime(sh.plannedPickupAt) }}</dd>
            <dt>{{ $t('controlTower.cases.shipment.actualPickup') }}</dt><dd>{{ formatCaseDateTime(sh.actualPickupAt) }}</dd>
            <dt>{{ $t('controlTower.cases.shipment.plannedDelivery') }}</dt><dd>{{ formatCaseDateTime(sh.plannedDeliveryAt) }}</dd>
            <dt>{{ $t('controlTower.cases.shipment.actualDelivery') }}</dt><dd>{{ formatCaseDateTime(sh.actualDeliveryAt) }}</dd>
            <dt>{{ $t('controlTower.cases.shipment.driver') }}</dt><dd>{{ sh.driverId || '—' }}</dd>
            <dt>{{ $t('controlTower.cases.shipment.vehicle') }}</dt><dd>{{ sh.vehicleId || '—' }}</dd>
            <dt>{{ $t('controlTower.cases.shipment.carrier') }}</dt><dd>{{ sh.carrierCompanyId || '—' }}</dd>
          </dl>
          <p class="case-drawer__limitation">{{ $t('controlTower.cases.dataLimitations.noGpsEta') }}</p>
        </article>
      </div>
      <p v-else>{{ $t('controlTower.cases.noLinkedShipments') }}</p>

      <h4>{{ $t('controlTower.cases.transportOrder') }}</h4>
      <div v-if="caseItem.linkedTransportOrders?.length" class="case-drawer__cards">
        <article v-for="ord in caseItem.linkedTransportOrders" :key="ord.id" class="case-drawer__card">
          <h5>{{ ord.reference || ord.id }}</h5>
          <p>{{ ord.status || '—' }}</p>
        </article>
      </div>
      <p v-else>{{ $t('controlTower.cases.noLinkedOrders') }}</p>
    </section>

    <section v-else-if="activeSection === 'actions'" class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.actionItems') }}</h4>
      <ul v-if="caseItem.actionItems?.length">
        <li v-for="action in caseItem.actionItems" :key="action.id" class="case-drawer__action-item">
          <div>
            <strong>{{ action.title }}</strong>
            <span class="case-drawer__badge" :data-overdue="isActionOverdue(action)">
              {{ $t(`controlTower.cases.actionStatuses.${action.status}`) }}
            </span>
            <span v-if="action.dueAt"> · {{ dueLabel(action.dueAt) }}</span>
          </div>
          <div class="case-drawer__inline-actions">
            <UiButton v-if="action.status === 'open'" size="sm" variant="ghost" @click="updateActionItem(caseItem.id, action.id, { status: 'in_progress' })">
              {{ $t('controlTower.cases.startAction') }}
            </UiButton>
            <UiButton v-if="action.status !== 'done' && action.status !== 'cancelled'" size="sm" variant="ghost" @click="completeActionItem(caseItem.id, action.id)">
              {{ $t('controlTower.cases.completeAction') }}
            </UiButton>
            <UiButton v-if="action.status !== 'done' && action.status !== 'cancelled'" size="sm" variant="ghost" @click="updateActionItem(caseItem.id, action.id, { status: 'cancelled' })">
              {{ $t('controlTower.cases.cancelAction') }}
            </UiButton>
          </div>
        </li>
      </ul>
      <p v-else>{{ $t('controlTower.cases.noActions') }}</p>
      <div class="case-drawer__inline-form">
        <input v-model="actionTitle" type="text" :placeholder="$t('controlTower.cases.addAction')" />
        <input v-model="actionDueAt" type="datetime-local" :aria-label="$t('controlTower.cases.actionDueAt')" />
        <select v-model="actionAssigneeId" :aria-label="$t('controlTower.cases.actionAssignee')">
          <option value="">{{ $t('controlTower.cases.unassigned') }}</option>
          <option v-for="user in tenantUsers ?? []" :key="user.id" :value="user.id">{{ user.displayName }}</option>
        </select>
        <UiButton size="sm" :disabled="!actionTitle.trim() || actionLoading" @click="onAddAction">{{ $t('common.create') }}</UiButton>
      </div>
    </section>

    <section v-else-if="activeSection === 'notes'" class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.notes') }}</h4>
      <ul v-if="caseItem.notes?.length">
        <li v-for="note in caseItem.notes" :key="note.id" class="case-drawer__note">
          <p>{{ note.body }}</p>
          <small>{{ formatCaseDateTime(note.createdAt) }}<span v-if="note.editedAt"> · {{ $t('controlTower.cases.edited') }}</span></small>
        </li>
      </ul>
      <label class="case-drawer__inline-form">
        <span>{{ $t('controlTower.cases.internalNote') }}</span>
        <textarea v-model="noteBody" rows="3" />
        <UiButton size="sm" :disabled="!noteBody.trim() || actionLoading" @click="onAddNote">{{ $t('controlTower.cases.addNote') }}</UiButton>
      </label>
    </section>

    <section v-else-if="activeSection === 'decisions'" class="case-drawer__section">
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
        <UiButton size="sm" :disabled="!decisionText.trim() || actionLoading" @click="onRecordDecision">{{ $t('controlTower.cases.recordDecision') }}</UiButton>
      </div>
    </section>

    <section v-else-if="activeSection === 'participants'" class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.participants') }}</h4>
      <div class="case-drawer__participant-group">
        <h5>{{ $t('controlTower.cases.caseOwner') }}</h5>
        <p>{{ caseItem.ownerDisplayName || $t('controlTower.cases.unassigned') }}</p>
        <p class="case-drawer__hint">{{ $t('controlTower.cases.ownerInvariantHint') }}</p>
      </div>
      <div class="case-drawer__participant-group">
        <h5>{{ $t('controlTower.cases.collaborators') }}</h5>
        <ul v-if="collaborators.length">
          <li v-for="p in collaborators" :key="p.userId" class="case-drawer__participant-row">
            <span>{{ p.displayName || userName(p.userId) }}</span>
            <UiButton size="sm" variant="ghost" @click="updateParticipantRole(caseItem.id, p.userId, 'observer')">{{ $t('controlTower.cases.makeObserver') }}</UiButton>
            <UiButton size="sm" variant="ghost" @click="removeParticipant(caseItem.id, p.userId)">{{ $t('controlTower.cases.removeParticipant') }}</UiButton>
          </li>
        </ul>
        <p v-else>{{ $t('controlTower.cases.noCollaborators') }}</p>
      </div>
      <div class="case-drawer__participant-group">
        <h5>{{ $t('controlTower.cases.observers') }}</h5>
        <ul v-if="observers.length">
          <li v-for="p in observers" :key="p.userId" class="case-drawer__participant-row">
            <span>{{ p.displayName || userName(p.userId) }}</span>
            <UiButton size="sm" variant="ghost" @click="updateParticipantRole(caseItem.id, p.userId, 'collaborator')">{{ $t('controlTower.cases.makeCollaborator') }}</UiButton>
            <UiButton size="sm" variant="ghost" @click="removeParticipant(caseItem.id, p.userId)">{{ $t('controlTower.cases.removeParticipant') }}</UiButton>
          </li>
        </ul>
        <p v-else>{{ $t('controlTower.cases.noObservers') }}</p>
      </div>
      <div class="case-drawer__inline-form">
        <label>
          <span>{{ $t('controlTower.cases.addParticipant') }}</span>
          <select v-model="newParticipantUserId">
            <option value="">{{ $t('controlTower.cases.selectParticipant') }}</option>
            <option
              v-for="user in tenantUsers ?? []"
              :key="user.id"
              :value="user.id"
              :disabled="user.id === ownerUserId"
            >
              {{ user.displayName }}
            </option>
          </select>
        </label>
        <select v-model="newParticipantRole">
          <option value="collaborator">{{ $t('controlTower.cases.collaborator') }}</option>
          <option value="observer">{{ $t('controlTower.cases.observer') }}</option>
        </select>
        <UiButton size="sm" :disabled="!newParticipantUserId || actionLoading" @click="onAddParticipant">{{ $t('controlTower.cases.addParticipant') }}</UiButton>
      </div>
    </section>

    <section v-else-if="activeSection === 'timeline'" class="case-drawer__section">
      <h4>{{ $t('controlTower.cases.timeline') }}</h4>
      <p v-if="timelineLoading && !timelineEntries.length">{{ $t('common.loading') }}</p>
      <p v-else-if="!timelineEntries.length">{{ $t('controlTower.cases.noTimelineEvents') }}</p>
      <ol v-else class="case-drawer__timeline">
        <li v-for="entry in timelineEntries" :key="entry.id">
          <span class="case-drawer__badge">{{ $t(`controlTower.cases.timelineCategories.${caseTimelineCategory(entry.source, entry.actionType)}`) }}</span>
          <strong>{{ timelineLabel(entry) }}</strong>
          <small>{{ formatCaseDateTime(entry.occurredAt) }}</small>
          <span v-if="entry.actorUserId"> · {{ userName(entry.actorUserId) }}</span>
        </li>
      </ol>
      <UiButton v-if="timelineHasNext" size="sm" variant="secondary" :loading="timelineLoading" @click="loadOlderTimeline(caseItem.id)">
        {{ $t('controlTower.cases.loadOlderTimeline') }}
      </UiButton>
    </section>

    <section v-else-if="activeSection === 'playbooks'" class="case-drawer__section">
      <h4>{{ $t('controlTower.automation.playbooksSection') }}</h4>
      <ControlTowerRecommendationCard
        :recommendations="(caseItem as any)?.playbookRecommendations ?? []"
        :loading="actionLoading"
      />
      <ControlTowerPlaybookExecutionPanel
        v-for="exec in ((caseItem as any)?.playbookExecutions ?? [])"
        :key="exec.id"
        :execution="exec"
        :loading="actionLoading"
      />
      <p v-if="!(caseItem as any)?.playbookRecommendations?.length && !(caseItem as any)?.playbookExecutions?.length">
        {{ $t('controlTower.automation.noRecommendations') }}
      </p>
    </section>
  </aside>
</template>

<style scoped>
.case-drawer {
  position: fixed;
  top: 0;
  right: 0;
  width: min(520px, 100vw);
  height: 100vh;
  background: var(--color-surface, #fff);
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.1);
  padding: 1rem;
  z-index: 55;
  overflow: auto;
}
.case-drawer__header { display: flex; justify-content: space-between; gap: 0.5rem; }
.case-drawer__ref { margin: 0; font-family: monospace; font-size: 0.8125rem; color: var(--color-text-muted, #666); }
.case-drawer__close { background: none; border: none; font-size: 1.5rem; cursor: pointer; }
.case-drawer__nav { display: flex; flex-wrap: wrap; gap: 0.35rem; margin: 0.75rem 0 1rem; }
.case-drawer__nav-btn { border: 1px solid var(--color-border, #ddd); background: transparent; border-radius: 999px; padding: 0.2rem 0.6rem; font-size: 0.8125rem; cursor: pointer; }
.case-drawer__nav-btn--active { border-color: var(--color-primary, #3366ff); background: var(--color-primary-soft, #eef3ff); }
.case-drawer__section { margin-top: 0.5rem; padding-top: 0.75rem; border-top: 1px solid var(--color-border, #eee); }
.case-drawer__meta { display: grid; grid-template-columns: auto 1fr; gap: 0.35rem 0.75rem; font-size: 0.875rem; }
.case-drawer__actions, .case-drawer__inline-actions { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.75rem; }
.case-drawer__inline-form { display: flex; flex-direction: column; gap: 0.5rem; margin-top: 0.5rem; }
.case-drawer__select, .case-drawer__inline-form input, .case-drawer__inline-form textarea, .case-drawer__inline-form select {
  width: 100%; padding: 0.5rem; border: 1px solid var(--color-border, #ddd); border-radius: var(--radius-sm, 4px);
}
.case-drawer__severity-controls { display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: end; margin: 0.75rem 0; }
.case-drawer__warn-list { font-size: 0.875rem; color: var(--color-warning-text, #9a6700); padding-left: 1rem; }
.case-drawer__badge { display: inline-block; font-size: 0.75rem; padding: 0.05rem 0.4rem; border: 1px solid var(--color-border, #ddd); border-radius: 999px; margin-right: 0.35rem; }
.case-drawer__badge[data-overdue='true'] { border-color: #c62828; }
.case-drawer__cards { display: grid; gap: 0.75rem; }
.case-drawer__card { border: 1px solid var(--color-border, #eee); border-radius: var(--radius-sm, 4px); padding: 0.75rem; }
.case-drawer__card dl { display: grid; grid-template-columns: auto 1fr; gap: 0.25rem 0.75rem; font-size: 0.8125rem; margin: 0.5rem 0 0; }
.case-drawer__limitation, .case-drawer__hint { font-size: 0.8125rem; color: var(--color-text-muted, #666); }
.case-drawer__participant-group { margin-bottom: 1rem; }
.case-drawer__participant-row { display: flex; flex-wrap: wrap; gap: 0.35rem; align-items: center; margin-bottom: 0.35rem; }
.case-drawer__timeline { list-style: none; padding: 0; margin: 0; }
.case-drawer__timeline li { padding: 0.5rem 0; border-bottom: 1px solid var(--color-border, #eee); }
.case-drawer__note p { margin: 0 0 0.25rem; white-space: pre-wrap; }
</style>
