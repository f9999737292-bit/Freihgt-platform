import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = join(import.meta.dirname, '..')
const componentsDeclaration = readFileSync(join(root, '.nuxt/components.d.ts'), 'utf8')

function collectVueFiles(dir: string): string[] {
  const entries = readdirSync(dir, { withFileTypes: true })
  const files: string[] = []
  for (const entry of entries) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) files.push(...collectVueFiles(path))
    else if (entry.name.endsWith('.vue')) files.push(path)
  }
  return files
}

function templateSection(source: string): string {
  const start = source.indexOf('<template>')
  const end = source.lastIndexOf('</template>')
  if (start === -1 || end === -1) return source
  return source.slice(start, end)
}

describe('rfx component auto-import resolution', () => {
  it('registers RFx create modal under RfxCreateModal', () => {
    expect(componentsDeclaration).toContain('export const RfxCreateModal')
    expect(componentsDeclaration).not.toContain('export const RfxRfxCreateModal')
  })

  it('does not reference deprecated RfxRfx-prefixed component tags in RFx templates', () => {
    const rfxSources = [
      ...collectVueFiles(join(root, 'components/rfx')),
      ...collectVueFiles(join(root, 'pages/rfx')),
    ]
    const offenders = rfxSources.filter((file) => /RfxRfx[A-Z]/.test(templateSection(readFileSync(file, 'utf8'))))
    expect(offenders).toEqual([])
  })

  it('references RfxCreateModal on the RFx index page', () => {
    const indexSource = readFileSync(join(root, 'pages/rfx/index.vue'), 'utf8')
    expect(templateSection(indexSource)).toContain('<RfxCreateModal')
  })

  it('uses registered Nuxt auto-import names for RFx Studio components', () => {
    const registered = new Set(
      [...componentsDeclaration.matchAll(/export const (RfxStudio[A-Za-z0-9]+):/g)].map((match) => match[1]),
    )
    const studioSources = [
      ...collectVueFiles(join(root, 'components/rfx/studio')),
      ...collectVueFiles(join(root, 'pages/rfx')).filter((file) => file.includes('studio')),
    ]
    const unresolvedShortTags = [
      'RfxQuestionnaireBuilder',
      'RfxSectionCard',
      'RfxQuestionCard',
      'RfxPublishReadinessPanel',
      'RfxCarrierPreview',
      'QuestionPropertyPanel',
      'QuestionOptionsEditor',
      'ConditionalRuleEditor',
    ]
    for (const file of studioSources) {
      const template = templateSection(readFileSync(file, 'utf8'))
      for (const tag of unresolvedShortTags) {
        expect(template, `${file} must not use unresolved <${tag}`).not.toMatch(new RegExp(`<${tag}[\\s/>]`))
      }
      for (const match of template.matchAll(/<(RfxStudio[A-Za-z0-9]+)/g)) {
        expect(registered, `${file} references unregistered ${match[1]}`).toContain(match[1])
      }
    }
    const studioIndex = readFileSync(join(root, 'pages/rfx/[id]/studio/index.vue'), 'utf8')
    expect(templateSection(studioIndex)).toContain('<RfxStudioRfxQuestionnaireBuilder')
  })
})
