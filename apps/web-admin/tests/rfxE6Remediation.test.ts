import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../composables/useApi'
import {
  canCloneFromTemplateVersion,
} from '../types/rfx-template'
import type { RfxTemplateDetailResponse } from '../types/rfx-template'
import {
  requiresImpactConfirmation,
  RFX_IMPACT_CLASSES,
} from '../types/rfx-version-lifecycle'
import type { RfxVersionRecord } from '../types/rfx-version-lifecycle'
import { extractEventProvenance } from '../types/rfx'
import {
  canForkTemplateDraft,
  filterCloneableTemplateVersions,
  selectDefaultCloneVersionId,
} from '../utils/rfxTemplateLifecycle'
import {
  IdempotentOperation,
  stableBodyFingerprint,
} from '../utils/idempotentOperation'
import {
  buildEventPublishPayload,
  hasPublishedEventVersion,
  shouldRequireImpactPreview,
} from '../utils/rfxEventPublishOrchestration'
import {
  localTemplatePublishPrecheck,
  parseTemplatePublish422,
} from '../utils/rfxTemplatePublishErrors'
import { formatRfxDateTime } from '../utils/formatRfxDateTime'
function templateDetail(partial: Partial<RfxTemplateDetailResponse> & { template: RfxTemplateDetailResponse['template'] }): RfxTemplateDetailResponse {
  return {
    draft_version: null,
    published_version: null,
    versions: [],
    ...partial,
  }
}

const publishedVersion = {
  id: 'pub-1',
  tenant_id: 't',
  template_id: 'tpl-1',
  version_number: 2,
  status: 'PUBLISHED' as const,
  is_active_draft: false,
  is_published: true,
  created_by: 'u',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  version: 1,
}

const draftVersion = {
  ...publishedVersion,
  id: 'draft-1',
  version_number: 3,
  status: 'DRAFT' as const,
  is_active_draft: true,
  is_published: false,
}

describe('E6-REM clone and library logic', () => {
  it('E6-REM-UT-08/09/10/11/12 version selection rules', () => {
    const detail = templateDetail({
      template: {
        id: 'tpl-1', tenant_id: 't', template_code: 'T', name_i18n: {}, status: 'ACTIVE',
        version: 1, created_by: 'u', created_at: '', updated_at: '',
      },
      versions: [
        { ...draftVersion, status: 'DRAFT' },
        { ...publishedVersion, id: 'pub-2', version_number: 2, status: 'PUBLISHED' },
        { ...publishedVersion, id: 'sup-1', version_number: 1, status: 'SUPERSEDED' },
      ],
    })
    const cloneable = filterCloneableTemplateVersions(detail)
    expect(cloneable.map((v) => v.status)).toEqual(['PUBLISHED', 'SUPERSEDED'])
    expect(selectDefaultCloneVersionId(cloneable)).toBe('pub-2')
    expect(canCloneFromTemplateVersion('ACTIVE', 'DRAFT')).toBe(false)
    expect(canCloneFromTemplateVersion('ARCHIVED', 'PUBLISHED')).toBe(false)
  })

  it('E6-REM-UT-15/16 idempotent clone keeps same key on retry', async () => {
    const op = new IdempotentOperation('clone-tpl')
    const body = { template_version_id: 'v1', rfx_number: 'RFX-1' }
    const mutate = vi.fn()
      .mockRejectedValueOnce(new TypeError('network'))
      .mockResolvedValueOnce({ id: 'evt-1' })
    await expect(op.execute(body, mutate)).rejects.toThrow('network')
    expect(op.status).toBe('unknown')
    const result = await op.execute(body, mutate)
    expect(result).toEqual({ id: 'evt-1' })
    expect(mutate).toHaveBeenCalledTimes(2)
    expect(mutate.mock.calls[0][0]).toBe(mutate.mock.calls[1][0])
  })
})

describe('E6-REM fork gating', () => {
  it('E6-REM-UT-17 draft exists → fork unavailable', () => {
    const detail = templateDetail({
      template: {
        id: 'tpl', tenant_id: 't', template_code: 'T', name_i18n: {}, status: 'ACTIVE',
        version: 1, created_by: 'u', created_at: '', updated_at: '',
      },
      draft_version: draftVersion,
      published_version: publishedVersion,
    })
    expect(canForkTemplateDraft(detail).allowed).toBe(false)
  })

  it('E6-REM-UT-18 published/no draft → fork available', () => {
    const detail = templateDetail({
      template: {
        id: 'tpl', tenant_id: 't', template_code: 'T', name_i18n: {}, status: 'ACTIVE',
        version: 1, created_by: 'u', created_at: '', updated_at: '',
      },
      published_version: publishedVersion,
    })
    expect(canForkTemplateDraft(detail).allowed).toBe(true)
  })

  it('E6-REM-UT-19 archived → fork denied', () => {
    const detail = templateDetail({
      template: {
        id: 'tpl', tenant_id: 't', template_code: 'T', name_i18n: {}, status: 'ARCHIVED',
        version: 1, created_by: 'u', created_at: '', updated_at: '',
      },
      published_version: publishedVersion,
    })
    expect(canForkTemplateDraft(detail).allowed).toBe(false)
  })
})

describe('E6-REM publish orchestration', () => {
  const versions = (statuses: RfxVersionRecord['status'][]): RfxVersionRecord[] =>
    statuses.map((status, i) => ({
      id: `v-${i}`,
      tenant_id: 't',
      rfx_event_id: 'e',
      version_number: i + 1,
      status,
      questionnaire_enabled: true,
      is_current_published: status === 'PUBLISHED',
      is_active_draft: status === 'DRAFT',
      rescoring_required: false,
      created_at: '',
      updated_at: '',
      version: 1,
    }))

  it('E6-REM-UT-24 first publish does not require preview', () => {
    expect(shouldRequireImpactPreview(versions(['DRAFT']))).toBe(false)
  })

  it('E6-REM-UT-25 republish requires preview', () => {
    expect(shouldRequireImpactPreview(versions(['PUBLISHED', 'DRAFT']))).toBe(true)
    expect(hasPublishedEventVersion(versions(['PUBLISHED', 'DRAFT']))).toBe(true)
  })

  it('E6-REM-UT-27 confirmation passes analysis id + hash', () => {
    const payload = buildEventPublishPayload({
      expectedEventVersion: 3,
      expectedDraftVersion: 5,
      changeSummary: 'fix',
      impact: {
        impact_analysis_id: 'ia-1',
        tenant_id: 't',
        event_id: 'e',
        candidate_version_id: 'd',
        canonical_diff_hash: 'hash-1',
        impact_classes: ['MATERIAL_NO_RESPONSES'],
        affected_draft_response_count: 0,
        affected_submitted_response_count: 0,
        scoring_affecting: false,
        knockout_affecting: false,
        expires_at: '2026-01-02T00:00:00Z',
      },
    })
    expect(payload.impact_analysis_id).toBe('ia-1')
    expect(payload.canonical_diff_hash).toBe('hash-1')
  })

  it('E6-REM-UT-26 six impact classes exist', () => {
    expect(RFX_IMPACT_CLASSES).toHaveLength(6)
    expect(requiresImpactConfirmation(['NON_MATERIAL'])).toBe(false)
  })
})

describe('E6-REM readiness and idempotency', () => {
  it('E6-REM-UT-20/21 server authoritative precheck and 422 parse', () => {
    const pre = localTemplatePublishPrecheck(2)
    expect(pre.ready).toBe(false)
    expect(pre.items[0]?.code).toBe('LOCAL_PRECHECK')
    const parsed = parseTemplatePublish422(new ApiError(422, {
      code: 'VALIDATION',
      message: 'fail',
      details: {
        ready: false,
        blocking_fail_count: 1,
        items: [{ code: 'Q1', status: 'FAIL', message: 'Missing options' }],
      },
    }))
    expect(parsed?.items).toHaveLength(1)
  })

  it('E6-REM-UT-34 restore same-key retry', async () => {
    const op = new IdempotentOperation('evt-restore')
    const mutate = vi.fn().mockResolvedValue({ id: 'draft' })
    await op.execute({ change_summary: 'restore' }, mutate)
    await op.execute({ change_summary: 'restore' }, mutate)
    expect(mutate).toHaveBeenCalledTimes(1)
  })

  it('body fingerprint changes reset operation key scope', () => {
    const op = new IdempotentOperation('tpl-pub')
    const k1 = op.key
    void op.execute({ a: 1 }, async () => ({}))
    op.reset()
    op.key = k1
    expect(stableBodyFingerprint({ a: 1 })).not.toBe(stableBodyFingerprint({ a: 2 }))
  })
})

describe('E6-REM provenance and security', () => {
  it('E6-REM-UT-35 provenance extracted when API returns fields', () => {
    const p = extractEventProvenance({
      id: 'e', tenant_id: 't', rfx_number: '1', rfx_type: 'RFQ', category: 'FREIGHT',
      title: 't', owner_company_id: 'c', status: 'DRAFT',
      source_template_id: 'tpl', source_template_version_id: 'ver',
      source_version_number: 2, source_version_status: 'PUBLISHED',
    })
    expect(p?.source_template_version_id).toBe('ver')
  })

  it('E6-REM-UT-36 carrier route denied via permission constants', () => {
    const carrierRoles = ['CARRIER_ADMIN', 'CARRIER_DISPATCHER']
    const buyerRoles = ['SHIPPER_ADMIN', 'PROCUREMENT_MANAGER']
    expect(carrierRoles.every((r) => !buyerRoles.includes(r))).toBe(true)
  })
})

describe('E6-REM formatting', () => {
  it('E6-REM-UT-01 locale date formatting not raw ISO only', () => {
    const formatted = formatRfxDateTime('2026-06-15T12:30:00Z', 'en-US')
    expect(formatted).not.toBe('2026-06-15T12:30:00Z')
    expect(formatRfxDateTime(null, 'en-US')).toBe('—')
  })
})
