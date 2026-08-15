<script setup lang="ts">
import { formatApiErrorForUser } from '~/composables/useApi'
import { useTenderEvaluationApi } from '~/composables/useTenderEvaluationApi'
import { useTenderWorkspaceState } from '~/composables/useTenderWorkspaceState'
import { ALLOCATION_STRATEGIES } from '~/types/tender'

const props = defineProps<{
  rfxEventId: string
  companyName?: (id: string) => string
}>()

const { runAllocationScenario } = useTenderEvaluationApi()
const { pushToast } = useToast()
const { t } = useI18n()
const { canEvaluateTender } = usePermissions()
const workspace = useTenderWorkspaceState()

const loading = ref(false)
const scenarioName = ref('Primary scenario')

function carrierLabel(id: string) {
  return props.companyName?.(id) || id
}

async function runScenario() {
  if (!canEvaluateTender()) {
    pushToast('error', t('common.insufficientPermission'))
    return
  }
  if (!workspace.value.evaluationId) {
    pushToast('error', t('tender.evaluationRequired'))
    return
  }

  loading.value = true
  try {
    const response = await runAllocationScenario({
      evaluation_id: workspace.value.evaluationId,
      name: scenarioName.value,
      config: workspace.value.allocationConfig,
      quota_targets: workspace.value.quotaTargets.length ? workspace.value.quotaTargets : undefined,
      quota_policy: workspace.value.quotaPolicy,
      actual_shares: workspace.value.actualShares,
    })
    workspace.value.scenarioId = response.scenario_id
    workspace.value.allocationOutcome = response.outcome
    workspace.value.quotaPositions = response.quota
    if (response.quota.length) {
      workspace.value.quotaTargets = response.quota.map((item) => ({
        carrier_company_id: item.carrier_company_id,
        target_share_pct: item.target_share_pct,
      }))
    }
    pushToast('success', t('tender.allocationComplete'))
  } catch (error) {
    pushToast('error', formatApiErrorForUser(error))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="tender-allocation">
    <header class="tender-allocation__header">
      <h3>{{ $t('tender.tabs.allocation') }}</h3>
      <UiButton v-if="canEvaluateTender()" :loading="loading" size="sm" @click="runScenario">
        {{ $t('tender.runAllocation') }}
      </UiButton>
    </header>

    <div class="tender-allocation__form">
      <UiInput v-model="scenarioName" :label="$t('tender.scenarioName')" />
      <UiSelect
        v-model="workspace.allocationConfig.strategy"
        :label="$t('tender.strategy')"
        :options="ALLOCATION_STRATEGIES.map((strategy) => ({ value: strategy, label: strategy }))"
      />
      <UiInput
        :model-value="String(workspace.allocationConfig.constraints.total_volume ?? '')"
        type="number"
        :label="$t('tender.totalVolume')"
        @update:model-value="workspace.allocationConfig.constraints.total_volume = Number($event)"
      />
      <UiInput
        :model-value="String(workspace.allocationConfig.constraints.min_suppliers ?? '')"
        type="number"
        :label="$t('tender.minSuppliers')"
        @update:model-value="workspace.allocationConfig.constraints.min_suppliers = Number($event)"
      />
      <UiInput
        :model-value="String(workspace.allocationConfig.constraints.max_suppliers ?? '')"
        type="number"
        :label="$t('tender.maxSuppliers')"
        @update:model-value="workspace.allocationConfig.constraints.max_suppliers = Number($event)"
      />
      <UiInput
        :model-value="String(workspace.allocationConfig.constraints.max_carrier_share_pct ?? '')"
        type="number"
        :label="$t('tender.maxCarrierShare')"
        @update:model-value="workspace.allocationConfig.constraints.max_carrier_share_pct = Number($event)"
      />
    </div>

    <div v-if="workspace.allocationOutcome" class="tender-allocation__result">
      <p>
        <strong>{{ $t('common.status') }}:</strong>
        {{ workspace.allocationOutcome.status }}
      </p>
      <ul v-if="workspace.allocationOutcome.reasons?.length">
        <li v-for="reason in workspace.allocationOutcome.reasons" :key="reason">{{ reason }}</li>
      </ul>

      <table v-if="workspace.allocationOutcome.lines?.length" class="tender-allocation__table">
        <thead>
          <tr>
            <th>{{ $t('tender.carrier') }}</th>
            <th>{{ $t('tender.totalScore') }}</th>
            <th>{{ $t('tender.baseShare') }}</th>
            <th>{{ $t('tender.quotaAdjustment') }}</th>
            <th>{{ $t('tender.finalShare') }}</th>
            <th>{{ $t('tender.proposedVolume') }}</th>
            <th>{{ $t('tender.capacity') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="line in workspace.allocationOutcome.lines" :key="line.carrier_company_id">
            <td>{{ carrierLabel(line.carrier_company_id) }}</td>
            <td>{{ line.score.toFixed(2) }}</td>
            <td>{{ line.base_share_pct.toFixed(2) }}%</td>
            <td>{{ line.balance_adjustment_pct.toFixed(2) }}%</td>
            <td>{{ line.proposed_share_pct.toFixed(2) }}%</td>
            <td>{{ line.proposed_volume.toFixed(2) }}</td>
            <td>{{ line.committed_capacity.toFixed(2) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <UiEmptyState v-else :title="$t('tender.noAllocationYet')" />
  </section>
</template>

<style scoped>
.tender-allocation__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.tender-allocation__form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.75rem;
  margin-top: 1rem;
}

.tender-allocation__result {
  margin-top: 1rem;
}

.tender-allocation__table {
  width: 100%;
  margin-top: 1rem;
  border-collapse: collapse;
}

.tender-allocation__table th,
.tender-allocation__table td {
  padding: 0.5rem;
  border-bottom: 1px solid var(--color-border);
  text-align: left;
}
</style>
