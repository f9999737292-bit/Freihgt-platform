/** Canonical active product branding for BINTRANS web-admin. */
export const PRODUCT_NAME = 'BINTRANS'
export const PRODUCT_DISPLAY_NAME = 'Bintrans Freight Platform'
export const PRODUCT_WORDMARK = 'BINTRANS'
export const PRODUCT_ADMIN_TITLE = 'Bintrans Freight Platform Admin'

/** Legacy marks that must not appear in active runtime UI. */
export const LEGACY_BRAND_MARKERS = ['7R', '7Rights', '7rights', '7rights.ru'] as const

export function containsLegacyBrand(text: string): boolean {
  return /\b7R\b|7[Rr]ights|7rights\.ru|7-rights|seven rights/i.test(text)
}
