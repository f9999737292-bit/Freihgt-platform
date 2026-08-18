import type { DriverMilestoneEventType } from '@/types/driver'

const MILESTONE_ACTIONS_BY_STATUS: Record<string, DriverMilestoneEventType[]> = {
  DRIVER_ASSIGNED: ['ARRIVED_AT_PICKUP'],
  PICKUP_SLOT_BOOKED: ['ARRIVED_AT_PICKUP'],
  IN_PICKUP: ['LOADING_STARTED', 'PICKUP_COMPLETED'],
  LOADED: ['DEPARTED_PICKUP'],
  IN_TRANSIT: ['ARRIVED_AT_DELIVERY'],
  ARRIVED_AT_CONSIGNEE: ['UNLOADING_STARTED', 'DELIVERY_COMPLETED'],
  UNLOADING: ['DELIVERY_COMPLETED'],
}

const POD_UPLOAD_STATUSES = new Set([
  'ARRIVED_AT_CONSIGNEE',
  'UNLOADING',
  'DELIVERED',
  'DELIVERY_CONFIRMED',
])

export function allowedMilestoneActions(status: string): DriverMilestoneEventType[] {
  return MILESTONE_ACTIONS_BY_STATUS[status] ?? []
}

export function canUploadPOD(status: string): boolean {
  return POD_UPLOAD_STATUSES.has(status)
}
