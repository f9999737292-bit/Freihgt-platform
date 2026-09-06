import type { CarrierValidationErrorItem } from '~/types/carrierResponse'

/** Thrown on HTTP 422 carrier answer validation — distinct from generic ApiError. */
export class CarrierValidationError extends Error {
  readonly status = 422
  readonly code = 'VALIDATION_FAILED'
  readonly errors: CarrierValidationErrorItem[]

  constructor(errors: CarrierValidationErrorItem[]) {
    super('VALIDATION_FAILED')
    this.name = 'CarrierValidationError'
    this.errors = errors
  }
}

/** Thrown on HTTP 409 stale save_version conflict. */
export class CarrierConflictError extends Error {
  readonly status = 409
  readonly code: string
  readonly details: Record<string, unknown>

  constructor(code: string, message: string, details: Record<string, unknown> = {}) {
    super(message)
    this.name = 'CarrierConflictError'
    this.code = code
    this.details = details
  }
}

export function isCarrierValidationError(err: unknown): err is CarrierValidationError {
  return err instanceof CarrierValidationError
}

export function isCarrierConflictError(err: unknown): err is CarrierConflictError {
  return err instanceof CarrierConflictError
}
