<script setup lang="ts">
import type { RfxQuestionnaireDefinition } from '~/types/rfx-questionnaire'
import { resolveI18nMapValue } from '~/utils/rfxTemplateI18n'

defineProps<{
  questionnaire: RfxQuestionnaireDefinition | null
  loading?: boolean
  error?: string | null
}>()

const { locale } = useI18n()

function sectionTitle(nameI18n: Record<string, string> | undefined, code: string) {
  return resolveI18nMapValue(nameI18n ?? {}, locale.value) || code
}

function questionTitle(nameI18n: Record<string, string> | undefined, code: string) {
  return resolveI18nMapValue(nameI18n ?? {}, locale.value) || code
}
</script>

<template>
  <div class="readonly-q">
    <p v-if="loading">{{ $t('common.loading') }}</p>
    <p v-else-if="error" class="readonly-q__error">{{ error }}</p>
    <p v-else-if="!questionnaire || questionnaire.sections.length === 0" class="readonly-q__empty">
      {{ $t('rfx.studio.noSections') }}
    </p>
    <div v-else class="readonly-q__sections">
      <section v-for="swq in questionnaire.sections" :key="swq.section.id" class="readonly-q__section">
        <h3>{{ sectionTitle(swq.section.name_i18n, swq.section.section_code) }}</h3>
        <ul>
          <li v-for="question in swq.questions" :key="question.id">
            <strong>{{ questionTitle(question.name_i18n, question.question_code) }}</strong>
            <span class="readonly-q__type">{{ question.question_type }}</span>
            <ul v-if="question.options?.length" class="readonly-q__options">
              <li v-for="opt in question.options" :key="opt.id">
                {{ resolveI18nMapValue(opt.label_i18n ?? {}, locale) || opt.option_code }}
              </li>
            </ul>
          </li>
        </ul>
      </section>
    </div>
  </div>
</template>

<style scoped>
.readonly-q { display: flex; flex-direction: column; gap: 1rem; }
.readonly-q__error { color: var(--color-danger, #b91c1c); }
.readonly-q__empty { color: var(--color-text-muted); }
.readonly-q__section { padding: 0.75rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); }
.readonly-q__section h3 { margin: 0 0 0.5rem; font-size: 1rem; }
.readonly-q__type { margin-left: 0.5rem; font-size: 0.75rem; color: var(--color-text-muted); }
.readonly-q__options { margin: 0.25rem 0 0; padding-left: 1.25rem; font-size: 0.875rem; }
</style>
