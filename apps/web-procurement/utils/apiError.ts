import { ApiError } from '~/utils/apiClient'

function errorStatus(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null || !('status' in error)) return undefined
  return typeof error.status === 'number' ? error.status : undefined
}

function errorCode(error: unknown): string | undefined {
  if (typeof error !== 'object' || error === null || !('code' in error)) return undefined
  return typeof error.code === 'string' ? error.code : undefined
}

export function isNotFoundError(error: unknown): boolean {
  return errorStatus(error) === 404 || errorCode(error) === 'NOT_FOUND'
}

/** Treat unauthorized cross-company or RBAC denial as "not found" in buyer UI. */
export function shouldShowNotFound(error: unknown): boolean {
  const status = errorStatus(error)
  const code = errorCode(error)
  return (
    status === 404
    || status === 403
    || code === 'NOT_FOUND'
    || code === 'FORBIDDEN'
  )
}

export function isApiUnavailableError(error: unknown): boolean {
  const status = errorStatus(error)
  const code = errorCode(error)
  if (status === 0 || (status !== undefined && status >= 500) || code === 'SERVICE_UNAVAILABLE') {
    return true
  }
  return error instanceof TypeError && !(error instanceof ApiError)
}
