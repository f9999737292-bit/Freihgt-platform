export interface LoginRequest {
  email: string
  password: string
}

export interface AuthUser {
  id: string
  tenant_id: string
  email: string
  full_name: string
  preferred_locale?: string
  status: string
  roles?: string[]
}

export interface LoginResponse {
  access_token: string
  user: AuthUser
}

export interface DriverMe {
  id: string
  displayName: string
  companyId?: string
  status: string
  preferredLocale?: string
  phone?: string
}

export interface DriverShipmentSummary {
  id: string
  shipmentNumber: string
  status: string
  originLocationId?: string
  destinationLocationId?: string
  plannedPickupAt?: string
  plannedDeliveryAt?: string
  vehicleId?: string
}

export interface DriverShipmentDetail extends DriverShipmentSummary {
  transportMode?: string
  version?: number
  actualPickupAt?: string
  actualDeliveryAt?: string
}

export interface DriverShipmentListResponse {
  items: DriverShipmentSummary[]
  total: number
}

export const DRIVER_DELAY_REASON_CODES = [
  'TRAFFIC',
  'VEHICLE_BREAKDOWN',
  'LOADING_DELAY',
  'UNLOADING_DELAY',
  'ROUTE_BLOCKED',
  'CUSTOMER_DELAY',
  'OTHER',
] as const

export type DriverDelayReasonCode = (typeof DRIVER_DELAY_REASON_CODES)[number]

export interface DriverDelayRequest {
  reasonCode: DriverDelayReasonCode
  reasonText?: string
  newEta?: string
  occurredAt?: string
  idempotencyKey: string
}

export interface DriverDelayResponse {
  id: string
  shipmentId: string
  driverId: string
  reasonCode: string
  occurredAt: string
  receivedAt: string
  idempotencyKey: string
  replayed: boolean
  reasonText?: string
  newEta?: string
  outboxEventId?: string
}

export const DRIVER_EXCEPTION_CATEGORIES = [
  'TRAFFIC',
  'VEHICLE_BREAKDOWN',
  'ACCIDENT',
  'LOADING_DELAY',
  'UNLOADING_DELAY',
  'CARGO_ISSUE',
  'DOCUMENT_ISSUE',
  'CUSTOMER_UNAVAILABLE',
  'ROUTE_BLOCKED',
  'OTHER',
] as const

export type DriverExceptionCategory = (typeof DRIVER_EXCEPTION_CATEGORIES)[number]

export interface DriverExceptionRequest {
  category: DriverExceptionCategory
  comment?: string
  occurredAt?: string
  idempotencyKey: string
}

export interface DriverExceptionResponse {
  id: string
  shipmentId: string
  driverId: string
  category: string
  occurredAt: string
  receivedAt: string
  idempotencyKey: string
  replayed: boolean
  comment?: string
  outboxEventId?: string
}

export const DRIVER_MILESTONE_EVENT_TYPES = [
  'ARRIVED_AT_PICKUP',
  'LOADING_STARTED',
  'PICKUP_COMPLETED',
  'DEPARTED_PICKUP',
  'ARRIVED_AT_DELIVERY',
  'UNLOADING_STARTED',
  'DELIVERY_COMPLETED',
] as const

export type DriverMilestoneEventType = (typeof DRIVER_MILESTONE_EVENT_TYPES)[number]

export interface DriverOperationalEventRequest {
  type: DriverMilestoneEventType
  idempotencyKey: string
  occurredAt?: string
}

export interface DriverOperationalEventResponse {
  shipmentId: string
  eventType: string
  shipmentStatus: string
  replayed: boolean
  idempotencyKey?: string
}

export interface DriverPODUploadRequest {
  mimeType: string
  fileName?: string
  idempotencyKey: string
}

export interface DriverPODUploadIntent {
  uploadId: string
  documentId: string
  uploadToken: string
  expiresAt?: string
  mimeType?: string
  maxBytes?: number
}

export interface DriverPODUploadCompleteRequest {
  checksumSha256?: string
}

export interface DriverPODUploadCompleteResponse {
  documentId: string
  uploadId: string
  fileId: string
  status: string
}
