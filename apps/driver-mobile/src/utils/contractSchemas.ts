import {
  DRIVER_DELAY_REASON_CODES,
  DRIVER_EXCEPTION_CATEGORIES,
  type DriverDelayRequest,
  type DriverExceptionRequest,
} from '@/types/driver'

const DELAY_REQUIRED = ['reasonCode', 'idempotencyKey'] as const
const EXCEPTION_REQUIRED = ['category', 'idempotencyKey'] as const

export function validateDelayRequest(body: DriverDelayRequest): string[] {
  const errors: string[] = []
  for (const field of DELAY_REQUIRED) {
    if (!String(body[field] ?? '').trim()) {
      errors.push(`missing ${field}`)
    }
  }
  if (body.reasonCode && !DRIVER_DELAY_REASON_CODES.includes(body.reasonCode)) {
    errors.push(`invalid reasonCode: ${body.reasonCode}`)
  }
  if (body.idempotencyKey && body.idempotencyKey.length > 128) {
    errors.push('idempotencyKey exceeds 128 characters')
  }
  if (body.reasonText && body.reasonText.length > 4000) {
    errors.push('reasonText exceeds 4000 characters')
  }
  return errors
}

export function validateExceptionRequest(body: DriverExceptionRequest): string[] {
  const errors: string[] = []
  for (const field of EXCEPTION_REQUIRED) {
    if (!String(body[field] ?? '').trim()) {
      errors.push(`missing ${field}`)
    }
  }
  if (body.category && !DRIVER_EXCEPTION_CATEGORIES.includes(body.category)) {
    errors.push(`invalid category: ${body.category}`)
  }
  if (body.idempotencyKey && body.idempotencyKey.length > 128) {
    errors.push('idempotencyKey exceeds 128 characters')
  }
  if (body.comment && body.comment.length > 4000) {
    errors.push('comment exceeds 4000 characters')
  }
  return errors
}

export function buildDelayRequestPayload(input: {
  reasonCode: DriverDelayRequest['reasonCode']
  reasonText?: string
  newEta?: string
  idempotencyKey: string
}): DriverDelayRequest {
  return {
    reasonCode: input.reasonCode,
    reasonText: input.reasonText?.trim() || undefined,
    newEta: input.newEta?.trim() || undefined,
    idempotencyKey: input.idempotencyKey,
  }
}

export function buildExceptionRequestPayload(input: {
  category: DriverExceptionRequest['category']
  comment?: string
  idempotencyKey: string
}): DriverExceptionRequest {
  return {
    category: input.category,
    comment: input.comment?.trim() || undefined,
    idempotencyKey: input.idempotencyKey,
  }
}
