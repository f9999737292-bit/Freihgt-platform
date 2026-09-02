/**
 * VIS-005 local E2E — event history navigation via real UI clicks.
 */
import { chromium } from 'playwright';
import fs from 'node:fs';
import path from 'node:path';

const BASE = process.env.PILOT_UI_BASE_URL || 'http://localhost:3001';
const OUT = path.resolve('../../artifacts/bintrans-visual-walkthrough-vis005');
const SHIPMENT_ID = '0b9fe8d5-d20e-4a81-b591-c0df9812fc95';
const EXPECTED = process.env.EXPECTED_EVENTS_URL || `/shipments/${SHIPMENT_ID}/events`;
const tenant = process.env.TENANT_W2 || '';
const email = process.env.BUYER_EMAIL || '';
const password = process.env.BUYER_PASSWORD || '';

function set(k, v) {
  console.log(`${k}=${v}`);
}

async function login(page) {
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' });
  await page.waitForSelector('#login-tenant-id');
  await page.locator('form.login-form').evaluate((form) => {
    form.setAttribute('method', 'post');
    form.addEventListener('submit', (e) => e.preventDefault(), { capture: true });
  });
  await page.locator('#login-tenant-id').fill(tenant);
  await page.locator('#login-email').fill(email);
  await page.locator('#login-password').fill(password);
  delete process.env.BUYER_PASSWORD;
  const loginWait = page.waitForResponse(
    (r) => r.request().method() === 'POST' && r.url().includes('/api/v1/auth/login'),
    { timeout: 35000 },
  );
  await page.locator('form.login-form button[type="submit"]').click();
  if ((await loginWait).status() !== 200) throw new Error('login failed');
  await page.waitForFunction(() => !window.location.pathname.includes('/login'), { timeout: 20000 });
  set('AUTHENTICATED_UI', 'PASS');
}

async function clickSidebar(page, href, label) {
  const link = page.locator(`aside.sidebar a.sidebar__link[href="${href}"]`).first();
  if ((await link.count()) > 0) await link.click();
  else await page.locator('aside.sidebar a.sidebar__link').filter({ hasText: label }).first().click();
  await page.waitForURL((u) => u.pathname.includes(href), { timeout: 15000 });
  await page.waitForLoadState('networkidle').catch(() => {});
  await page.waitForTimeout(1000);
}

async function main() {
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch({ channel: 'chrome', headless: true });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  await login(page);

  await clickSidebar(page, '/control-tower', 'Центр управления');
  set('R31_SHIPMENT_VISIBLE', (await page.locator(`a[href*="${SHIPMENT_ID}"]`).count()) > 0 ? 'YES' : 'NO');
  await page.locator(`a[href*="${SHIPMENT_ID}"]`).first().click();
  await page.waitForURL((u) => u.pathname.includes(`/shipments/${SHIPMENT_ID}`), { timeout: 15000 });
  const detail = await page.locator('body').innerText();
  set('R31_SHIPMENT_STATUS', /IN_TRANSIT/i.test(detail) ? 'IN_TRANSIT' : 'NOT_SHOWN');

  const urlBefore = page.url();
  const btn = page.getByRole('button', { name: 'История событий' });
  set('EVENT_HISTORY_CONTROL_FOUND', (await btn.count()) > 0 ? 'YES' : 'NO');
  set('EVENT_HISTORY_CONTROL_ROLE', 'button');
  await btn.click();
  await page.waitForURL((u) => u.pathname.includes('/events'), { timeout: 15000 });
  const urlAfter = page.url();
  set('URL_BEFORE', urlBefore);
  set('URL_AFTER', urlAfter);
  set('EVENT_HISTORY_CLICK_NAVIGATED', urlAfter.includes('/events') ? 'YES' : 'NO');
  set('EVENT_HISTORY_URL_MATCH', urlAfter.includes(`/shipments/${SHIPMENT_ID}/events`) ? 'YES' : 'NO');

  await page.waitForSelector('h1, .ui-page-header', { timeout: 10000 }).catch(() => {});
  await page.waitForLoadState('networkidle').catch(() => {});
  await page.waitForTimeout(2500);
  const body = await page.locator('body').innerText();
  const rows = await page.locator('table tbody tr, [class*="timeline"] li, [class*="event-row"]').count();
  const timestamps = body.match(/\d{2}\.\d{2}\.\d{4}|\d{4}-\d{2}-\d{2}/g) || [];
  const distinct = /История событий|Event History/i.test(body) && (/Хронология|timeline/i.test(body) || rows > 0);
  set('EVENT_HISTORY_DISTINCT_PAGE', distinct ? 'YES' : 'NO');
  set('EVENT_COUNT', String(rows));
  set('EVENT_ROWS_VISIBLE', rows > 0 ? 'YES' : /нет событий/i.test(body) ? 'NO' : 'PARTIAL');
  set('EVENT_TIMESTAMPS_VISIBLE', timestamps.length > 0 ? 'YES' : 'NO');
  set('CANONICAL_DERIVED_VISUAL_DISTINCTION', /derived|вычислен|canonical|канон/i.test(body) ? 'EXPOSED_IN_UI' : 'NOT_EXPOSED_IN_UI');

  await page.screenshot({ path: path.join(OUT, '01-event-history-fixed.png'), fullPage: true });
  console.log(`SCREENSHOT=${path.join(OUT, '01-event-history-fixed.png')}`);

  const pass =
    urlAfter.includes('/events') &&
    distinct &&
    (rows > 0 || timestamps.length > 0);
  set('EVENT_HISTORY_VISUALLY_VERIFIED', pass ? 'YES' : 'NO');
  set('VIS_005_STATUS', pass ? 'FIXED_PENDING_STAGING' : 'OPEN');
  await browser.close();
  process.exit(pass ? 0 : 1);
}

main().catch((e) => {
  console.error('VIS005_E2E_FAIL', e.message);
  process.exit(1);
});
