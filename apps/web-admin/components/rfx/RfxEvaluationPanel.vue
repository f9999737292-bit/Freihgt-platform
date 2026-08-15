<script setup lang="ts">
import { formatApiErrorForUser } from '~/composables/useApi'
import { useTenderEvaluationApi } from '~/composables/useTenderEvaluationApi'
import { useTenderWorkspaceState } from '~/composables/useTenderWorkspaceState'
import { TENDER_SCORING_FACTORS } from '~/types/tender'
import { validateScoringWeights } from '~/utils/tenderValidation'

const props = defineProps<{
  rfxEventId: string
  companyName?: (id: string) => string
}>()

const { createScoringTemplate, runEvaluation } = useTenderEvaluationApi()
const { pushToast } = useToast()
const { t } = useI18n()
const { canEvaluateTender } = usePermissions()
const workspace = useTenderWorkspaceState()

const loading = ref(false)
const expandedCarrier = ref<string | null>(null)

const weightValidation = computed(() => validateScoringWeights(workspace.value.scoringFactors))
const weightTotal = computed(() => weightValidation.value.totalWeight.toFixed(2))

function carrierLabel(id: string) {
  return props.companyName?.(id) || id
}

function qualificationFor(carrierId: string) {
  return workspace.value.evaluation?.qualification.find((q) => q.carrier_company_id === carrierId)
}

async function saveTemplateAndEvaluate() {
  if (!canEvaluateTender()) {
    pushToast('error', t('common.insufficientPermission'))
    return
  }
  if (!weightValidation.value.valid) {
    pushToast('error', t('tender.scoringValidationFailed'))
    return
  }

  loading.value = true
  try {
    if (!workspace.value.templateVersionId) {
      const created = await createScoringTemplate(
        `rfx-${props.rfxEventId.slice(0, 8)}`,
        t('tender.defaultTemplateName'),
        workspace.value.scoringFactors,
      )
      workspace.value.templateVersionId = created.version_id
    }

    const result = await runEvaluation(props.rfxEventId, {
      scoring_template_version_id: workspace.value.templateVersionId,
      qualification_rules: workspace.value.qualificationRules,
      required_volume: workspace.value.requiredVolume,
    })
    workspace.value.evaluation = result
    workspace.value.evaluationId = result.evaluation_id
    pushToast('success', t('tender.evaluationComplete'))
  } catch (error) {
    pushToast('error', formatApiErrorForUser(error))
  } finally {
    loading.value = false
  }
}

function addFactorRow() {
  const unused = TENDER_SCORING_FACTORS.find(
    (factor) => !workspace.value.scoringFactors.some((row) => row.factor === factor),
  )
  if (!unused) return
  workspace.value.scoringFactors.push({ factor: unused, weight: 0 })
}

function removeFactorRow(index: number) {
  workspace.value.scoringFactors.splice(index, 1)
}
</script>

<template>
  <section class="rfx-evaluation">
    <header class="rfx-evaluation__header">
      <h3>{{ $t('tender.evaluationTitle') }}</h3>
      <UiButton v-if="canEvaluateTender()" :loading="loading" size="sm" @click="saveTemplateAndEvaluate">
        {{ $t('tender.runEvaluation') }}
      </UiButton>
    </header>

    <section class="rfx-evaluation__builder">
      <h4>{{ $t('tender.scoringBuilder') }}</h4>
      <table class="rfx-evaluation__table">
        <thead>
          <tr>
            <th>{{ $t('tender.factor') }}</th>
            <th>{{ $t('tender.weight') }}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, index) in workspace.scoringFactors" :key="`${row.factor}-${index}`">
            <td>
              <UiSelect
                v-model="row.factor"
                :options="TENDER_SCORING_FACTORS.map((factor) => ({ value: factor, label: factor }))"
              />
            </td>
            <td>
              <UiInput
                :model-value="String(row.weight)"
                type="number"
                @update:model-value="row.weight = Number($event)"
              />
            </td>
            <td>
              <UiButton size="sm" variant="secondary" @click="removeFactorRow(index)">
                {{ $t('tender.removeFactor') }}
              </UiButton>
            </td>
          </tr>
        </tbody>
      </table>
      <div class="rfx-evaluation__builder-actions">
        <UiButton size="sm" variant="secondary" @click="addFactorRow">{{ $t('tender.addFactor') }}</UiButton>
        <span :class="{ 'rfx-evaluation__total--invalid': !weightValidation.valid }">
          {{ $t('tender.weightTotal') }}: {{ weightTotal }}%
        </span>
      </div>
      <p v-if="!weightValidation.valid" class="rfx-evaluation__error">
        {{ $t('tender.scoringValidationFailed') }}
      </p>
    </section>

    <section class="rfx-evaluation__rules">
      <h4>{{ $t('tender.qualificationRules') }}</h4>
      <div class="rfx-evaluation__rules-grid">
        <UiInput
          :model-value="String(workspace.qualificationRules.minimum_sla_score ?? '')"
          type="number"
          :label="$t('tender.minimumSla')"
          @update:model-value="workspace.qualificationRules.minimum_sla_score = Number($event)"
        />
        <UiInput
          :model-value="String(workspace.qualificationRules.minimum_capacity ?? '')"
          type="number"
          :label="$t('tender.minimumCapacity')"
          @update:model-value="workspace.qualificationRules.minimum_capacity = Number($event)"
        />
        <UiInput
          :model-value="String(workspace.requiredVolume)"
          type="number"
          :label="$t('tender.requiredVolume')"
          @update:model-value="workspace.requiredVolume = Number($event)"
        />
      </div>
    </section>

    <p v-if="!workspace.evaluation" class="rfx-evaluation__hint">{{ $t('tender.evaluationHint') }}</p>

    <table v-if="workspace.evaluation?.scores?.length" class="rfx-evaluation__table">
      <thead>
        <tr>
          <th>{{ $t('tender.carrier') }}</th>
          <th>{{ $t('tender.qualification') }}</th>
          <th>{{ $t('tender.totalScore') }}</th>
          <th>{{ $t('tender.price') }}</th>
          <th>{{ $t('tender.sla') }}</th>
          <th>{{ $t('tender.kpi') }}</th>
          <th />
        </tr>
      </thead>
      <tbody>
        <template v-for="row in workspace.evaluation.scores" :key="row.carrier_company_id">
          <tr>
            <td>{{ carrierLabel(row.carrier_company_id) }}</td>
            <td>{{ qualificationFor(row.carrier_company_id)?.result }}</td>
            <td>{{ row.total_score.toFixed(2) }}</td>
            <td>{{ row.price_score.toFixed(2) }}</td>
            <td>{{ row.sla_score.toFixed(2) }}</td>
            <td>{{ row.carrier_kpi_score.toFixed(2) }}</td>
            <td>
              <UiButton
                size="sm"
                variant="secondary"
                @click="
                  expandedCarrier =
                    expandedCarrier === row.carrier_company_id ? null : row.carrier_company_id
                "
              >
                {{ $t('tender.scoreBreakdown') }}
              </UiButton>
            </td>
          </tr>
          <tr v-if="expandedCarrier === row.carrier_company_id">
            <td colspan="7">
              <RfxTenderScoreBreakdown
                :score="row"
                :qualification="qualificationFor(row.carrier_company_id)"
              />
            </td>
          </tr>
        </template>
      </tbody>
    </table>
  </section>
</template>

<style scoped>
.rfx-evaluation__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.rfx-evaluation__builder,
.rfx-evaluation__rules {
  margin-top: 1rem;
}

.rfx-evaluation__builder-actions {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-top: 0.75rem;
}

.rfx-evaluation__total--invalid {
  color: var(--color-danger, #b91c1c);
}

.rfx-evaluation__error {
  color: var(--color-danger, #b91c1c);
  margin-top: 0.5rem;
}

.rfx-evaluation__hint {
  color: var(--color-text-muted);
  margin-top: 0.75rem;
}

.rfx-evaluation__rules-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.75rem;
}

.rfx-evaluation__table {
  width: 100%;
  margin-top: 1rem;
  border-collapse: collapse;
}

.rfx-evaluation__table th,
.rfx-evaluation__table td {
  padding: 0.5rem;
  border-bottom: 1px solid var(--color-border);
  text-align: left;
  vertical-align: top;
}
</style>
