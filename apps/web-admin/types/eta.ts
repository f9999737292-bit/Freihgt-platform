export type ETATargetStatus = 'unavailable' | 'available' | 'stale' | 'expired' | 'completed'

export type ETAFreshnessStatus = 'unknown' | 'fresh' | 'stale' | 'expired'

export type ETAQualityStatus = 'unknown' | 'good' | 'degraded' | 'poor'

export type ETAArrivalProjection = 'early' | 'on_time' | 'at_risk' | 'late' | 'unknown'

export type ETASourceType =
  | 'provider_eta'
  | 'carrier_eta'
  | 'driver_eta'
  | 'manual_operator'
  | 'calculated'

export interface ETATargetSummary {
  status: ETATargetStatus
  estimatedArrivalAt?: string | null
  sourceType?: ETASourceType | null
  provider?: string | null
  sourceObservedAt?: string | null
  receivedAt?: string | null
  ageSeconds?: number | null
  freshnessStatus: ETAFreshnessStatus
  qualityStatus: ETAQualityStatus
  qualityReasons?: string[]
  providerConfidence?: number | null
  deliveryLagSeconds?: number | null
  plannedArrivalAt?: string | null
  projectedDeviationSeconds?: number | null
  arrivalProjection: ETAArrivalProjection
}

export interface ShipmentETASummary {
  shipmentId: string
  delivery?: ETATargetSummary | null
  pickup?: ETATargetSummary | null
}

export interface ETAObservation {
  id: string
  shipmentId: string
  targetType: string
  estimatedArrivalAt: string
  sourceType: ETASourceType
  provider?: string | null
  providerEventId?: string | null
  sourceObservedAt: string
  receivedAt: string
  qualityStatus: ETAQualityStatus
  qualityReasons?: string[]
  providerConfidence?: number | null
}

export interface ETAHistoryResponse {
  items: ETAObservation[]
  total: number
  limit: number
  offset: number
}

export interface ShipmentETAContext {
  plannedPickupAt?: string | null
  plannedDeliveryAt?: string | null
  actualPickupAt?: string | null
  actualDeliveryAt?: string | null
  shipmentStatus?: string | null
}
