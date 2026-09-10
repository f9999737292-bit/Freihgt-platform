/** Client idempotency key for RFx v3.0E lifecycle mutations (max 128 chars per OpenAPI). */

export function createIdempotencyKey(scope: string): string {
  const raw = `${scope}-${crypto.randomUUID()}`
  return raw.length <= 128 ? raw : raw.slice(0, 128)
}

export function stableIdempotencyKey(scope: string, token: string): string {
  const raw = `${scope}-${token}`
  return raw.length <= 128 ? raw : raw.slice(0, 128)
}
