import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { canPublishStatus } from '~/types/rfx'
import { hasTenderManageRole, tenderWorkspaceActionVisibility } from '~/utils/tenderWorkspaceActions'

describe('tender workspace actions', () => {
  it('explicitly imports PageHeader/Button/Card and keeps permission guards', () => {
    const source = readFileSync(resolve(import.meta.dirname, '../pages/tenders/[id]/index.vue'), 'utf8')
    expect(source).toContain("import PageHeader from '~/components/ui/PageHeader.vue'")
    expect(source).toContain("import Button from '~/components/ui/Button.vue'")
    expect(source).toContain("import Card from '~/components/ui/Card.vue'")
    expect(source).toContain('data-testid="tender-back"')
    expect(source).toContain('data-testid="tender-edit"')
    expect(source).toContain('data-testid="tender-evaluation"')
    expect(source).toContain('data-testid="tender-publish"')
    expect(source).toContain('data-testid="tender-cancel"')
    expect(source).toContain('canManageTenders()')
    expect(source).toContain('canPublishTenders()')
    expect(source).toContain('canPublishStatus(event.status)')
  })

  it('lets PROCUREMENT_MANAGER manage and publish a DRAFT', () => {
    const actions = tenderWorkspaceActionVisibility({
      status: 'DRAFT',
      roles: ['PROCUREMENT_MANAGER'],
    })
    expect(hasTenderManageRole(['PROCUREMENT_MANAGER'])).toBe(true)
    expect(canPublishStatus('DRAFT')).toBe(true)
    expect(actions).toEqual({
      back: true,
      edit: true,
      evaluation: true,
      publish: true,
      cancel: true,
    })
  })

  it('hides buyer actions from a carrier', () => {
    const actions = tenderWorkspaceActionVisibility({
      status: 'DRAFT',
      roles: ['CARRIER_DISPATCHER'],
    })
    expect(hasTenderManageRole(['CARRIER_DISPATCHER'])).toBe(false)
    expect(actions).toEqual({
      back: true,
      edit: false,
      evaluation: false,
      publish: false,
      cancel: false,
    })
  })

  it('hides publish when the event is not DRAFT', () => {
    expect(canPublishStatus('PUBLISHED')).toBe(false)
    expect(canPublishStatus('AWARDED')).toBe(false)
    expect(tenderWorkspaceActionVisibility({
      status: 'PUBLISHED',
      roles: ['PROCUREMENT_MANAGER'],
    }).publish).toBe(false)
  })
})
