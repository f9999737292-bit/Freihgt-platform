export type SlotWindowStatus = 'unavailable' | 'available'

export type SlotLifecycleStatus = 'proposed' | 'booked' | 'confirmed' | 'cancelled' | 'completed' | 'missed'

export type SlotArrivalProjection =
  | 'unknown'
  | 'early'
  | 'on_time'
  | 'at_risk'
  | 'projected_miss'
  | 'missed'
  | 'completed'

export interface SlotTargetSummary {
  windowStatus: SlotWindowStatus
  slotStatus?: SlotLifecycleStatus | null
  windowStart?: string | null
  windowEnd?: string | null
  timezone?: string | null
  sourceType?: string | null
  provider?: string | null
  sourceObservedAt?: string | null
  qualityStatus: string
  arrivalProjection: SlotArrivalProjection
  projectedLateBySeconds?: number | null
  earlyBySeconds?: number | null
  marginSeconds?: number | null
  etaRelation?: string | null
}

export interface ShipmentSlotSummary {
  shipmentId: string
  pickup?: SlotTargetSummary | null
  delivery?: SlotTargetSummary | null
}

export interface SlotRevision {
  id: string
  shipmentId: string
  slotType: string
  windowStart: string
  windowEnd: string
  slotStatus: string
  sourceType: string
  sourceObservedAt: string
  receivedAt: string
  qualityStatus: string
  timezone?: string | null
  provider?: string | null
}

export interface SlotHistoryResponse {
  items: SlotRevision[]
  total: number
  limit: number
  offset: number
}

export interface ShipmentSlotQueryContext {
  shipmentStatus?: string | null
  actualPickupAt?: string | null
  actualDeliveryAt?: string | null
  deliveryEtaStatus?: string | null
  deliveryEtaFreshness?: string | null
  deliveryEtaQuality?: string | null
  deliveryEstimatedArrivalAt?: string | null
}
