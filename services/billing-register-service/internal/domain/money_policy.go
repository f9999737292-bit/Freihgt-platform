package domain

// MoneyPolicy documents rounding rules for billing v1.8.
//
// Storage: PostgreSQL NUMERIC(18,2) for amounts, NUMERIC(5,2) for rates.
// Calculation boundary: round2() — half-away-from-zero to 2 decimal places.
// Line items: base + extra - penalties, then VAT on line subtotal.
// Register totals: sum of rounded line amounts (not re-rounding the sum differently).
// Currency: ISO 4217 code; one currency per register; mixed-currency inclusion rejected.
//
// Legacy note: Go domain structs still use float64 for compatibility with existing
// billing-register code paths. New settlement-derived amounts are computed with
// round2() before persistence into NUMERIC columns.

const MoneyDecimalPlaces = 2
