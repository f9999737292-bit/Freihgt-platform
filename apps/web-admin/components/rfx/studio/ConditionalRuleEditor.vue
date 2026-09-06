<script setup lang="ts">

import type { RfxConditionalExpression, RfxQuestion, RfxQuestionRule } from '~/types/rfx-questionnaire'

import { RULE_ACTIONS } from '~/types/rfx-questionnaire'



const props = defineProps<{ targetQuestion: RfxQuestion }>()



const api = useInjectedRfxQuestionnaireApi()

const { t } = useI18n()

const { pushToast } = useToast()



const allQuestions = computed(() => {

  const items: RfxQuestion[] = []

  for (const swq of api.studio.value?.sections ?? []) {

    items.push(...swq.questions)

  }

  return items

})



const sourceOptions = computed(() =>

  allQuestions.value

    .filter((q) => q.id !== props.targetQuestion.id)

    .map((q) => ({ value: q.question_code, label: `${q.question_code} — ${q.label}` })),

)



const rulesForTarget = computed(() =>

  (api.studio.value?.rules ?? []).filter((rule) => rule.target_question_id === props.targetQuestion.id),

)



function parseCondition(rule: RfxQuestionRule): RfxConditionalExpression {

  const raw = rule.condition_json

  if (raw && typeof raw === 'object' && 'operator' in raw) {

    return raw as RfxConditionalExpression

  }

  return { operator: 'EQUALS', source_question_code: '', value: '' }

}



const operatorOptions = [

  'EQUALS',

  'NOT_EQUALS',

  'IN',

  'NOT_IN',

  'IS_EMPTY',

  'IS_NOT_EMPTY',

].map((op) => ({ value: op, label: t(`rfx.studio.ruleOperators.${op}`) }))



const actionOptions = RULE_ACTIONS.map((action) => ({

  value: action,

  label: t(`rfx.studio.ruleActions.${action}`),

}))



async function addRule() {

  const sortOrder = (api.studio.value?.rules.length ?? 0) + 1

  const code = `RULE_${sortOrder}`

  const condition: RfxConditionalExpression = {

    operator: 'EQUALS',

    source_question_code: sourceOptions.value[0]?.value ?? '',

    value: '',

  }

  try {

    await api.createRule({

      rule_code: code,

      action: 'SHOW',

      target_question_code: props.targetQuestion.question_code,

      condition_json: condition,

      sort_order: sortOrder,

    })

  } catch (e) {

    pushToast('error', e instanceof Error ? e.message : t('common.error'))

  }

}



function scheduleRuleFieldUpdate(rule: RfxQuestionRule, fields: Record<string, unknown>) {

  api.scheduleRuleUpdate(rule.id, fields)

}



function onActionChange(rule: RfxQuestionRule, action: string) {

  scheduleRuleFieldUpdate(rule, { action })

}



function onSourceChange(rule: RfxQuestionRule, sourceCode: string) {

  const condition = { ...parseCondition(rule), source_question_code: sourceCode }

  scheduleRuleFieldUpdate(rule, { condition_json: condition })

}



function onOperatorChange(rule: RfxQuestionRule, operator: string) {

  const condition = { ...parseCondition(rule), operator }

  scheduleRuleFieldUpdate(rule, { condition_json: condition })

}



function onValueChange(rule: RfxQuestionRule, value: string) {

  const condition = { ...parseCondition(rule), value }

  scheduleRuleFieldUpdate(rule, { condition_json: condition })

}



async function removeRule(rule: RfxQuestionRule) {

  if (!confirm(t('rfx.studio.confirmDeleteRule'))) return

  try {

    await api.deleteRule(rule.id, rule.version)

  } catch (e) {

    pushToast('error', e instanceof Error ? e.message : t('common.error'))

  }

}

</script>



<template>

  <UiCard class="rules-editor">

    <header class="rules-header">

      <h3>{{ t('rfx.studio.rulesTitle') }}</h3>

      <UiButton variant="secondary" size="sm" @click="addRule">{{ t('rfx.studio.addRule') }}</UiButton>

    </header>



    <p v-if="api.fieldError.value && api.autosaveStatus.value === 'invalid'" class="rule-error">

      {{ api.fieldError.value }}

    </p>



    <p v-if="rulesForTarget.length === 0" class="muted">{{ t('rfx.studio.noRules') }}</p>



    <div v-for="rule in rulesForTarget" :key="rule.id" class="rule-row">

      <div class="rule-row__head">

        <code>{{ rule.rule_code }}</code>

        <UiButton variant="ghost" size="sm" @click="removeRule(rule)">{{ t('rfx.studio.deleteRule') }}</UiButton>

      </div>



      <UiSelect

        :model-value="rule.action"

        :label="t('rfx.studio.ruleAction')"

        :options="actionOptions"

        @update:model-value="onActionChange(rule, $event)"

      />



      <UiSelect

        :model-value="parseCondition(rule).source_question_code ?? ''"

        :label="t('rfx.studio.ruleSource')"

        :options="sourceOptions"

        @update:model-value="onSourceChange(rule, $event)"

      />



      <UiSelect

        :model-value="parseCondition(rule).operator"

        :label="t('rfx.studio.ruleOperator')"

        :options="operatorOptions"

        @update:model-value="onOperatorChange(rule, $event)"

      />



      <UiInput
        :model-value="String(parseCondition(rule).value ?? '')"
        :label="t('rfx.studio.ruleValue')"
        @input="onValueChange(rule, ($event.target as HTMLInputElement).value)"
      />

    </div>

  </UiCard>

</template>



<style scoped>

.rules-editor {

  padding: 1rem;

  display: flex;

  flex-direction: column;

  gap: 0.75rem;

}



.rules-header {

  display: flex;

  justify-content: space-between;

  align-items: center;

  gap: 0.5rem;

}



.rules-header h3 {

  margin: 0;

  font-size: 1rem;

}



.rule-row {

  display: flex;

  flex-direction: column;

  gap: 0.5rem;

  padding: 0.75rem 0;

  border-top: 1px solid var(--color-border);

}



.rule-row__head {

  display: flex;

  justify-content: space-between;

  align-items: center;

}



.rule-error {

  color: #b91c1c;

  font-size: 0.875rem;

  margin: 0;

}



.muted {

  color: var(--color-text-muted);

  font-size: 0.875rem;

}

</style>

