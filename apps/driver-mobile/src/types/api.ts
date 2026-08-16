export type SubmissionOutcome =
  | 'REQUEST_NOT_SENT'
  | 'REQUEST_SENT_RESPONSE_UNKNOWN'
  | 'SERVER_REJECTED'
  | 'SUCCESS'

export interface ApiErrorBody {
  error: {
    code: string
    message: string
    details?: Record<string, unknown>
  }
}

export class DriverApiError extends Error {
  readonly code: string
  readonly status: number
  readonly details: Record<string, unknown>

  constructor(status: number, body: ApiErrorBody['error']) {
    super(body.message)
    this.name = 'DriverApiError'
    this.code = body.code
    this.status = status
    this.details = body.details ?? {}
  }
}

export interface RequestResult<T> {
  outcome: SubmissionOutcome
  data?: T
  error?: DriverApiError
}
