import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  canCloneFromTemplateVersion,
  isTemplateVersionEditable,
  isTemplateVersionReadOnly,
} from '../types/rfx-template'
import {
  isEventVersionEditable,
  isEventVersionReadOnly,
  requiresImpactConfirmation,
  RFX_IMPACT_CLASSES,
} from '../types/rfx-version-lifecycle'
import {
  compareItemLabel,
  filterVisibleCompareItems,
  groupCompareItemsByChange,
  isCompareEmpty,
  sortImpactClasses,
  swapCompareDirection,
} from '../utils/rfxVersionCompare'
import { ApiError } from '../composables/useApi'
import {
  classifyRfxHttpError,
  resolveRfxConflictDetailKey,
  resolveRfxErrorMessageKey,
} from '../utils/rfxApiError'
import { createIdempotencyKey, stableIdempotencyKey } from '../utils/idempotencyKey'
import { RFX_TEMPLATE_OPENAPI_PARITY } from '../utils/rfxTemplateApiRoutes'
import { RFX_VERSION_OPENAPI_PARITY } from '../utils/rfxVersionApiRoutes'
import { buildStudioNavSteps } from '../components/rfx/studio/studioNav'

const openapiPath = resolve(import.meta.dirname, '../../../packages/openapi/rfx-service.yaml')
const openapiYaml = readFileSync(openapiPath, 'utf8')
const en = JSON.parse(readFileSync(resolve(import.meta.dirname, '../i18n/en-US.json'), 'utf8'))
const ru = JSON.parse(readFileSync(resolve(import.meta.dirname, '../i18n/ru-RU.json'), 'utf8'))
const zh = JSON.parse(readFileSync(resolve(import.meta.dirname, '../i18n/zh-CN.json'), 'utf8'))

const mockCompareItem = (change: 'ADDED' | 'REMOVED' | 'CHANGED' | 'REORDERED' | 'UNCHANGED') => ({
  entity_type: 'question',
  change,
  section_code: 'SEC_1',
  question_code: 'Q_1',
})

describe('E6-UT template library helpers', () => {
  it('E6-UT-04/06 draft editable vs published read-only', () => {
    expect(isTemplateVersionEditable('DRAFT')).toBe(true)
    expect(isTemplateVersionReadOnly('PUBLISHED')).toBe(true)
    expect(isTemplateVersionReadOnly('SUPERSEDED')).toBe(true)
    expect(isEventVersionEditable('DRAFT')).toBe(true)
    expect(isEventVersionReadOnly('PUBLISHED')).toBe(true)
    expect(isEventVersionReadOnly('ARCHIVED')).toBe(true)
  })

  it('E6-UT-26..28 clone eligibility', () => {
    expect(canCloneFromTemplateVersion('ACTIVE', 'PUBLISHED')).toBe(true)
    expect(canCloneFromTemplateVersion('ACTIVE', 'SUPERSEDED')).toBe(true)
    expect(canCloneFromTemplateVersion('ACTIVE', 'DRAFT')).toBe(false)
    expect(canCloneFromTemplateVersion('ARCHIVED', 'PUBLISHED')).toBe(false)
  })
})

describe('E6-UT compare helpers', () => {
  it('E6-UT-14..16 compare grouping and reverse', () => {
    const items = [
      mockCompareItem('ADDED'),
      mockCompareItem('REMOVED'),
      mockCompareItem('UNCHANGED'),
    ]
    const visible = filterVisibleCompareItems(items)
    expect(visible).toHaveLength(2)
    const grouped = groupCompareItemsByChange(items)
    expect(grouped.ADDED).toHaveLength(1)
    expect(grouped.UNCHANGED).toHaveLength(1)
    expect(
      swapCompareDirection({ source_version_id: 'a', target_version_id: 'b' }),
    ).toEqual({ source_version_id: 'b', target_version_id: 'a' })
    expect(isCompareEmpty({ added_count: 0, removed_count: 0, changed_count: 0, reordered_count: 0, unchanged_count: 3 })).toBe(true)
    expect(compareItemLabel(mockCompareItem('CHANGED'))).toContain('SEC_1')
  })
})

describe('E6-UT change impact', () => {
  it('E6-UT-21/25 six impact classes and confirmation', () => {
    expect(RFX_IMPACT_CLASSES).toHaveLength(6)
    expect(requiresImpactConfirmation(['NON_MATERIAL'])).toBe(false)
    expect(requiresImpactConfirmation(['MATERIAL_WITH_SUBMITTED_RESPONSES'])).toBe(true)
    const sorted = sortImpactClasses(['NON_MATERIAL', 'KNOCKOUT_AFFECTING', 'MATERIAL_NO_RESPONSES'])
    expect(sorted[0]).toBe('KNOCKOUT_AFFECTING')
  })
})

describe('E6-UT error semantics', () => {
  it('E6-UT-22..24/34 conflict and http mapping', () => {
    expect(classifyRfxHttpError(new ApiError(409, { code: 'CONFLICT', message: 'x', details: {} }))).toBe('conflict')
    expect(classifyRfxHttpError(new ApiError(422, { code: 'VALIDATION', message: 'x', details: {} }))).toBe('validation')
    expect(resolveRfxConflictDetailKey('STALE_DIFF')).toBe('rfx.errors.staleDiff')
    expect(resolveRfxConflictDetailKey('IMPACT_ANALYSIS_CONSUMED')).toBe('rfx.errors.impactAnalysisConsumed')
    expect(resolveRfxErrorMessageKey('validation')).toBe('rfx.errors.validation')
    expect(resolveRfxErrorMessageKey('forbidden')).toBe('rfx.errors.forbidden')
  })
})

describe('E6-UT idempotency', () => {
  it('E6-UT-19/29/30 keys within OpenAPI max length', () => {
    const key = createIdempotencyKey('restore')
    expect(key.length).toBeLessThanOrEqual(128)
    expect(stableIdempotencyKey('clone', 'body-hash')).toContain('clone-body-hash')
  })
})

describe('E6-UT OpenAPI parity', () => {
  it('E6-UT-36 template and version routes exist in OpenAPI', () => {
    for (const route of [...RFX_TEMPLATE_OPENAPI_PARITY, ...RFX_VERSION_OPENAPI_PARITY]) {
      expect(openapiYaml).toContain(route.path.replace('{id}', '{id}'))
    }
  })
})

describe('E6-UT studio nav regression', () => {
  it('E6-UT-36 adds version history when versioning enabled', () => {
    const t = (k: string) => k
    const steps = buildStudioNavSteps('evt-1', 'questionnaire', t, { versioningEnabled: true })
    expect(steps.some((s) => s.id === 'versionHistory')).toBe(true)
    const legacy = buildStudioNavSteps('evt-1', 'questionnaire', t)
    expect(legacy.some((s) => s.id === 'versionHistory')).toBe(false)
  })
})

describe('E6-UT i18n completeness', () => {
  const keys = [
    'rfx.templates.libraryTitle',
    'rfx.versions.title',
    'rfx.compare.title',
    'rfx.restore.title',
    'rfx.changeImpact.rescoringRequired',
    'rfx.errors.staleDiff',
    'nav.rfxTemplates',
  ]

  function get(obj: Record<string, unknown>, path: string): unknown {
    return path.split('.').reduce<unknown>((acc, part) => {
      if (acc && typeof acc === 'object' && part in (acc as Record<string, unknown>)) {
        return (acc as Record<string, unknown>)[part]
      }
      return undefined
    }, obj)
  }

  it('E6-UT-31 EN keys complete', () => {
    for (const key of keys) expect(get(en, key)).toBeTruthy()
  })
  it('E6-UT-32 RU keys complete', () => {
    for (const key of keys) expect(get(ru, key)).toBeTruthy()
  })
  it('E6-UT-33 ZH keys complete', () => {
    for (const key of keys) expect(get(zh, key)).toBeTruthy()
  })
})

describe('E6-UT competitor confidentiality regression', () => {
  it('E6-UT-35 carrier templates middleware fails closed', () => {
    const middleware = readFileSync(resolve(import.meta.dirname, '../middleware/rfx-buyer-manage.ts'), 'utf8')
    expect(middleware).toContain('canManageRfxTemplates')
    expect(middleware).toContain('isCarrierWithoutBuyerAccess')
  })
})
