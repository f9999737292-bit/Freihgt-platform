<script setup lang="ts">
import type { AutomationRecommendation, DismissReason } from '~/types/automation'

const props = defineProps<{
  recommendations: AutomationRecommendation[]
  loading?: boolean
}>()

const emit = defineEmits<{
  accept: [recommendation: AutomationRecommendation]
  dismiss: [recommendation: AutomationRecommendation, reason: DismissReason]
}>()

const { t } = useI18n()
const dismissTarget = ref<AutomationRecommendation | null>(null)
const dismissReason = ref<DismissReason>('not_relevant')

const dismissReasons: DismissReason[] = ['not_relevant', 'already_handled', 'duplicate', 'false_positive', 'other']

function openDismiss(rec: AutomationRecommendation) {
  dismissTarget.value = rec
  dismissReason.value = 'not_relevant'
}

function confirmDismiss() {
  if (dismissTarget.value) {
    emit('dismiss', dismissTarget.value, dismissReason.value)
    dismissTarget.value = null
  }
}
</script>

<template>
  <section v-if="recommendations.length" class="ct-recommendations">
    <h4>{{ $t('controlTower.automation.recommendationsTitle') }}</h4>
    <article v-for="rec in recommendations" :key="rec.id" class="ct-recommendation-card">
      <header>
        <strong>{{ rec.playbookName || rec.playbookId }}</strong>
        <time>{{ rec.createdAt }}</time>
      </header>
      <p>{{ $t('controlTower.automation.recommendedAction') }}</p>
      <ul v-if="rec.matchedConditions?.length" class="ct-recommendation-card__conditions">
        <li v-for="(cond, idx) in rec.matchedConditions.filter((c) => c.matched)" :key="idx">
          {{ cond.field }} {{ cond.operator }} {{ cond.expected }}
        </li>
      </ul>
      <div class="ct-recommendation-card__actions">
        <UiButton size="sm" :disabled="loading" @click="emit('accept', rec)">
          {{ $t('controlTower.automation.startPlaybook') }}
        </UiButton>
        <UiButton size="sm" variant="secondary" :disabled="loading" @click="openDismiss(rec)">
          {{ $t('controlTower.automation.dismiss') }}
        </UiButton>
      </div>
    </article>

    <dialog v-if="dismissTarget" open class="ct-recommendation-dismiss">
      <p>{{ $t('controlTower.automation.dismissReason') }}</p>
      <select v-model="dismissReason">
        <option v-for="reason in dismissReasons" :key="reason" :value="reason">
          {{ $t(`controlTower.automation.dismissReasons.${reason}`) }}
        </option>
      </select>
      <div>
        <UiButton size="sm" @click="confirmDismiss">{{ $t('controlTower.automation.dismiss') }}</UiButton>
        <UiButton size="sm" variant="secondary" @click="dismissTarget = null">{{ $t('common.cancel') }}</UiButton>
      </div>
    </dialog>
  </section>
  <p v-else class="ct-recommendations-empty">{{ $t('controlTower.automation.noRecommendations') }}</p>
</template>

<style scoped>
.ct-recommendation-card {
  border: 1px solid var(--border-subtle, #ddd);
  border-radius: 8px;
  padding: 12px;
  margin-bottom: 8px;
}
.ct-recommendation-card__conditions {
  font-size: 0.875rem;
  color: var(--text-muted, #666);
}
.ct-recommendation-card__actions {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}
</style>
