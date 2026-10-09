import { isPortalClientError } from '@freight-platform/portal-client'

export type PortalViewKind =
  | 'loading'
  | 'ready'
  | 'empty'
  | 'company_gate'
  | 'forbidden'
  | 'not_found'
  | 'unavailable'
  | 'unauthorized'
  | 'rejected'

export function viewKindFromError(error: unknown): Exclude<PortalViewKind, 'loading' | 'ready' | 'empty' | 'company_gate'> {
  if (!isPortalClientError(error)) return 'unavailable'
  if (error.status === 401 || error.code === 'UNAUTHORIZED') return 'unauthorized'
  if (error.status === 403 || error.code === 'FORBIDDEN' || error.code === 'COMPANY_SPOOF') return 'forbidden'
  if (error.status === 404 || error.code === 'NOT_FOUND') return 'not_found'
  if (error.status === 0 || error.status >= 500 || error.code === 'UNAVAILABLE') return 'unavailable'
  return 'rejected'
}

export function isMissingRecord(error: unknown): boolean {
  return viewKindFromError(error) === 'not_found'
}
