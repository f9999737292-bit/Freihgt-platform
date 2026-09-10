import { createIdempotencyKey } from '~/utils/idempotencyKey'

export type IdempotentOperationStatus =
  | 'idle'
  | 'submitting'
  | 'unknown'
  | 'succeeded'
  | 'failed'

export function stableBodyFingerprint(body: unknown): string {
  return JSON.stringify(body)
}

export class IdempotentOperation<TBody, TResult> {
  readonly scope: string
  key: string
  status: IdempotentOperationStatus = 'idle'
  result: TResult | null = null
  private bodyFingerprint = ''

  constructor(scope: string) {
    this.scope = scope
    this.key = createIdempotencyKey(scope)
  }

  reset(): void {
    this.key = createIdempotencyKey(this.scope)
    this.status = 'idle'
    this.result = null
    this.bodyFingerprint = ''
  }

  isSubmitting(): boolean {
    return this.status === 'submitting'
  }

  async execute(
    body: TBody,
    mutate: (key: string, body: TBody) => Promise<TResult>,
  ): Promise<TResult> {
    const fingerprint = stableBodyFingerprint(body)
    if (this.bodyFingerprint && this.bodyFingerprint !== fingerprint) {
      this.reset()
      this.bodyFingerprint = fingerprint
      this.key = createIdempotencyKey(this.scope)
    } else if (!this.bodyFingerprint) {
      this.bodyFingerprint = fingerprint
    }

    if (this.status === 'succeeded' && this.result != null) {
      return this.result
    }
    if (this.status === 'submitting') {
      throw new Error('IDEMPOTENT_OPERATION_IN_FLIGHT')
    }

    this.status = 'submitting'
    try {
      const response = await mutate(this.key, body)
      this.result = response
      this.status = 'succeeded'
      return response
    } catch (error) {
      if (isUnknownNetworkResult(error)) {
        this.status = 'unknown'
      } else {
        this.status = 'failed'
      }
      throw error
    }
  }
}

export function isUnknownNetworkResult(error: unknown): boolean {
  if (error instanceof TypeError) return true
  if (error instanceof Error) {
    const msg = error.message.toLowerCase()
    return msg.includes('network') || msg.includes('fetch') || msg.includes('timeout')
  }
  return false
}
