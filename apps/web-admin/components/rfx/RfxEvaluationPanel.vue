<script setup lang="ts">
import { DEFAULT_SCORING_TEMPLATE } from '~/types/tender'

const props = defineProps<{ rfxEventId: string }>()

const { createScoringTemplate, runEvaluation } = useTenderEvaluationApi()
const { pushToast } = useToast()
const { t } = useI18n()

const loading = ref(false)
const templateVersionId = ref('')
const evaluation = ref<Awaited<ReturnType<typeof runEvaluation>> | null>(null)

async function ensureTemplateAndEvaluate() {
  loading.value = true
  try {
    if (!templateVersionId.value) {
      const created = await createScoringTemplate(
        `default-${props.rfxEventId.slice(0, 8)}`,
        'Default Enterprise Template',
        DEFAULT_SCORING_TEMPLATE,
      )
      templateVersionId.value = created.version_id
    }
    evaluation.value = await runEvaluation(props.rfxEventId, {
      scoring_template_version_id: templateVersionId.value,
      qualification_rules: {
        minimum_sla_score: 75,
        minimum_capacity: 100,
        require_carrier_active: true,
      },
      required_volume: 500,
    })
    pushToast('success', t('tender.evaluationComplete'))
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="rfx-evaluation">
    <header class="rfx-evaluation__header">
      <h3>{{ $t('tender.evaluationTitle') }}</h3>
      <UiButton :loading="loading" size="sm" @click="ensureTemplateAndEvaluate">
        {{ $t('tender.runEvaluation') }}
      </UiButton>
    </header>

    <p v-if="!evaluation" class="rfx-evaluation__hint">{{ $t('tender.evaluationHint') }}</p>

    <table v-if="evaluation?.scores?.length" class="rfx-evaluation__table">
      <thead>
        <tr>
          <th>{{ $t('tender.carrier') }}</th>
          <th>{{ $t('tender.qualification') }}</th>
          <th>{{ $t('tender.totalScore') }}</th>
          <th>{{ $t('tender.price') }}</th>
          <th>{{ $t('tender.sla') }}</th>
          <th>{{ $t('tender.kpi') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in evaluation.scores" :key="row.carrier_company_id">
          <td>{{ row.carrier_company_id }}</td>
          <td>
            {{
              evaluation.qualification.find((q) => q.carrier_company_id === row.carrier_company_id)?.result
            }}
          </td>
          <td>{{ row.total_score.toFixed(2) }}</td>
          <td>{{ row.price_score.toFixed(2) }}</td>
          <td>{{ row.sla_score.toFixed(2) }}</td>
          <td>{{ row.carrier_kpi_score.toFixed(2) }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<style scoped>
.rfx-evaluation {
  margin-top: 1.5rem;
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
}

.rfx-evaluation__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.rfx-evaluation__hint {
  color: var(--color-text-muted);
  margin-top: 0.75rem;
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
}
</style>
