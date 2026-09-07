<script setup lang="ts">
import type { V3ScoreExplanation, V3ScoreLoadState } from '~/types/rfx-score'
import { formatV3Score } from '~/composables/useRfxScoreApi'

const props = defineProps<{
  eventId: string
  responseId: string
  loadState: V3ScoreLoadState
  questionnaireScore?: number | null
  qualificationStatus?: string | null
  calculationStatus?: string | null
  knockoutTriggered?: boolean
  scoreModelVersion?: number | null
}>()

const emit = defineEmits<{ explain: [] }>()

const { t } = useI18n()

const showScore = computed(() => {
  if (props.loadState === 'LOADING') return t('common.loading')
  if (props.loadState === 'FAILED') return t('tenders.evaluation.v3ScoreFailed')
  if (props.loadState === 'PENDING') return t('tenders.evaluation.v3ScorePending')
  if (props.loadState === 'NOT_AVAILABLE') return '—'
  return formatV3Score(props.questionnaireScore)
})
</script>

<template>
  <div class="v3-score-cell" data-testid="v3-score-cell">
    <span data-testid="v3-questionnaire-score">{{ showScore }}</span>
    <Badge
      v-if="qualificationStatus && loadState === 'AVAILABLE'"
      :status="qualificationStatus"
      data-testid="v3-qualification-status"
    />
    <Badge
      v-if="knockoutTriggered"
      status="REJECTED"
      data-testid="v3-knockout-badge"
    >
      {{ t('tenders.evaluation.knockoutBadge') }}
    </Badge>
    <span v-if="scoreModelVersion" class="muted version" data-testid="v3-model-version">
      v{{ scoreModelVersion }}
    </span>
    <Button
      v-if="loadState === 'AVAILABLE'"
      size="sm"
      variant="secondary"
      data-testid="v3-explain-button"
      @click="emit('explain')"
    >
      {{ t('tenders.evaluation.explainScore') }}
    </Button>
  </div>
</template>

<style scoped>
.v3-score-cell { display: flex; flex-direction: column; gap: 0.25rem; align-items: flex-start; }
.version { font-size: 0.75rem; }
.muted { color: var(--color-text-muted); }
</style>
