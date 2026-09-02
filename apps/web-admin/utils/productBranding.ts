/** Canonical active product branding for BINTRANS web-admin. */
export const PRODUCT_NAME = 'BINTRANS'
export const PRODUCT_DISPLAY_NAME = 'Bintrans Freight Platform'
export const PRODUCT_WORDMARK = 'BINTRANS'
export const PRODUCT_ADMIN_TITLE = 'Bintrans Freight Platform Admin'

/** Detect legacy marks that must not appear in active runtime UI. */
export function containsLegacyBrand(text: string): boolean {
  const seven = String.fromCharCode(55)
  return new RegExp(
    `\\b${seven}R\\b|${seven}[Rr]ights|${seven}rights\\.ru|${seven}-rights|seven rights`,
    'i',
  ).test(text)
}
