import type {
  AutomationListPage,
  AutomationRecommendation,
  AutomationRule,
  ConditionGroup,
  GuardedAction,
  OperationalPlaybook,
  PlaybookExecution,
  PlaybookStep,
} from '~/types/automation'

export function useAutomationApi() {
  const { apiGet, apiPost, apiPatch } = useApi()

  async function listRules(params: Record<string, string | number> = {}) {
    return apiGet<AutomationListPage<AutomationRule>>('/api/v1/control-tower/automation/rules', { query: params })
  }

  async function getRule(ruleId: string) {
    return apiGet<AutomationRule>(`/api/v1/control-tower/automation/rules/${ruleId}`)
  }

  async function createRule(body: Partial<AutomationRule> & { conditions: ConditionGroup }) {
    return apiPost<AutomationRule>('/api/v1/control-tower/automation/rules', body)
  }

  async function updateRule(ruleId: string, body: Record<string, unknown>) {
    return apiPatch<AutomationRule>(`/api/v1/control-tower/automation/rules/${ruleId}`, body)
  }

  async function activateRule(ruleId: string) {
    return apiPost<AutomationRule>(`/api/v1/control-tower/automation/rules/${ruleId}/activate`)
  }

  async function disableRule(ruleId: string) {
    return apiPost<AutomationRule>(`/api/v1/control-tower/automation/rules/${ruleId}/disable`)
  }

  async function retireRule(ruleId: string) {
    return apiPost<AutomationRule>(`/api/v1/control-tower/automation/rules/${ruleId}/retire`)
  }

  async function dryRunRule(ruleId: string, body: Record<string, unknown>) {
    return apiPost<{ matched: boolean; matchedConditions: unknown[]; selectedPlaybookId?: string }>(
      `/api/v1/control-tower/automation/rules/${ruleId}/dry-run`,
      body,
    )
  }

  async function listPlaybooks(params: Record<string, string | number> = {}) {
    return apiGet<AutomationListPage<OperationalPlaybook>>('/api/v1/control-tower/playbooks', { query: params })
  }

  async function getPlaybook(playbookId: string) {
    return apiGet<OperationalPlaybook>(`/api/v1/control-tower/playbooks/${playbookId}`)
  }

  async function createPlaybook(body: { name: string; description?: string; steps: PlaybookStep[] }) {
    return apiPost<OperationalPlaybook>('/api/v1/control-tower/playbooks', body)
  }

  async function updatePlaybook(playbookId: string, body: Record<string, unknown>) {
    return apiPatch<OperationalPlaybook>(`/api/v1/control-tower/playbooks/${playbookId}`, body)
  }

  async function listRecommendations(params: Record<string, string | number> = {}) {
    return apiGet<AutomationListPage<AutomationRecommendation>>('/api/v1/control-tower/automation/recommendations', { query: params })
  }

  async function acceptRecommendation(recommendationId: string) {
    return apiPost<{ recommendation: AutomationRecommendation; execution: PlaybookExecution }>(
      `/api/v1/control-tower/automation/recommendations/${recommendationId}/accept`,
    )
  }

  async function dismissRecommendation(recommendationId: string, reason: string) {
    return apiPost<AutomationRecommendation>(
      `/api/v1/control-tower/automation/recommendations/${recommendationId}/dismiss`,
      { reason },
    )
  }

  async function listPlaybookExecutions(params: Record<string, string | number> = {}) {
    return apiGet<AutomationListPage<PlaybookExecution>>('/api/v1/control-tower/playbook-executions', { query: params })
  }

  async function getExecution(executionId: string) {
    return apiGet<PlaybookExecution>(`/api/v1/control-tower/playbook-executions/${executionId}`)
  }

  async function startExecution(executionId: string) {
    return apiPost<PlaybookExecution>(`/api/v1/control-tower/playbook-executions/${executionId}/start`)
  }

  async function completeExecutionStep(executionId: string, stepId: string) {
    return apiPost<PlaybookExecution>(
      `/api/v1/control-tower/playbook-executions/${executionId}/steps/${stepId}/complete`,
    )
  }

  async function skipExecutionStep(executionId: string, stepId: string, reason?: string) {
    return apiPost<PlaybookExecution>(
      `/api/v1/control-tower/playbook-executions/${executionId}/steps/${stepId}/skip`,
      { reason },
    )
  }

  async function completeExecution(executionId: string) {
    return apiPost<PlaybookExecution>(`/api/v1/control-tower/playbook-executions/${executionId}/complete`)
  }

  async function listGuardedActions(executionId: string) {
    return apiGet<{ items: GuardedAction[] }>(`/api/v1/control-tower/automation/executions/${executionId}/actions`)
  }

  async function approveGuardedAction(executionId: string, actionId: string) {
    return apiPost<GuardedAction>(`/api/v1/control-tower/automation/executions/${executionId}/actions/${actionId}/approve`)
  }

  async function rejectGuardedAction(executionId: string, actionId: string, reason?: string) {
    return apiPost<GuardedAction>(`/api/v1/control-tower/automation/executions/${executionId}/actions/${actionId}/reject`, { reason })
  }

  return {
    listRules,
    getRule,
    createRule,
    updateRule,
    activateRule,
    disableRule,
    retireRule,
    dryRunRule,
    listPlaybooks,
    getPlaybook,
    createPlaybook,
    updatePlaybook,
    listRecommendations,
    acceptRecommendation,
    dismissRecommendation,
    listPlaybookExecutions,
    getExecution,
    startExecution,
    completeExecutionStep,
    skipExecutionStep,
    completeExecution,
    listGuardedActions,
    approveGuardedAction,
    rejectGuardedAction,
  }
}
