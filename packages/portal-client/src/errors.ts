export class PortalClientError extends Error {
  readonly status: number
  readonly code: string
  readonly body: unknown

  constructor(message: string, status = 0, code = 'PORTAL_CLIENT', body?: unknown) {
    super(message)
    this.name = 'PortalClientError'
    this.status = status
    this.code = code
    this.body = body
  }
}

export function isPortalClientError(error: unknown): error is PortalClientError {
  return error instanceof PortalClientError
}
