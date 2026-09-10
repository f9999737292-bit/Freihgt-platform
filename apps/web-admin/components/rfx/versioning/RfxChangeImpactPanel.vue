<script setup lang="ts">
import type { RfxChangeImpactAnalysisResponse, RfxImpactClass } from '~/types/rfx-version-lifecycle'
import { requiresImpactConfirmation } from '~/types/rfx-version-lifecycle'
import { sortImpactClasses as sortBySeverity } from '~/utils/rfxVersionCompare'

defineProps<{
  analysis: RfxChangeImpactAnalysisResponse | null
  loading?: boolean
}>()

const emit = defineEmits<{ confirm: [] }>()

const { t } = useI18n()

function impactLabel(className: RfxImpactClass) {
  return t(`rfx.changeImpact.classes.${className}`)
}

function sortedClasses(classes: RfxImpactClass[]) {
  return sortBySeverity(classes)
}

function needsConfirmation(classes: RfxImpactClass[]) {
  return requiresImpactConfirmation(classes)
}
</script>

<template>
  <div class="impact-panel">
    <p v-if="loading">{{ $t('rfx.changeImpact.loading') }}</p>
    <p v-else-if="!analysis">{{ $t('rfx.changeImpact.noAnalysis') }}</p>

    <template v-else>
      <p class="impact-panel__hash">
        {{ $t('rfx.compare.diffHash') }}: <code>{{ analysis.canonical_diff_hash }}</code>
      </p>
      <p class="impact-panel__expiry">
        {{ $t('rfx.changeImpact.expiresAt') }}: {{ analysis.expires_at }}
      </p>

      <ul class="impact-panel__classes">
        <li v-for="cls in sortedClasses(analysis.impact_classes)" :key="cls">
          {{ impactLabel(cls) }}
        </li>
      </ul>

      <div class="impact-panel__counts">
        <span>{{ $t('rfx.changeImpact.draftResponses') }}: {{ analysis.affected_draft_response_count }}</span>
        <span>{{ $t('rfx.changeImpact.submittedResponses') }}: {{ analysis.affected_submitted_response_count }}</span>
      </div>

      <p v-if="analysis.scoring_affecting" class="impact-panel__warning">
        {{ $t('rfx.changeImpact.scoringAffecting') }}
      </p>
      <p v-if="analysis.knockout_affecting" class="impact-panel__warning">
        {{ $t('rfx.changeImpact.knockoutAffecting') }}
      </p>
      <p v-if="analysis.scoring_affecting || analysis.knockout_affecting" class="impact-panel__warning">
        {{ $t('rfx.changeImpact.rescoringRequired') }}
      </p>

      <button
        v-if="needsConfirmation(analysis.impact_classes)"
        type="button"
        class="btn btn--primary"
        @click="emit('confirm')"
      >
        {{ $t('rfx.changeImpact.confirmRepublish') }}
      </button>
    </template>
  </div>
</template>

<style scoped>
.impact-panel { display: flex; flex-direction: column; gap: 0.75rem; }
.impact-panel__hash, .impact-panel__expiry { font-size: 0.875rem; margin: 0; }
.impact-panel__classes { margin: 0; padding-left: 1.25rem; }
.impact-panel__counts { display: flex; gap: 1rem; font-size: 0.875rem; }
.impact-panel__warning { color: var(--color-warning-text, #b45309); margin: 0; font-weight: 500; }
</style>
