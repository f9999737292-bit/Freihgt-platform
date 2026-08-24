import { expect } from '@playwright/test'

export function decimalAmountPattern(amount: string): RegExp {
  const normalized = amount.trim()
  const [integerPart, fractionPart = '00'] = normalized.split('.')
  const digits = integerPart.replace(/\D/g, '')
  if (!digits) {
    throw new Error(`invalid decimal amount: ${amount}`)
  }

  const lastThree = digits.slice(-3)
  const prefix = digits.slice(0, -3)
  let groupedPrefix = prefix
  if (prefix.length > 0) {
    groupedPrefix = prefix.replace(/\B(?=(\d{3})+(?!\d))/g, '[,\\s\\u00a0]?')
  }

  const integerPattern = prefix.length > 0
    ? `${groupedPrefix}[,\\s\\u00a0]?${lastThree}`
    : lastThree

  const fractionPattern = fractionPart.replace(/0+$/, '') || '0'
  const optionalFraction = fractionPart === '00'
    ? '(?:[,\\.][0-9]{2})?'
    : `(?:[,\\.]${fractionPattern})?`

  return new RegExp(`${integerPattern}${optionalFraction}`)
}

export function expectDecimalClose(actual: string | number | null | undefined, expected: string) {
  const actualNumber = Number(String(actual ?? '').replace(/,/g, ''))
  const expectedNumber = Number(expected)
  expect(Number.isFinite(actualNumber)).toBeTruthy()
  expect(actualNumber).toBeCloseTo(expectedNumber, 2)
}

export async function expectShellReady(page: import('@playwright/test').Page) {
  await page.waitForSelector('.freight-cost-subnav', { timeout: 60_000 })
  await expect(page.locator('.freight-cost-subnav__link').first()).toBeVisible()
}

export async function expectWorkspaceContent(page: import('@playwright/test').Page) {
  await expect(page.locator('.freight-cost-shell')).not.toContainText(/Loading|Загрузка/i, { timeout: 60_000 })
  await expect(
    page.locator('.intelligence-overview, .ui-table-wrap, .kpi-card__value').first(),
  ).toBeVisible({ timeout: 60_000 })
}

export async function expectIntelligenceTableReady(page: import('@playwright/test').Page) {
  await expect(page.locator('.freight-cost-shell')).not.toContainText(/Loading|Загрузка/i, { timeout: 60_000 })
  await expect(page.locator('.ui-table-wrap tbody tr').first()).toBeVisible({ timeout: 60_000 })
}

export async function expectRenderedFixtureText(
  page: import('@playwright/test').Page,
  pattern: string | RegExp,
) {
  await expectIntelligenceTableReady(page)
  await expect(page.getByText(pattern)).toBeVisible({ timeout: 60_000 })
}
