<script setup lang="ts">
import { formatApiErrorForUser } from '~/composables/useApi'
import { useTenderEvaluationApi } from '~/composables/useTenderEvaluationApi'
import { useTenderWorkspaceState } from '~/composables/useTenderWorkspaceState'

const props = defineProps<{
  rfxEventId: string
  companyName?: (id: string) => string
}>()

const {
  createAwardProposal,
  submitAwardProposal,
  approveAwardProposal,
  rejectAwardProposal,
  finalizeAwardProposal,
} = useTenderEvaluationApi()
const { pushToast } = useToast()
const { t } = useI18n()
const {
  canEvaluateTender,
  canApproveAward,
  canFinalizeAward,
} = usePermissions()
const workspace = useTenderWorkspaceState()

const loading = ref(false)

function carrierLabel(id: string) {
  return props.companyName?.(id) || id
}

function qualificationFor(carrierId: string) {
  return workspace.value.evaluation?.qualification.find((q) => q.carrier_company_id === carrierId)
}

function scoreFor(carrierId: string) {
  return workspace.value.evaluation?.scores.find((s) => s.carrier_company_id === carrierId)
}

function quotaFor(carrierId: string) {
  return workspace.value.quotaPositions.find((q) => q.carrier_company_id === carrierId)
}

async function createProposal() {
  if (!canEvaluateTender()) {
    pushToast('error', t('common.insufficientPermission'))
    return
  }
  if (!workspace.value.evaluationId || !workspace.value.scenarioId) {
    pushToast('error', t('tender.allocationRequired'))
    return
  }

  loading.value = true
  try {
    const response = await createAwardProposal({
      rfx_event_id: props.rfxEventId,
      evaluation_id: workspace.value.evaluationId,
      scenario_id: workspace.value.scenarioId,
      idempotency_key: `proposal-${props.rfxEventId}-${workspace.value.scenarioId}`,
    })
    workspace.value.proposalId = response.proposal_id
    workspace.value.proposalStatus = 'DRAFT_PROPOSAL'
    pushToast('success', t('tender.proposalCreated'))
  } catch (error) {
    pushToast('error', formatApiErrorForUser(error))
  } finally {
    loading.value = false
  }
}

async function submitProposal() {
  if (!workspace.value.proposalId) return
  loading.value = true
  try {
    await submitAwardProposal(workspace.value.proposalId)
    workspace.value.proposalStatus = 'PENDING_APPROVAL'
    pushToast('success', t('tender.proposalSubmitted'))
  } catch (error) {
    pushToast('error', formatApiErrorForUser(error))
  } finally {
    loading.value = false
  }
}

async function approveProposal() {
  if (!canApproveAward() || !workspace.value.proposalId) return
  loading.value = true
  try {
    await approveAwardProposal(workspace.value.proposalId)
    workspace.value.proposalStatus = 'APPROVED'
    pushToast('success', t('tender.proposalApproved'))
  } catch (error) {
    pushToast('error', formatApiErrorForUser(error))
  } finally {
    loading.value = false
  }
}

async function rejectProposal() {
  if (!canApproveAward() || !workspace.value.proposalId) return
  loading.value = true
  try {
    await rejectAwardProposal(workspace.value.proposalId)
    workspace.value.proposalStatus = 'REJECTED'
    pushToast('success', t('tender.proposalRejected'))
  } catch (error) {
    pushToast('error', formatApiErrorForUser(error))
  } finally {
    loading.value = false
  }
}

async function finalizeProposal() {
  if (!canFinalizeAward() || !workspace.value.proposalId) return
  loading.value = true
  try {
    const response = await finalizeAwardProposal(
      workspace.value.proposalId,
      `award-${workspace.value.proposalId}`,
    )
    workspace.value.proposalStatus = 'AWARDED'
    workspace.value.awardId = response.award_id
    workspace.value.conversion = response.conversion ?? null
    pushToast('success', t('tender.awardFinalized'))
  } catch (error) {
    pushToast('error', formatApiErrorForUser(error))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="tender-award">
    <header class="tender-award__header">
      <h3>{{ $t('tender.tabs.award') }}</h3>
      <div class="tender-award__actions">
        <UiButton
          v-if="canEvaluateTender() && !workspace.proposalId"
          size="sm"
          :loading="loading"
          @click="createProposal"
        >
          {{ $t('tender.createProposal') }}
        </UiButton>
        <UiButton
          v-if="workspace.proposalId && workspace.proposalStatus === 'DRAFT_PROPOSAL'"
          size="sm"
          :loading="loading"
          @click="submitProposal"
        >
          {{ $t('tender.submitProposal') }}
        </UiButton>
        <UiButton
          v-if="canApproveAward() && workspace.proposalStatus === 'PENDING_APPROVAL'"
          size="sm"
          :loading="loading"
          @click="approveProposal"
        >
          {{ $t('tender.approveProposal') }}
        </UiButton>
        <UiButton
          v-if="canApproveAward() && workspace.proposalStatus === 'PENDING_APPROVAL'"
          size="sm"
          variant="secondary"
          :loading="loading"
          @click="rejectProposal"
        >
          {{ $t('tender.rejectProposal') }}
        </UiButton>
        <UiButton
          v-if="canFinalizeAward() && workspace.proposalStatus === 'APPROVED'"
          size="sm"
          :loading="loading"
          @click="finalizeProposal"
        >
          {{ $t('tender.finalizeAward') }}
        </UiButton>
      </div>
    </header>

    <p v-if="workspace.proposalStatus" class="tender-award__status">
      {{ $t('tender.proposalStatus') }}: <strong>{{ workspace.proposalStatus }}</strong>
    </p>

    <table v-if="workspace.allocationOutcome?.lines?.length" class="tender-award__table">
      <thead>
        <tr>
          <th>{{ $t('tender.carrier') }}</th>
          <th>{{ $t('tender.qualification') }}</th>
          <th>{{ $t('tender.totalScore') }}</th>
          <th>{{ $t('tender.targetShare') }}</th>
          <th>{{ $t('tender.actualShare') }}</th>
          <th>{{ $t('tender.adjustment') }}</th>
          <th>{{ $t('tender.finalShare') }}</th>
          <th>{{ $t('tender.proposedVolume') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="line in workspace.allocationOutcome.lines" :key="line.carrier_company_id">
          <td>{{ carrierLabel(line.carrier_company_id) }}</td>
          <td>{{ qualificationFor(line.carrier_company_id)?.result || '—' }}</td>
          <td>{{ scoreFor(line.carrier_company_id)?.total_score.toFixed(2) || line.score.toFixed(2) }}</td>
          <td>{{ quotaFor(line.carrier_company_id)?.target_share_pct.toFixed(2) || '—' }}%</td>
          <td>{{ quotaFor(line.carrier_company_id)?.actual_share_pct.toFixed(2) || '—' }}%</td>
          <td>{{ line.balance_adjustment_pct.toFixed(2) }}%</td>
          <td>{{ line.proposed_share_pct.toFixed(2) }}%</td>
          <td>{{ line.proposed_volume.toFixed(2) }}</td>
        </tr>
      </tbody>
    </table>

    <section v-if="workspace.awardId" class="tender-award__result">
      <h4>{{ $t('tender.awardResult') }}</h4>
      <dl class="tender-award__result-grid">
        <div><dt>{{ $t('tender.awardId') }}</dt><dd>{{ workspace.awardId }}</dd></div>
        <div><dt>{{ $t('tender.proposalStatus') }}</dt><dd>{{ workspace.proposalStatus }}</dd></div>
        <div>
          <dt>{{ $t('tender.conversionStatus') }}</dt>
          <dd>{{ workspace.conversion?.status || '—' }}</dd>
        </div>
        <div>
          <dt>{{ $t('tender.transportOrderId') }}</dt>
          <dd>{{ workspace.conversion?.transport_order_id || '—' }}</dd>
        </div>
        <div>
          <dt>{{ $t('tender.evaluationId') }}</dt>
          <dd>{{ workspace.evaluationId || '—' }}</dd>
        </div>
        <div>
          <dt>{{ $t('tender.scenarioId') }}</dt>
          <dd>{{ workspace.scenarioId || '—' }}</dd>
        </div>
      </dl>
      <p v-if="workspace.conversion?.message" class="tender-award__message">
        {{ workspace.conversion.message }}
      </p>
    </section>

    <UiEmptyState v-else-if="!workspace.allocationOutcome?.lines?.length" :title="$t('tender.noAwardYet')" />
  </section>
</template>

<style scoped>
.tender-award__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.tender-award__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.tender-award__status {
  margin-top: 0.75rem;
}

.tender-award__table {
  width: 100%;
  margin-top: 1rem;
  border-collapse: collapse;
}

.tender-award__table th,
.tender-award__table td {
  padding: 0.5rem;
  border-bottom: 1px solid var(--color-border);
  text-align: left;
}

.tender-award__result {
  margin-top: 1.5rem;
  padding-top: 1rem;
  border-top: 1px solid var(--color-border);
}

.tender-award__result-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.75rem;
  margin-top: 0.75rem;
}

.tender-award__result-grid dt {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.tender-award__result-grid dd {
  margin: 0.15rem 0 0;
  font-family: ui-monospace, monospace;
  font-size: 0.8125rem;
}

.tender-award__message {
  margin-top: 0.75rem;
  color: var(--color-text-muted);
}
</style>
