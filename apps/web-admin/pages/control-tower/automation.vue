<script setup lang="ts">
import type { AutomationRule, OperationalPlaybook } from '~/types/automation'
import { AUTOMATION_TRIGGER_TYPES } from '~/types/automation'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { t } = useI18n()
const { listRules, listPlaybooks, activateRule, disableRule } = useAutomationApi()

const tab = ref<'rules' | 'playbooks'>('rules')
const rules = ref<AutomationRule[]>([])
const playbooks = ref<OperationalPlaybook[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    if (tab.value === 'rules') {
      const page = await listRules({ limit: 50 })
      rules.value = page.items
    } else {
      const page = await listPlaybooks({ limit: 50 })
      playbooks.value = page.items
    }
  } finally {
    loading.value = false
  }
}

watch(tab, load, { immediate: true })

function triggerLabel(value: string) {
  return t(`controlTower.automation.triggers.${value}`, value)
}
</script>

<template>
  <div class="automation-admin">
    <header>
      <h1>{{ $t('controlTower.automation.title') }}</h1>
      <p>{{ $t('controlTower.automation.subtitle') }}</p>
    </header>

    <nav class="automation-admin__tabs">
      <button type="button" :class="{ active: tab === 'rules' }" @click="tab = 'rules'">
        {{ $t('controlTower.automation.rules') }}
      </button>
      <button type="button" :class="{ active: tab === 'playbooks' }" @click="tab = 'playbooks'">
        {{ $t('controlTower.automation.playbooks') }}
      </button>
    </nav>

    <section v-if="tab === 'rules'">
      <table v-if="rules.length">
        <thead>
          <tr>
            <th>{{ $t('controlTower.automation.ruleName') }}</th>
            <th>{{ $t('controlTower.automation.status') }}</th>
            <th>{{ $t('controlTower.automation.trigger') }}</th>
            <th>{{ $t('controlTower.automation.mode') }}</th>
            <th>{{ $t('controlTower.automation.playbook') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="rule in rules" :key="rule.id">
            <td>{{ rule.name }}</td>
            <td>{{ $t(`controlTower.automation.statuses.${rule.status}`) }}</td>
            <td>{{ triggerLabel(rule.triggerType) }}</td>
            <td>{{ $t(`controlTower.automation.modes.${rule.executionMode}`) }}</td>
            <td>{{ rule.playbookId || '—' }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else>{{ $t('controlTower.automation.noRules') }}</p>
    </section>

    <section v-else>
      <table v-if="playbooks.length">
        <thead>
          <tr>
            <th>{{ $t('controlTower.automation.playbookName') }}</th>
            <th>{{ $t('controlTower.automation.status') }}</th>
            <th>{{ $t('controlTower.automation.version') }}</th>
            <th>{{ $t('controlTower.automation.stepCount') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="pb in playbooks" :key="pb.id">
            <td>{{ pb.name }}</td>
            <td>{{ $t(`controlTower.automation.playbookStatuses.${pb.status}`) }}</td>
            <td>{{ pb.currentVersion }}</td>
            <td>{{ pb.stepCount ?? pb.steps?.length ?? 0 }}</td>
          </tr>
        </tbody>
      </table>
      <p v-else>{{ $t('controlTower.automation.noPlaybooks') }}</p>
    </section>
  </div>
</template>

<style scoped>
.automation-admin__tabs {
  display: flex;
  gap: 8px;
  margin: 16px 0;
}
.automation-admin__tabs button.active {
  font-weight: 600;
}
table {
  width: 100%;
  border-collapse: collapse;
}
th, td {
  text-align: left;
  padding: 8px;
  border-bottom: 1px solid var(--border-subtle, #ddd);
}
</style>
