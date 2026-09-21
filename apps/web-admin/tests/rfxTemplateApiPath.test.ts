import { describe, expect, it } from 'vitest'
import { rfxTemplateApiPath, RFX_TEMPLATE_API_ROUTES } from '../utils/rfxTemplateApiRoutes'

describe('rfxTemplateApiPath', () => {
  it('does not append a trailing slash for template detail', () => {
    expect(rfxTemplateApiPath('tpl-1')).toBe('/api/v1/rfx-templates/tpl-1')
    expect(rfxTemplateApiPath('tpl-1', '')).toBe('/api/v1/rfx-templates/tpl-1')
    expect(rfxTemplateApiPath('tpl-1', '/')).toBe('/api/v1/rfx-templates/tpl-1')
    expect(RFX_TEMPLATE_API_ROUTES.getTemplate.path).toBe('/api/v1/rfx-templates/{id}')
  })

  it('keeps published suffix routes without inventing new APIs', () => {
    expect(rfxTemplateApiPath('tpl-1', '/archive')).toBe('/api/v1/rfx-templates/tpl-1/archive')
    expect(rfxTemplateApiPath('tpl-1', 'versions/publish')).toBe('/api/v1/rfx-templates/tpl-1/versions/publish')
  })
})
