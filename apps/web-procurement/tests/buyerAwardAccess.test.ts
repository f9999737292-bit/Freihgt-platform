import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  BUYER_AWARD_MANAGE_ROLES,
  hasBuyerAwardManageRole,
} from '~/utils/buyerAwardAccess'

describe('buyer award manage roles (PolicyBuyerManage parity)', () => {
  it('matches gateway buyer manage set', () => {
    expect([...BUYER_AWARD_MANAGE_ROLES].sort()).toEqual([
      'FORWARDER_MANAGER',
      'PLATFORM_ADMIN',
      'PROCUREMENT_MANAGER',
      'SHIPPER_ADMIN',
    ])
  })

  it('allows procurement and shipper admin', () => {
    expect(hasBuyerAwardManageRole(['PROCUREMENT_MANAGER'])).toBe(true)
    expect(hasBuyerAwardManageRole(['SHIPPER_ADMIN'])).toBe(true)
  })

  it('denies SHIPPER_LOGIST and carrier roles', () => {
    expect(hasBuyerAwardManageRole(['SHIPPER_LOGIST'])).toBe(false)
    expect(hasBuyerAwardManageRole(['CARRIER_DISPATCHER'])).toBe(false)
    expect(hasBuyerAwardManageRole(['CARRIER_ADMIN'])).toBe(false)
  })
})

describe('evaluation award UI guard', () => {
  it('uses canBuyerAwardManage for evaluation-award control', () => {
    const source = readFileSync(resolve(import.meta.dirname, '../pages/tenders/[id]/evaluation.vue'), 'utf8')
    expect(source).toContain('canBuyerAwardManage')
    const awardBlock = source.slice(source.indexOf('data-testid="evaluation-award"') - 200, source.indexOf('data-testid="evaluation-award"') + 40)
    expect(awardBlock).toContain('canBuyerAwardManage()')
  })

  it('rechecks buyer manage and selected company before award confirmation', () => {
    const source = readFileSync(resolve(import.meta.dirname, '../pages/tenders/[id]/evaluation.vue'), 'utf8')
    const confirmBlock = source.slice(source.indexOf('async function confirmAward'), source.indexOf('async function confirmConversion'))
    expect(confirmBlock).toContain('canBuyerAwardManage()')
    expect(confirmBlock).toContain('selectedCompanyMatchesAward()')
    expect(confirmBlock).toContain('error.status === 403')
  })

  it('gates shortlist and recalculate with PolicyBuyerManage', () => {
    const source = readFileSync(resolve(import.meta.dirname, '../pages/tenders/[id]/evaluation.vue'), 'utf8')
    expect(source).toContain('data-testid="evaluation-recalculate"')
    expect(source).toContain('data-testid="evaluation-shortlist"')
    const recalculate = source.slice(source.indexOf('data-testid="evaluation-recalculate"') - 80, source.indexOf('data-testid="evaluation-recalculate"'))
    const shortlist = source.slice(source.indexOf('data-testid="evaluation-shortlist"') - 160, source.indexOf('data-testid="evaluation-shortlist"'))
    expect(recalculate).toContain('canBuyerAwardManage()')
    expect(shortlist).toContain('canBuyerAwardManage()')
  })
})
