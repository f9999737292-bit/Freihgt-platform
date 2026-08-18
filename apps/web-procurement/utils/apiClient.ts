export class ApiError extends Error {
  code: string
  details: Record<string, unknown>
  status: number

  constructor(
    status: number,
    body: { code: string; message: string; details?: Record<string, unknown> },
  ) {
    super(body.message)
    this.name = 'ApiError'
    this.code = body.code
    this.details = body.details ?? {}
    this.status = status
  }
}

export class TenantRequiredError extends Error {
  constructor(message = 'Tenant ID is required') {
    super(message)
    this.name = 'TenantRequiredError'
  }
}
