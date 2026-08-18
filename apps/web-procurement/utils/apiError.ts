import { ApiError } from '~/utils/apiClient'

export function isNotFoundError(error: unknown): boolean {
  if (error instanceof ApiError) {
    return error.status === 404 || error.code === 'NOT_FOUND'
  }
  return false
}

/** Treat unauthorized cross-company access as "not found" in buyer UI. */
export function shouldShowNotFound(error: unknown): boolean {
  if (error instanceof ApiError) {
    return (
      error.status === 404
      || error.status === 403
      || error.code === 'NOT_FOUND'
      || error.code === 'FORBIDDEN'
    )
  }
  return false
}

export function isApiUnavailableError(error: unknown): boolean {
  if (error instanceof ApiError) {
    return error.status === 0 || error.status >= 500 || error.code === 'SERVICE_UNAVAILABLE'
  }
  return error instanceof TypeError
}
