import { ApiError } from '~/utils/apiClient'

export function isCarrierQuestionnaireBindingRequired(error: unknown): boolean {
  if (!(error instanceof ApiError)) return false
  if (error.status !== 422) return false
  if (typeof error.code !== 'string' || error.code.length === 0) return false
  if (error.code === 'VALIDATION_FAILED') return false
  return error.details?.field === 'rfx_version_id'
}

export function shouldStartCarrierResponseOnLoadError(
  error: unknown,
  options: { startIfMissing: boolean; deadlineExpired: boolean },
): boolean {
  if (!options.startIfMissing || options.deadlineExpired) return false
  if (error instanceof ApiError && error.status === 404) return true
  return isCarrierQuestionnaireBindingRequired(error)
}

export async function resolveCarrierQuestionnaireWorkspace<T>(input: {
  get: () => Promise<T>
  start: () => Promise<T>
  startIfMissing: boolean
  deadlineExpired: boolean
  isNotStarted: (workspace: T) => boolean
}): Promise<{ workspace: T; started: boolean }> {
  let started = false
  let workspace: T
  try {
    workspace = await input.get()
  } catch (error) {
    if (!shouldStartCarrierResponseOnLoadError(error, input)) throw error
    workspace = await input.start()
    started = true
  }
  if (!started && input.isNotStarted(workspace) && input.startIfMissing && !input.deadlineExpired) {
    workspace = await input.start()
    started = true
  }
  return { workspace, started }
}
