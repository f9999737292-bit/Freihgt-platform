/**
 * @vitest-environment jsdom
 */
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { computed, defineComponent, h } from 'vue'

const nuxtGlobals = globalThis as typeof globalThis & { computed: typeof computed }
nuxtGlobals.computed = computed
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import Badge from '~/components/ui/Badge.vue'

const pageSource = readFileSync(resolve(__dirname, '../pages/tenders/[id]/index.vue'), 'utf8')

function pageImportsBadge(source: string): boolean {
  return /import\s+Badge\s+from\s+['"]~\/components\/ui\/Badge\.vue['"]/.test(source)
}

describe('tender detail status badge', () => {
  it('renders the API status as visible badge text instead of an unknown badge element', () => {
    expect(pageSource).toContain('<dd data-testid="tender-status"><Badge :status="event.status" /></dd>')
    expect(pageSource).toContain('data-testid="tender-creation-channel"')
    const registered = pageImportsBadge(pageSource)
    const wrapper = mount(defineComponent({
      components: registered ? { Badge } : {},
      setup() {
        const badge = registered
          ? h(Badge, { status: 'DRAFT' })
          : h('badge', { status: 'DRAFT' })
        return () => h('dd', { 'data-testid': 'tender-status' }, [badge])
      },
    }))
    const status = wrapper.get('[data-testid="tender-status"]')
    expect(status.text()).toBe('DRAFT')
    expect(status.find('.ui-badge').exists()).toBe(true)
    expect(status.find('badge').exists()).toBe(false)
    expect(status.html()).not.toContain('<badge')
  })
})
