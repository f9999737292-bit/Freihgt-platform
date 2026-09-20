export const RFX_CREATION_CHANNELS = ['MANUAL', 'TEMPLATE', 'EXCEL', 'ERP'] as const

export type RfxCreationChannel = (typeof RFX_CREATION_CHANNELS)[number]

const KNOWN_CHANNELS = new Set<string>(RFX_CREATION_CHANNELS)

export function isKnownCreationChannel(value: string | null | undefined): value is RfxCreationChannel {
  return typeof value === 'string' && KNOWN_CHANNELS.has(value)
}

export function creationChannelLabelKey(value: string | null | undefined): string {
  if (isKnownCreationChannel(value)) {
    return `tenders.creationChannel.values.${value}`
  }
  return 'tenders.creationChannel.values.unknown'
}

export function humanRfxEventPath(eventId: string) {
  return `/api/v1/rfx-events/${eventId}`
}

export function isErpIntegrationPath(url: string) {
  return url.includes('/integrations/erp/')
}
