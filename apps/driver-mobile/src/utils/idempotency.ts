import { randomUUID } from '@/utils/uuid'

const OPERATION_PREFIX = 'driver-mobile-op:'

export function createOperationId(
  kind: 'delay' | 'problem' | 'milestone' | 'pod' | 'arrive' | 'start-service' | 'complete' | 'confirm' | 'fail' | 'disposition',
  entityId: string,
): string {
  return `${OPERATION_PREFIX}${kind}:${entityId}:${randomUUID()}`
}

export function isValidOperationId(value: string): boolean {
  return value.startsWith(OPERATION_PREFIX) && value.length <= 128
}
