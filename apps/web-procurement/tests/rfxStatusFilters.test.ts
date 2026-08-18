import { describe, expect, it } from 'vitest'
import { RFX_STATUSES } from '~/types/rfx'
import {
  buildStatusFilterOptions,
  filterByStatus,
  matchesStatusFilter,
} from '~/utils/rfxStatusFilters'

describe('rfxStatusFilters', () => {
  it('builds options with an empty all-value first', () => {
    const options = buildStatusFilterOptions(RFX_STATUSES, 'All')
    expect(options[0]).toEqual({ label: 'All', value: '' })
    expect(options).toHaveLength(RFX_STATUSES.length + 1)
  })

  it('matches only when filter is empty or equal', () => {
    expect(matchesStatusFilter('DRAFT', '')).toBe(true)
    expect(matchesStatusFilter('DRAFT', 'DRAFT')).toBe(true)
    expect(matchesStatusFilter('PUBLISHED', 'DRAFT')).toBe(false)
  })

  it('filters list items by status', () => {
    const items = [
      { id: '1', status: 'DRAFT' },
      { id: '2', status: 'PUBLISHED' },
    ]
    expect(filterByStatus(items, 'DRAFT')).toEqual([{ id: '1', status: 'DRAFT' }])
    expect(filterByStatus(items, '')).toEqual(items)
  })
})
