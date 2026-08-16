import { randomUUID } from '@/utils/uuid'

const OPERATION_PREFIX = 'driver-mobile-op:'

export function createOperationId(kind: 'delay' | 'problem', shipmentId: string): string {
  return `${OPERATION_PREFIX}${kind}:${shipmentId}:${randomUUID()}`
}

export function isValidOperationId(value: string): boolean {
  return value.startsWith(OPERATION_PREFIX) && value.length <= 128
}
