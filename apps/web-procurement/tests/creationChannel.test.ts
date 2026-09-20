import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  creationChannelLabelKey,
  humanRfxEventPath,
  isErpIntegrationPath,
  isKnownCreationChannel,
  RFX_CREATION_CHANNELS,
} from '~/utils/creationChannel'

function i18nCreationChannel(locale: string) {
  const raw = JSON.parse(
    readFileSync(resolve(__dirname, `../i18n/${locale}/tenders.json`), 'utf8'),
  ) as {
    tenders: {
      creationChannel: { label: string; values: Record<string, string> }
    }
  }
  return raw.tenders.creationChannel
}

describe('F4 human provenance creation_channel', () => {
  it('uses the existing human GET path only', () => {
    const eventId = '11111111-1111-4111-8111-111111111111'
    expect(humanRfxEventPath(eventId)).toBe(`/api/v1/rfx-events/${eventId}`)
    expect(humanRfxEventPath(eventId)).not.toContain('/integrations/erp/')
    expect(humanRfxEventPath(eventId)).not.toContain('external_link')
  })

  it('accepts only persisted channel values', () => {
    expect(RFX_CREATION_CHANNELS).toEqual(['MANUAL', 'TEMPLATE', 'EXCEL', 'ERP'])
    for (const channel of RFX_CREATION_CHANNELS) {
      expect(isKnownCreationChannel(channel)).toBe(true)
      expect(creationChannelLabelKey(channel)).toBe(`tenders.creationChannel.values.${channel}`)
    }
  })

  it('never surfaces an unknown raw code', () => {
    expect(isKnownCreationChannel('SAP')).toBe(false)
    expect(isKnownCreationChannel('')).toBe(false)
    expect(isKnownCreationChannel(null)).toBe(false)
    expect(creationChannelLabelKey('SAP')).toBe('tenders.creationChannel.values.unknown')
    expect(creationChannelLabelKey('SAP')).not.toContain('SAP')
    expect(creationChannelLabelKey(undefined)).toBe('tenders.creationChannel.values.unknown')
  })

  it('has RU/EN/ZH labels without raw codes', () => {
    for (const locale of ['en-US', 'ru-RU', 'zh-CN']) {
      const copy = i18nCreationChannel(locale)
      expect(copy.label.trim()).not.toBe('')
      expect(copy.values.unknown.trim()).not.toBe('')
      expect(copy.values.unknown).not.toBe('SAP')
      for (const channel of RFX_CREATION_CHANNELS) {
        expect(copy.values[channel].trim()).not.toBe('')
        expect(copy.values[channel]).not.toBe(channel)
      }
    }
  })

  it('does not treat human GET as an ERP integration call', () => {
    expect(isErpIntegrationPath('/api/v1/rfx-events/11111111-1111-4111-8111-111111111111')).toBe(false)
    expect(isErpIntegrationPath('/api/v1/integrations/erp/rfx-events/11111111-1111-4111-8111-111111111111')).toBe(true)
  })

  it('keeps buyer GET types free of external_link', () => {
    const types = readFileSync(resolve(__dirname, '../types/rfx.ts'), 'utf8')
    const page = readFileSync(resolve(__dirname, '../pages/tenders/[id]/index.vue'), 'utf8')
    const api = readFileSync(resolve(__dirname, '../composables/useRfxApi.ts'), 'utf8')
    expect(types).toContain('creation_channel')
    expect(types).not.toContain('external_link')
    expect(page).not.toContain('external_link')
    expect(page).not.toContain('/integrations/erp/')
    expect(api).toContain('/api/v1/rfx-events/${id}')
    expect(api).not.toContain('/integrations/erp/')
    expect(api).not.toContain('external_link')
  })
})
