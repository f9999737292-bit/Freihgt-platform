import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import {
  LEGACY_BRAND_MARKERS,
  PRODUCT_ADMIN_TITLE,
  PRODUCT_DISPLAY_NAME,
  PRODUCT_NAME,
  PRODUCT_WORDMARK,
  containsLegacyBrand,
} from '../utils/productBranding'

const WEB_ADMIN_ROOT = fileURLToPath(new URL('..', import.meta.url))

const ACTIVE_SOURCE_DIRS = [
  'components',
  'layouts',
  'pages',
  'composables',
  'utils',
  'i18n',
  'assets',
]

const ACTIVE_SOURCE_FILES = ['nuxt.config.ts', 'app.vue']

function collectFiles(dir: string, acc: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    const stat = statSync(full)
    if (stat.isDirectory()) {
      if (entry === 'node_modules' || entry === '.nuxt' || entry === 'tests') continue
      collectFiles(full, acc)
    } else if (/\.(vue|ts|json|css|html|svg|mjs)$/.test(entry)) {
      acc.push(full)
    }
  }
  return acc
}

function readActiveSources(): string[] {
  const files: string[] = []
  for (const dir of ACTIVE_SOURCE_DIRS) {
    const path = join(WEB_ADMIN_ROOT, dir)
    try {
      collectFiles(path, files)
    } catch {
      // optional dir
    }
  }
  for (const file of ACTIVE_SOURCE_FILES) {
    const path = join(WEB_ADMIN_ROOT, file)
    try {
      files.push(path)
    } catch {
      // optional file
    }
  }
  return files
}

describe('product branding constants', () => {
  it('defines canonical BINTRANS active identity', () => {
    expect(PRODUCT_NAME).toBe('BINTRANS')
    expect(PRODUCT_WORDMARK).toBe('BINTRANS')
    expect(PRODUCT_DISPLAY_NAME).toContain('Bintrans')
    expect(PRODUCT_ADMIN_TITLE).toContain('Bintrans')
  })

  it('detects legacy 7R branding markers', () => {
    expect(containsLegacyBrand('sidebar logo 7R')).toBe(true)
    expect(containsLegacyBrand('7Rights Freight Platform Admin')).toBe(true)
    expect(containsLegacyBrand('BINTRANS Freight Platform')).toBe(false)
  })
})

describe('active runtime branding sources (VIS-001)', () => {
  it('removes legacy 7R mark from auth layout', () => {
    const authLayout = readFileSync(join(WEB_ADMIN_ROOT, 'layouts', 'auth.vue'), 'utf8')
    expect(authLayout).not.toMatch(/>7R</)
    expect(authLayout).toContain('LayoutProductWordmark')
    expect(authLayout).toContain('PRODUCT_DISPLAY_NAME')
  })

  it('removes legacy 7R mark from app sidebar', () => {
    const sidebar = readFileSync(join(WEB_ADMIN_ROOT, 'components', 'layout', 'AppSidebar.vue'), 'utf8')
    expect(sidebar).not.toMatch(/>7R</)
    expect(sidebar).toContain('LayoutProductWordmark')
  })

  it('uses BINTRANS wordmark component', () => {
    const wordmark = readFileSync(join(WEB_ADMIN_ROOT, 'components', 'layout', 'ProductWordmark.vue'), 'utf8')
    expect(wordmark).toContain('PRODUCT_WORDMARK')
    expect(wordmark).not.toMatch(/>7R</)
  })

  it('sets app title metadata to Bintrans admin branding', () => {
    const nuxtConfig = readFileSync(join(WEB_ADMIN_ROOT, 'nuxt.config.ts'), 'utf8')
    expect(nuxtConfig).toContain("title: 'Bintrans Freight Platform Admin'")
    expect(nuxtConfig).toContain("'Bintrans Freight Platform Admin'")
  })

  it('scans active frontend sources and finds no legacy 7R/7rights branding', () => {
    const excluded = new Set([
      join(WEB_ADMIN_ROOT, 'utils', 'productBranding.ts'),
      join(WEB_ADMIN_ROOT, 'tests', 'productBranding.test.ts'),
    ])
    const uiPatterns = [
      />7R</,
      /["']7R["']/,
      /7[Rr]ights Freight Platform/,
      /7rights\.ru/i,
    ]
    const offenders: string[] = []
    for (const file of readActiveSources()) {
      if (excluded.has(file)) continue
      const source = readFileSync(file, 'utf8')
      if (uiPatterns.some((pattern) => pattern.test(source))) {
        offenders.push(file.replace(`${WEB_ADMIN_ROOT}\\`, '').replace(`${WEB_ADMIN_ROOT}/`, ''))
      }
    }
    expect(offenders, `legacy branding in: ${offenders.join(', ')}`).toEqual([])
  })

  it('documents forbidden legacy markers for regression checks', () => {
    expect(LEGACY_BRAND_MARKERS).toContain('7R')
    expect(LEGACY_BRAND_MARKERS).toContain('7rights.ru')
  })
})

describe('branding regression contract labels', () => {
  it('LOGIN_LEGACY_BRAND_ABSENT and LOGIN_BINTRANS_PRESENT', () => {
    const authLayout = readFileSync(join(WEB_ADMIN_ROOT, 'layouts', 'auth.vue'), 'utf8')
    expect(authLayout.includes('7R')).toBe(false)
    expect(authLayout.includes('LayoutProductWordmark')).toBe(true)
  })

  it('AUTH_LAYOUT_LEGACY_BRAND_ABSENT and AUTH_LAYOUT_BINTRANS_PRESENT', () => {
    const authLayout = readFileSync(join(WEB_ADMIN_ROOT, 'layouts', 'auth.vue'), 'utf8')
    expect(authLayout).not.toMatch(/\b7R\b/)
    expect(authLayout).toContain('LayoutProductWordmark')
  })

  it('APP_TITLE_BINTRANS_PRESENT', () => {
    const nuxtConfig = readFileSync(join(WEB_ADMIN_ROOT, 'nuxt.config.ts'), 'utf8')
    expect(nuxtConfig).toMatch(/Bintrans Freight Platform Admin/)
  })
})
