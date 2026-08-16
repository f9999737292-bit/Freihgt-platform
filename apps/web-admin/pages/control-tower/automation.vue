<script setup lang="ts">
import type { AutomationExecutionMode, AutomationRule, ConditionGroup, OperationalPlaybook, PlaybookExecution, PlaybookStep } from '~/types/automation'
import { isApiUnavailableError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth', layout: 'default' })

const { t } = useI18n()
const { pushToast } = useToast()
const { isPlatformAdmin } = usePermissions()
const {
  listRules,
  listPlaybooks,
  listPlaybookExecutions,
  createRule,
  updateRule,
  activateRule,
  disableRule,
  retireRule,
  createPlaybook,
  updatePlaybook,
} = useAutomationApi()

const tab = ref<'rules' | 'playbooks' | 'executions'>('rules')
const rules = ref<AutomationRule[]>([])
const playbooks = ref<OperationalPlaybook[]>([])
const executions = ref<PlaybookExecution[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const actionLoadingId = ref<string | null>(null)

const ruleEditorOpen = ref(false)
const editingRule = ref<AutomationRule | null>(null)
const ruleSaving = ref(false)

const playbookEditorOpen = ref(false)
const editingPlaybook = ref<OperationalPlaybook | null>(null)
const playbookSaving = ref(false)

const canManage = computed(() => isPlatformAdmin())

async function loadRules() {
  const page = await listRules({ limit: 50 })
  rules.value = page.items
}

async function loadPlaybooks() {
  const page = await listPlaybooks({ limit: 50 })
  playbooks.value = page.items
}

async function loadExecutions() {
  const page = await listPlaybookExecutions({ limit: 50 })
  executions.value = page.items
}

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    if (tab.value === 'rules') {
      await loadRules()
    } else if (tab.value === 'playbooks') {
      await loadPlaybooks()
    } else {
      await loadExecutions()
    }
  } catch (error) {
    if (tab.value === 'rules') rules.value = []
    if (tab.value === 'playbooks') playbooks.value = []
    if (tab.value === 'executions') executions.value = []
    loadFailed.value = isApiUnavailableError(error)
    if (!loadFailed.value) {
      pushToast('error', error instanceof Error ? error.message : t('common.error'))
    }
  } finally {
    loading.value = false
  }
}

watch(tab, load, { immediate: true })

function triggerLabel(value: string) {
  return t(`controlTower.automation.triggers.${value}`, value)
}

function playbookName(playbookId?: string) {
  if (!playbookId) return '—'
  return playbooks.value.find((pb) => pb.id === playbookId)?.name ?? playbookId
}

function openCreateRule() {
  if (!canManage.value) {
    pushToast('error', t('common.insufficientPermission'))
    return
  }
  editingRule.value = null
  ruleEditorOpen.value = true
}

function openEditRule(rule: AutomationRule) {
  if (!canManage.value) {
    pushToast('error', t('common.insufficientPermission'))
    return
  }
  editingRule.value = rule
  ruleEditorOpen.value = true
}

type RuleSavePayload = {
  name: string
  description?: string
  triggerType: string
  executionMode: AutomationExecutionMode
  playbookId?: string
  priority: number
  conditions: ConditionGroup
}

async function onSaveRule(payload: RuleSavePayload) {
  if (!canManage.value) return
  ruleSaving.value = true
  try {
    if (editingRule.value) {
      await updateRule(editingRule.value.id, payload)
      pushToast('success', t('controlTower.automation.ruleUpdated', 'Rule updated'))
    } else {
      await createRule(payload)
      pushToast('success', t('controlTower.automation.ruleCreated', 'Rule created'))
    }
    ruleEditorOpen.value = false
    editingRule.value = null
    await loadRules()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    ruleSaving.value = false
  }
}

async function onRuleAction(ruleId: string, action: 'activate' | 'disable' | 'retire') {
  if (!canManage.value) {
    pushToast('error', t('common.insufficientPermission'))
    return
  }
  actionLoadingId.value = ruleId
  try {
    if (action === 'activate') await activateRule(ruleId)
    else if (action === 'disable') await disableRule(ruleId)
    else await retireRule(ruleId)
    await loadRules()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    actionLoadingId.value = null
  }
}

function openCreatePlaybook() {
  if (!canManage.value) {
    pushToast('error', t('common.insufficientPermission'))
    return
  }
  editingPlaybook.value = null
  playbookEditorOpen.value = true
}

function openEditPlaybook(playbook: OperationalPlaybook) {
  if (!canManage.value) {
    pushToast('error', t('common.insufficientPermission'))
    return
  }
  editingPlaybook.value = playbook
  playbookEditorOpen.value = true
}

async function onSavePlaybook(payload: { name: string; description?: string; steps: PlaybookStep[] }) {
  if (!canManage.value) return
  playbookSaving.value = true
  try {
    if (editingPlaybook.value) {
      await updatePlaybook(editingPlaybook.value.id, payload)
      pushToast('success', t('controlTower.automation.playbookUpdated', 'Playbook updated'))
    } else {
      await createPlaybook(payload)
      pushToast('success', t('controlTower.automation.playbookCreated', 'Playbook created'))
    }
    playbookEditorOpen.value = false
    editingPlaybook.value = null
    await loadPlaybooks()
  } catch (error) {
    pushToast('error', error instanceof Error ? error.message : t('common.error'))
  } finally {
    playbookSaving.value = false
  }
}

onMounted(async () => {
  try {
    await loadPlaybooks()
  } catch {
    // Playbook names are optional for the rules table.
  }
})
</script>

<template>
  <div class="automation-admin page-stack">
    <UiPageHeader
      :title="$t('controlTower.automation.title')"
      :subtitle="$t('controlTower.automation.subtitle')"
    >
      <template v-if="canManage && tab !== 'executions'" #actions>
        <UiButton v-if="tab === 'rules'" @click="openCreateRule">
          {{ $t('common.create') }}
        </UiButton>
        <UiButton v-else-if="tab === 'playbooks'" @click="openCreatePlaybook">
          {{ $t('common.create') }}
        </UiButton>
      </template>
    </UiPageHeader>

    <nav class="automation-admin__tabs" aria-label="Automation sections">
      <button type="button" :class="{ active: tab === 'rules' }" @click="tab = 'rules'">
        {{ $t('controlTower.automation.rules') }}
      </button>
      <button type="button" :class="{ active: tab === 'playbooks' }" @click="tab = 'playbooks'">
        {{ $t('controlTower.automation.playbooks') }}
      </button>
      <button type="button" :class="{ active: tab === 'executions' }" @click="tab = 'executions'">
        {{ $t('controlTower.automation.executions', 'Executions') }}
      </button>
    </nav>

    <div v-if="loadFailed" class="automation-admin__error">
      <p>{{ $t('common.apiUnavailable') }}</p>
      <p class="automation-admin__hint">{{ $t('common.apiUnavailableHint') }}</p>
      <UiButton variant="secondary" size="sm" @click="load">
        {{ $t('common.refresh') }}
      </UiButton>
    </div>

    <section v-else-if="tab === 'rules'">
      <UiTable
        :loading="loading"
        :columns="[
          $t('controlTower.automation.ruleName'),
          $t('controlTower.automation.status'),
          $t('controlTower.automation.trigger'),
          $t('controlTower.automation.mode'),
          $t('controlTower.automation.playbook'),
          ...(canManage ? [$t('common.actions')] : []),
        ]"
      >
        <tr v-if="!loading && !rules.length">
          <td :colspan="canManage ? 6 : 5">
            <UiEmptyState :title="$t('controlTower.automation.noRules')" />
          </td>
        </tr>
        <template v-else-if="!loading">
          <tr v-for="rule in rules" :key="rule.id">
            <td>{{ rule.name }}</td>
            <td>{{ $t(`controlTower.automation.statuses.${rule.status}`) }}</td>
            <td>{{ triggerLabel(rule.triggerType) }}</td>
            <td>{{ $t(`controlTower.automation.modes.${rule.executionMode}`) }}</td>
            <td>{{ playbookName(rule.playbookId) }}</td>
            <td v-if="canManage" class="automation-admin__actions">
              <UiButton variant="ghost" size="sm" @click="openEditRule(rule)">
                {{ $t('lowCode.edit') }}
              </UiButton>
              <UiButton
                v-if="rule.status === 'draft' || rule.status === 'disabled'"
                variant="secondary"
                size="sm"
                :loading="actionLoadingId === rule.id"
                @click="onRuleAction(rule.id, 'activate')"
              >
                {{ $t('controlTower.automation.activate', 'Activate') }}
              </UiButton>
              <UiButton
                v-if="rule.status === 'active'"
                variant="secondary"
                size="sm"
                :loading="actionLoadingId === rule.id"
                @click="onRuleAction(rule.id, 'disable')"
              >
                {{ $t('controlTower.automation.disable', 'Disable') }}
              </UiButton>
              <UiButton
                v-if="rule.status !== 'retired'"
                variant="ghost"
                size="sm"
                :loading="actionLoadingId === rule.id"
                @click="onRuleAction(rule.id, 'retire')"
              >
                {{ $t('controlTower.automation.statuses.retired') }}
              </UiButton>
            </td>
          </tr>
        </template>
      </UiTable>
    </section>

    <section v-else-if="tab === 'playbooks'">
      <UiTable
        :loading="loading"
        :columns="[
          $t('controlTower.automation.playbookName'),
          $t('controlTower.automation.status'),
          $t('controlTower.automation.version'),
          $t('controlTower.automation.stepCount'),
          ...(canManage ? [$t('common.actions')] : []),
        ]"
      >
        <tr v-if="!loading && !playbooks.length">
          <td :colspan="canManage ? 5 : 4">
            <UiEmptyState :title="$t('controlTower.automation.noPlaybooks')" />
          </td>
        </tr>
        <template v-else-if="!loading">
          <tr v-for="pb in playbooks" :key="pb.id">
            <td>{{ pb.name }}</td>
            <td>{{ $t(`controlTower.automation.playbookStatuses.${pb.status}`) }}</td>
            <td>{{ pb.currentVersion }}</td>
            <td>{{ pb.stepCount ?? pb.steps?.length ?? 0 }}</td>
            <td v-if="canManage">
              <UiButton variant="ghost" size="sm" @click="openEditPlaybook(pb)">
                {{ $t('lowCode.edit') }}
              </UiButton>
            </td>
          </tr>
        </template>
      </UiTable>
    </section>

    <section v-else>
      <ControlTowerAutomationExecutionList
        :executions="executions"
        :loading="loading"
        :load-failed="loadFailed"
        @refresh="load"
      />
    </section>

    <ControlTowerAutomationRuleEditor
      :open="ruleEditorOpen"
      :rule="editingRule"
      :playbooks="playbooks"
      :saving="ruleSaving"
      @close="ruleEditorOpen = false"
      @save="onSaveRule"
    />

    <ControlTowerAutomationPlaybookEditor
      :open="playbookEditorOpen"
      :playbook="editingPlaybook"
      :saving="playbookSaving"
      @close="playbookEditorOpen = false"
      @save="onSavePlaybook"
    />
  </div>
</template>

<style scoped>
.automation-admin__tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 1rem;
}

.automation-admin__tabs button {
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  cursor: pointer;
}

.automation-admin__tabs button.active {
  font-weight: 600;
  border-color: var(--color-primary);
}

.automation-admin__error {
  padding: 1.5rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-surface);
}

.automation-admin__hint {
  color: var(--color-text-muted);
  font-size: 0.875rem;
}

.automation-admin__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
</style>
