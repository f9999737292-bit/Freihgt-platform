<script setup lang="ts">
import type { GuardedAction } from '~/types/automation'
import { formatLowCodeDate } from '~/types/lowCode'

const props = defineProps<{
  executionId: string
}>()

const emit = defineEmits<{ refresh: [] }>()

const { t } = useI18n()
const { pushToast } = useToast()
const { hasPermission } = usePermissions()
const { listGuardedActions, approveGuardedAction, rejectGuardedAction } = useAutomationApi()

const actions = ref<GuardedAction[]>([])
const loading = ref(false)
const actionBusyId = ref<string | null>(null)

const canApprove = computed(() => hasPermission('automation.approve') || hasPermission('automation.manage'))

async function load() {
  if (!props.executionId) return
  loading.value = true
  try {
    const page = await listGuardedActions(props.executionId)
    actions.value = page.items ?? []
  } catch (error) {
    actions.value = []
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    loading.value = false
  }
}

watch(() => props.executionId, load, { immediate: true })

function statusLabel(action: GuardedAction): string {
  return t(`controlTower.automation.guardedStatuses.${action.status}`, action.status)
}

function guardLabel(action: GuardedAction): string {
  return t(`controlTower.automation.guardDecisions.${action.guardDecision}`, action.guardDecision)
}

async function onApprove(action: GuardedAction) {
  if (!canApprove.value) return
  actionBusyId.value = action.id
  try {
    await approveGuardedAction(props.executionId, action.id)
    pushToast('success', t('controlTower.automation.actionApproved', 'Action approved'))
    await load()
    emit('refresh')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    actionBusyId.value = null
  }
}

async function onReject(action: GuardedAction) {
  if (!canApprove.value) return
  actionBusyId.value = action.id
  try {
    await rejectGuardedAction(props.executionId, action.id, t('controlTower.automation.operatorRejected', 'Rejected by operator'))
    pushToast('success', t('controlTower.automation.actionRejected', 'Action rejected'))
    await load()
    emit('refresh')
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    actionBusyId.value = null
  }
}

function responseSummary(action: GuardedAction): string {
  if (!action.response) return '—'
  const raw = action.response as Record<string, unknown>
  const reason = raw.reason ?? raw.delayReason ?? raw.status
  const comment = raw.comment ?? raw.notes
  const parts = [reason, comment].filter(Boolean).map(String)
  return parts.join(' · ') || JSON.stringify(raw)
}
</script>

<template>
  <section class="guarded-actions">
    <header>
      <h4>{{ $t('controlTower.automation.guardedActions', 'Guarded actions') }}</h4>
    </header>

    <p v-if="loading">{{ $t('common.loading') }}</p>
    <UiEmptyState
      v-else-if="!actions.length"
      :title="$t('controlTower.automation.noGuardedActions', 'No guarded actions for this execution')"
    />
    <div v-else class="guarded-actions__list">
      <article v-for="action in actions" :key="action.id" class="guarded-actions__item" :data-status="action.status">
        <div class="guarded-actions__row">
          <strong>{{ action.actionType }}</strong>
          <span class="guarded-actions__badge">{{ statusLabel(action) }}</span>
        </div>
        <dl class="guarded-actions__meta">
          <div><dt>{{ $t('controlTower.automation.guardDecision', 'Guard') }}</dt><dd>{{ guardLabel(action) }}</dd></div>
          <div v-if="action.guardReason"><dt>{{ $t('controlTower.automation.guardReason', 'Reason') }}</dt><dd>{{ action.guardReason }}</dd></div>
          <div v-if="action.shipmentId"><dt>{{ $t('shipments.shipment') }}</dt><dd>{{ action.shipmentId }}</dd></div>
          <div v-if="action.driverId"><dt>{{ $t('drivers.driver') }}</dt><dd>{{ action.driverId }}</dd></div>
          <div v-if="action.driverTaskId"><dt>{{ $t('controlTower.automation.driverTaskId', 'Driver task') }}</dt><dd>{{ action.driverTaskId }}</dd></div>
          <div v-if="action.expiresAt"><dt>{{ $t('controlTower.automation.expiresAt', 'Expires') }}</dt><dd>{{ formatLowCodeDate(action.expiresAt) }}</dd></div>
          <div v-if="action.createdAt"><dt>{{ $t('common.createdAt') }}</dt><dd>{{ formatLowCodeDate(action.createdAt) }}</dd></div>
          <div v-if="action.response"><dt>{{ $t('controlTower.automation.driverResponse', 'Driver response') }}</dt><dd>{{ responseSummary(action) }}</dd></div>
          <div v-if="action.approval"><dt>{{ $t('controlTower.automation.approval', 'Approval') }}</dt><dd>{{ action.approval.status }}</dd></div>
        </dl>

        <div v-if="action.status === 'waiting_approval' && canApprove" class="guarded-actions__actions">
          <UiButton size="sm" :loading="actionBusyId === action.id" @click="onApprove(action)">
            {{ $t('common.approve') }}
          </UiButton>
          <UiButton size="sm" variant="secondary" :loading="actionBusyId === action.id" @click="onReject(action)">
            {{ $t('common.reject') }}
          </UiButton>
        </div>
        <p v-else-if="action.status === 'waiting_approval'" class="guarded-actions__hint">
          {{ $t('common.insufficientPermission') }}
        </p>
      </article>
    </div>
  </section>
</template>

<style scoped>
.guarded-actions__list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.guarded-actions__item {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: 12px;
  background: var(--color-surface);
}

.guarded-actions__row {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.guarded-actions__badge {
  font-size: 0.8125rem;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.guarded-actions__meta {
  display: grid;
  gap: 4px;
  margin: 0;
}

.guarded-actions__meta div {
  display: grid;
  grid-template-columns: 140px 1fr;
  gap: 8px;
}

.guarded-actions__meta dt {
  color: var(--color-text-muted);
  font-size: 0.8125rem;
}

.guarded-actions__meta dd {
  margin: 0;
  word-break: break-word;
}

.guarded-actions__actions {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}

.guarded-actions__hint {
  margin: 8px 0 0;
  color: var(--color-text-muted);
  font-size: 0.875rem;
}
</style>
