<script setup lang="ts">
import { useTenderWorkspaceState } from '~/composables/useTenderWorkspaceState'

defineProps<{
  companyName?: (id: string) => string
}>()

const workspace = useTenderWorkspaceState()

const newTarget = reactive({
  carrier_company_id: '',
  target_share_pct: '0',
})

function carrierLabel(id: string, companyName?: (id: string) => string) {
  return companyName?.(id) || id
}

function addTarget() {
  if (!newTarget.carrier_company_id.trim()) return
  workspace.value.quotaTargets.push({
    carrier_company_id: newTarget.carrier_company_id.trim(),
    target_share_pct: Number(newTarget.target_share_pct),
  })
  newTarget.carrier_company_id = ''
  newTarget.target_share_pct = '0'
}

function removeTarget(index: number) {
  workspace.value.quotaTargets.splice(index, 1)
}
</script>

<template>
  <section class="tender-quota">
    <h3>{{ $t('tender.tabs.quota') }}</h3>

    <div class="tender-quota__policy">
      <UiInput
        :model-value="String(workspace.quotaPolicy.tolerance_pct ?? '')"
        type="number"
        :label="$t('tender.tolerance')"
        @update:model-value="workspace.quotaPolicy.tolerance_pct = Number($event)"
      />
      <UiInput
        :model-value="String(workspace.quotaPolicy.max_correction_pct ?? '')"
        type="number"
        :label="$t('tender.maxCorrection')"
        @update:model-value="workspace.quotaPolicy.max_correction_pct = Number($event)"
      />
      <UiInput
        :model-value="workspace.quotaPolicy.period_type || ''"
        :label="$t('tender.periodType')"
        @update:model-value="workspace.quotaPolicy.period_type = $event"
      />
      <label class="tender-quota__checkbox">
        <input v-model="workspace.quotaPolicy.carry_balance" type="checkbox">
        {{ $t('tender.carryBalance') }}
      </label>
    </div>

    <section class="tender-quota__targets">
      <h4>{{ $t('tender.targetAllocation') }}</h4>
      <div class="tender-quota__target-form">
        <UiInput v-model="newTarget.carrier_company_id" :label="$t('tender.carrierId')" />
        <UiInput
          v-model="newTarget.target_share_pct"
          type="number"
          :label="$t('tender.targetShare')"
        />
        <UiButton size="sm" @click="addTarget">{{ $t('common.create') }}</UiButton>
      </div>
      <table v-if="workspace.quotaTargets.length" class="tender-quota__table">
        <thead>
          <tr>
            <th>{{ $t('tender.carrier') }}</th>
            <th>{{ $t('tender.targetShare') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="(target, index) in workspace.quotaTargets" :key="`${target.carrier_company_id}-${index}`">
            <td>{{ carrierLabel(target.carrier_company_id, companyName) }}</td>
            <td>{{ target.target_share_pct }}%</td>
            <td>
              <UiButton size="sm" variant="secondary" @click="removeTarget(index)">
                {{ $t('tender.removeTarget') }}
              </UiButton>
            </td>
          </tr>
        </tbody>
      </table>
    </section>

    <section v-if="workspace.quotaPositions.length" class="tender-quota__balance">
      <h4>{{ $t('tender.quotaBalance') }}</h4>
      <table class="tender-quota__table">
        <thead>
          <tr>
            <th>{{ $t('tender.carrier') }}</th>
            <th>{{ $t('tender.targetShare') }}</th>
            <th>{{ $t('tender.actualShare') }}</th>
            <th>{{ $t('tender.balance') }}</th>
            <th>{{ $t('common.status') }}</th>
            <th>{{ $t('tender.adjustment') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in workspace.quotaPositions" :key="row.carrier_company_id">
            <td>{{ carrierLabel(row.carrier_company_id, companyName) }}</td>
            <td>{{ row.target_share_pct.toFixed(2) }}%</td>
            <td>{{ row.actual_share_pct.toFixed(2) }}%</td>
            <td>{{ row.balance_pp.toFixed(2) }}</td>
            <td>{{ row.status }}</td>
            <td>{{ row.next_adjustment_pct.toFixed(2) }}%</td>
          </tr>
        </tbody>
      </table>
    </section>

    <p v-else class="tender-quota__hint">{{ $t('tender.quotaHint') }}</p>
  </section>
</template>

<style scoped>
.tender-quota__policy,
.tender-quota__targets,
.tender-quota__balance {
  margin-top: 1rem;
}

.tender-quota__policy {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.75rem;
}

.tender-quota__checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-top: 1.5rem;
}

.tender-quota__target-form {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.75rem;
  align-items: end;
}

.tender-quota__table {
  width: 100%;
  margin-top: 1rem;
  border-collapse: collapse;
}

.tender-quota__table th,
.tender-quota__table td {
  padding: 0.5rem;
  border-bottom: 1px solid var(--color-border);
  text-align: left;
}

.tender-quota__hint {
  margin-top: 1rem;
  color: var(--color-text-muted);
}
</style>
