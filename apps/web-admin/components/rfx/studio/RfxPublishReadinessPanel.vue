<script setup lang="ts">

import type { RfxPublishReadinessResult } from '~/types/rfx-questionnaire'
import {
  readinessItemClass,
  readinessSummaryBadge,
  resolveReadinessStatusLabel,
} from '~/utils/rfxStudioQuestionnaire'

defineProps<{ result: RfxPublishReadinessResult | null }>()

const { t } = useI18n()

function statusLabel(status: string) {
  return resolveReadinessStatusLabel(status, t)
}

</script>



<template>

  <UiCard v-if="result" class="readiness-panel">

    <h2>{{ t('rfx.studio.validationTitle') }}</h2>



    <div class="summary">

      <UiBadge
        :status="readinessSummaryBadge(result).status"
        :tone="readinessSummaryBadge(result).tone"
      >
        {{ result.ready ? t('rfx.studio.readyPass') : t('rfx.studio.readyFail') }}
      </UiBadge>

      <span>{{ t('rfx.studio.errorsCount', { n: result.blocking_fail_count }) }}</span>

      <span>{{ t('rfx.studio.warningsCount', { n: result.warning_count }) }}</span>

    </div>



    <ul class="checks">

      <li

        v-for="item in result.items"

        :key="item.code"

        :class="readinessItemClass(item.status)"

      >

        <UiBadge :status="item.status" tone="neutral">{{ statusLabel(item.status) }}</UiBadge>

        <span class="check-message">{{ item.message }}</span>

        <code v-if="item.code" class="check-code">{{ item.code }}</code>

      </li>

    </ul>

  </UiCard>

  <p v-else class="muted">{{ t('rfx.studio.runValidationHint') }}</p>

</template>



<style scoped>

.readiness-panel {

  padding: 1rem;

}



.readiness-panel h2 {

  margin: 0 0 1rem;

  font-size: 1.125rem;

}



.summary {

  display: flex;

  flex-wrap: wrap;

  gap: 1rem;

  align-items: center;

  margin-bottom: 1rem;

  font-size: 0.875rem;

}



.checks {

  list-style: none;

  padding: 0;

  margin: 0;

  display: flex;

  flex-direction: column;

  gap: 0.5rem;

}



.checks li {

  display: flex;

  align-items: flex-start;

  gap: 0.5rem;

  flex-wrap: wrap;

  padding: 0.5rem;

  border-radius: var(--radius-md);

  background: var(--color-bg);

}



.check-message {

  flex: 1;

  min-width: 200px;

}



.check-code {

  font-size: 0.75rem;

  color: var(--color-text-muted);

}



.check--fail {

  border-left: 3px solid #b91c1c;

}



.check--warn {

  border-left: 3px solid #b45309;

}



.check--pass {

  border-left: 3px solid #047857;

}



.muted {

  color: var(--color-text-muted);

}

</style>

