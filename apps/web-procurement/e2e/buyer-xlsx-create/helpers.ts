import { expect, type Page } from '@playwright/test'

export function requireCreateEnv(name: string): string {
  const value = (process.env[name] || '').trim()
  if (!value) {
    throw new Error(`${name} is required for the buyer XLSX create browser gate`)
  }
  return value
}

export const webURL = requireCreateEnv('BROWSER_E2E_WEB_URL')
export const gatewayURL = requireCreateEnv('BROWSER_E2E_GATEWAY_URL')
export const buyerJwt = requireCreateEnv('BROWSER_E2E_JWT')
export const tenantId = requireCreateEnv('BROWSER_E2E_TENANT_ID')
export const buyerCompanyId = requireCreateEnv('BROWSER_E2E_BUYER_COMPANY_ID')
export const buyerUserId = requireCreateEnv('BROWSER_E2E_USER_ID')
export const workbookPath = requireCreateEnv('BROWSER_E2E_WORKBOOK_PATH')
export const flagOffWebURL = requireCreateEnv('BROWSER_E2E_FLAG_OFF_WEB_URL')
export const flagOffGatewayURL = requireCreateEnv('BROWSER_E2E_FLAG_OFF_GATEWAY_URL')
export const logistJwt = requireCreateEnv('BROWSER_E2E_LOGIST_JWT')
export const logistUserId = requireCreateEnv('BROWSER_E2E_LOGIST_USER_ID')
export const carrierJwt = requireCreateEnv('BROWSER_E2E_CARRIER_JWT')
export const carrierUserId = requireCreateEnv('BROWSER_E2E_CARRIER_USER_ID')
export const carrierCompanyId = requireCreateEnv('BROWSER_E2E_CARRIER_COMPANY_ID')
export const foreignJwt = requireCreateEnv('BROWSER_E2E_FOREIGN_JWT')
export const foreignTenantId = requireCreateEnv('BROWSER_E2E_FOREIGN_TENANT_ID')
export const foreignUserId = requireCreateEnv('BROWSER_E2E_FOREIGN_USER_ID')
export const otherCompanyId = requireCreateEnv('BROWSER_E2E_OTHER_COMPANY_ID')
export const flagOffRfxNumber = requireCreateEnv('BROWSER_E2E_FLAG_OFF_RFX_NUMBER')

export interface CreateNetworkProbe {
  request: string[]
  response: string[]
  commitKeys: string[]
  previewBodies: string[]
}

export function attachCreateNetworkProbe(page: Page): CreateNetworkProbe {
  const probe: CreateNetworkProbe = {
    request: [],
    response: [],
    commitKeys: [],
    previewBodies: [],
  }
  page.on('request', (req) => {
    const url = req.url()
    if (!url.includes('/api/v1/')) return
    probe.request.push(`${req.method()} ${url}`)
    if (req.method() === 'POST' && url.includes('/xlsx-create/commit')) {
      probe.commitKeys.push(req.headers()['idempotency-key'] || '')
    }
  })
  page.on('response', (resp) => {
    const url = resp.url()
    if (!url.includes('/api/v1/')) return
    probe.response.push(`${resp.request().method()} ${resp.status()} ${url}`)
  })
  return probe
}

export function formatCreateNetworkProbe(probe: CreateNetworkProbe): string {
  return [
    `request=${JSON.stringify(probe.request)}`,
    `response=${JSON.stringify(probe.response)}`,
    `commitKeys=${JSON.stringify(probe.commitKeys)}`,
  ].join('\n')
}

export function authHeaders(token: string, company: string, tenant = tenantId): Record<string, string> {
  return {
    Authorization: `Bearer ${token}`,
    'X-Company-ID': company,
    'X-Tenant-ID': tenant,
  }
}

export async function seedCreateSession(
  page: Page,
  input: {
    token: string
    user: string
    company: string
    tenant?: string
    roles: string[]
    excelFlag?: boolean
  },
) {
  await page.addInitScript((session) => {
    localStorage.setItem(
      'freight_procurement_session',
      JSON.stringify({
        token: session.token,
        user: {
          id: session.user,
          tenant_id: session.tenant,
          email: 'buyer-xlsx-create@freight.test',
          full_name: 'Buyer XLSX Create',
          preferred_locale: 'en-US',
          status: 'ACTIVE',
          roles: session.roles,
        },
      }),
    )
    localStorage.setItem('freight_procurement_tenant_id', session.tenant)
    localStorage.setItem('freight_procurement_company_id', session.company)
    if (session.excelFlag) {
      localStorage.setItem('freight_procurement_rfx_excel_exchange', 'true')
    } else {
      localStorage.removeItem('freight_procurement_rfx_excel_exchange')
    }
    document.cookie = 'freight_procurement_locale=en-US; path=/'
  }, {
    token: input.token,
    user: input.user,
    company: input.company,
    tenant: input.tenant || tenantId,
    roles: input.roles,
    excelFlag: input.excelFlag !== false,
  })
}

export async function openTendersList(page: Page) {
  await seedCreateSession(page, {
    token: buyerJwt,
    user: buyerUserId,
    company: buyerCompanyId,
    roles: ['PROCUREMENT_MANAGER'],
  })
  await page.goto(`${webURL}/tenders`, { waitUntil: 'domcontentloaded' })
  await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible({ timeout: 30_000 })
}

export async function fillCreateMetadata(page: Page, rfxNumber: string, title: string) {
  await expect(page.getByTestId('buyer-xlsx-create-owner')).not.toHaveValue('', { timeout: 15_000 })
  await page.getByTestId('buyer-xlsx-create-rfx-number').fill(rfxNumber)
  await page.getByTestId('buyer-xlsx-create-title').fill(title)
  await page.getByTestId('buyer-xlsx-create-type').selectOption('SPOT_RFQ')
  await page.getByTestId('buyer-xlsx-create-category').selectOption('FREIGHT')
}

export async function expectNoProductLeaks(page: Page, probe: CreateNetworkProbe) {
  const leaked = probe.request.filter((line) => (
    /\/integrations\/erp/i.test(line)
    || /\/publish\b/i.test(line)
    || /\/submit\b/i.test(line)
    || (/\/participants\b/.test(line) && line.startsWith('POST '))
  ))
  expect(leaked, formatCreateNetworkProbe(probe)).toEqual([])
  await expect(page.locator('body')).not.toContainText('canonical_payload_hash')
  await expect(page.locator('body')).not.toContainText('rfx.buyer_xlsx')
}

export function assertPreviewMultipart(form: { name: string; value: string }[] | undefined) {
  const names = (form || []).map((part) => part.name)
  expect(names).toContain('file')
  expect(names).toContain('owner_company_id')
  expect(names).toContain('rfx_number')
  expect(names).toContain('title')
  expect(names).toContain('rfx_type')
  expect(names).toContain('category')
  expect(names).not.toContain('tenant_id')
}
