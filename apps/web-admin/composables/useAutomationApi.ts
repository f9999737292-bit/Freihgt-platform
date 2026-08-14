import type {
  AutomationListPage,
  AutomationRecommendation,
  AutomationRule,
  ConditionGroup,
  OperationalPlaybook,
  PlaybookExecution,
  PlaybookStep,
} from '~/types/automation'

export function useAutomationApi() {
  const { apiFetch } = useApi()

  async function listRules(params: Record<string, string | number> = {}) {
    return apiFetch<AutomationListPage<AutomationRule>>('/api/v1/control-tower/automation/rules', { query: params })
  }

  async function getRule(ruleId: string) {
    return apiFetch<AutomationRule>(`/api/v1/control-tower/automation/rules/${ruleId}`)
  }

  async function createRule(body: Partial<AutomationRule> & { conditions: ConditionGroup }) {
    return apiFetch<AutomationRule>('/api/v1/control-tower/automation/rules', { method: 'POST', body })
  }

  async function updateRule(ruleId: string, body: Record<string, unknown>) {
    return apiFetch<AutomationRule>(`/api/v1/control-tower/automation/rules/${ruleId}`, { method: 'PATCH', body })
  }

  async function activateRule(ruleId: string) {
    return apiFetch<AutomationRule>(`/api/v1/control-tower/automation/rules/${ruleId}/activate`, { method: 'POST' })
  }

  async function disableRule(ruleId: string) {
    return apiFetch<AutomationRule>(`/api/v1/control-tower/automation/rules/${ruleId}/disable`, { method: 'POST' })
  }

  async function dryRunRule(ruleId: string, body: Record<string, unknown>) {
    return apiFetch<{ matched: boolean; matchedConditions: unknown[]; selectedPlaybookId?: string }>(
      `/api/v1/control-tower/automation/rules/${ruleId}/dry-run`,
      { method: 'POST', body },
    )
  }

  async function listPlaybooks(params: Record<string, string | number> = {}) {
    return apiFetch<AutomationListPage<OperationalPlaybook>>('/api/v1/control-tower/playbooks', { query: params })
  }

  async function getPlaybook(playbookId: string) {
    return apiFetch<OperationalPlaybook>(`/api/v1/control-tower/playbooks/${playbookId}`)
  }

  async function createPlaybook(body: { name: string; description?: string; steps: PlaybookStep[] }) {
    return apiFetch<OperationalPlaybook>('/api/v1/control-tower/playbooks', { method: 'POST', body })
  }

  async function updatePlaybook(playbookId: string, body: Record<string, unknown>) {
    return apiFetch<OperationalPlaybook>(`/api/v1/control-tower/playbooks/${playbookId}`, { method: 'PATCH', body })
  }

  async function listRecommendations(params: Record<string, string | number> = {}) {
    return apiFetch<AutomationListPage<AutomationRecommendation>>('/api/v1/control-tower/automation/recommendations', { query: params })
  }

  async function acceptRecommendation(recommendationId: string) {
    return apiFetch<{ recommendation: AutomationRecommendation; execution: PlaybookExecution }>(
      `/api/v1/control-tower/automation/recommendations/${recommendationId}/accept`,
      { method: 'POST' },
    )
  }

  async function dismissRecommendation(recommendationId: string, reason: string) {
    return apiFetch<AutomationRecommendation>(
      `/api/v1/control-tower/automation/recommendations/${recommendationId}/dismiss`,
      { method: 'POST', body: { reason } },
    )
  }

  async function getExecution(executionId: string) {
    return apiFetch<PlaybookExecution>(`/api/v1/control-tower/playbook-executions/${executionId}`)
  }

  async function startExecution(executionId: string) {
    return apiFetch<PlaybookExecution>(`/api/v1/control-tower/playbook-executions/${executionId}/start`, { method: 'POST' })
  }

  async function completeExecutionStep(executionId: string, stepId: string) {
    return apiFetch<PlaybookExecution>(
      `/api/v1/control-tower/playbook-executions/${executionId}/steps/${stepId}/complete`,
      { method: 'POST' },
    )
  }

  async function skipExecutionStep(executionId: string, stepId: string, reason?: string) {
    return apiFetch<PlaybookExecution>(
      `/api/v1/control-tower/playbook-executions/${executionId}/steps/${stepId}/skip`,
      { method: 'POST', body: { reason } },
    )
  }

  async function completeExecution(executionId: string) {
    return apiFetch<PlaybookExecution>(`/api/v1/control-tower/playbook-executions/${executionId}/complete`, { method: 'POST' })
  }

  return {
    listRules,
    getRule,
    createRule,
    updateRule,
    activateRule,
    disableRule,
    dryRunRule,
    listPlaybooks,
    getPlaybook,
    createPlaybook,
    updatePlaybook,
    listRecommendations,
    acceptRecommendation,
    dismissRecommendation,
    getExecution,
    startExecution,
    completeExecutionStep,
    skipExecutionStep,
    completeExecution,
  }
}
