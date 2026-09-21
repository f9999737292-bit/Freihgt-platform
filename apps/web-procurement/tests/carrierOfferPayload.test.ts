import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { buildLotOfferLines, parseOfferAmount } from '~/utils/carrierOfferPayload'

describe('carrier offer payload', () => {
  it('parses a filled lot amount as a finite number and empty as null', () => {
    expect(parseOfferAmount('15000')).toBe(15000)
    expect(parseOfferAmount(15000)).toBe(15000)
    expect(parseOfferAmount('')).toBeNull()
    expect(parseOfferAmount(null)).toBeNull()
    expect(parseOfferAmount('abc')).toBeNull()
    expect(parseOfferAmount(-1)).toBeNull()
  })

  it('puts 15000 on the matching rfx_lot_id and keeps currency', () => {
    const built = buildLotOfferLines(
      [{ id: 'lot-1' }],
      { 'lot-1': '15000' },
      'RUB',
    )
    expect(built).toEqual({
      ok: true,
      lines: [{ rfx_lot_id: 'lot-1', amount: 15000, currency_code: 'RUB' }],
    })
  })

  it('blocks PATCH when any lot amount is missing', () => {
    expect(buildLotOfferLines([{ id: 'lot-1' }], {}, 'RUB')).toEqual({
      ok: false,
      missingLotIds: ['lot-1'],
    })
    expect(buildLotOfferLines(
      [{ id: 'lot-1' }, { id: 'lot-2' }],
      { 'lot-1': 15000 },
      'RUB',
    )).toEqual({
      ok: false,
      missingLotIds: ['lot-2'],
    })
  })

  it('requires an amount for every lot before building lines', () => {
    const built = buildLotOfferLines(
      [{ id: 'lot-1' }, { id: 'lot-2' }],
      { 'lot-1': 10, 'lot-2': 20 },
      'EUR',
    )
    expect(built).toEqual({
      ok: true,
      lines: [
        { rfx_lot_id: 'lot-1', amount: 10, currency_code: 'EUR' },
        { rfx_lot_id: 'lot-2', amount: 20, currency_code: 'EUR' },
      ],
    })
  })

  it('carrier save-offer stays on commercial PATCH and does not submit', () => {
    const source = readFileSync(resolve(import.meta.dirname, '../pages/carrier/tenders/[id]/index.vue'), 'utf8')
    expect(source).toContain("import Input from '~/components/ui/Input.vue'")
    expect(source).toContain('data-testid="carrier-save-offer"')
    expect(source).toContain('carrier-offer-lot-')
    expect(source).toContain('buildLotOfferLines')
    expect(source).toContain('offerLotRequired')
    expect(source).toContain('updateResponseCommercial')
    const saveOfferFn = source.match(/async function handleSaveOffer\([\s\S]*?\nasync function /)?.[0] ?? ''
    expect(saveOfferFn).toContain('updateResponseCommercial')
    expect(saveOfferFn).not.toContain('submitResponse')
  })

  it('keeps the commercial save on PATCH and does not mark the response submitted', () => {
    const source = readFileSync(resolve(import.meta.dirname, '../pages/carrier/tenders/[id]/index.vue'), 'utf8')
    const saveOfferFn = source.match(/async function handleSaveOffer\([\s\S]*?\nasync function /)?.[0] ?? ''
    expect(saveOfferFn).toContain('updateResponseCommercial')
    expect(saveOfferFn).toContain('offerSaved')
    expect(saveOfferFn).not.toContain('SUBMITTED')
    expect(saveOfferFn).not.toContain('submitResponse')
    expect(source).toContain('offerLotRequired')
  })
})
