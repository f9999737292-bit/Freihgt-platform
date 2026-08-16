export type TrackingStatus =
  | 'not_configured'
  | 'awaiting_data'
  | 'active'
  | 'stale'
  | 'lost'
  | 'ended'

export type TrackingFreshnessStatus = 'unknown' | 'fresh' | 'stale' | 'lost'
export type TrackingQualityStatus = 'unknown' | 'good' | 'degraded' | 'poor'

export interface ShipmentTrackingPosition {
  latitude: number
  longitude: number
  recordedAt: string
  ageSeconds: number
}

export interface ShipmentTrackingSummary {
  shipmentId: string
  trackingStatus: TrackingStatus
  provider?: string
  lastKnownPosition?: ShipmentTrackingPosition | null
  freshness: {
    status: TrackingFreshnessStatus
    ageSeconds?: number
  }
  quality: {
    status: TrackingQualityStatus
    reason?: string
  }
  lastRecordedAt?: string
  lastReceivedAt?: string
  speedKph?: number
  headingDegrees?: number
  deliveryDelaySeconds?: number
}

export interface ShipmentLocationHistoryItem {
  id: string
  shipmentId: string
  provider: string
  providerDeviceId: string
  providerEventId?: string
  latitude: number
  longitude: number
  recordedAt: string
  receivedAt: string
  sourceType: string
  speedKph?: number
  headingDegrees?: number
  accuracyMeters?: number
  quality: {
    status: TrackingQualityStatus
    reason?: string
  }
}

export interface ShipmentLocationHistoryResponse {
  items: ShipmentLocationHistoryItem[]
  total: number
  limit: number
  offset: number
}
