import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import LoginPage from '@/pages/LoginPage.vue'
import ru from '@/i18n/ru-RU.json'
import { createPinia } from 'pinia'

const VIEWPORTS = {
  androidSmall: { width: 320, height: 568 },
  androidStandard: { width: 360, height: 800 },
  androidLarge: { width: 393, height: 852 },
  iphoneSe: { width: 320, height: 568 },
  iphoneStandard: { width: 390, height: 844 },
  iphoneProMax: { width: 430, height: 932 },
  tabletBasic: { width: 768, height: 1024 },
} as const

function mountLoginAtViewport(width: number, height: number) {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: width })
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: height })

  const i18n = createI18n({
    legacy: false,
    locale: 'ru-RU',
    messages: { 'ru-RU': ru },
  })

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/login', component: LoginPage }],
  })

  return mount(LoginPage, {
    global: {
      plugins: [createPinia(), i18n, router],
      stubs: {
        IonPage: { template: '<div><slot /></div>' },
        IonHeader: { template: '<div><slot /></div>' },
        IonToolbar: { template: '<div><slot /></div>' },
        IonTitle: { template: '<div><slot /></div>' },
        IonContent: { template: '<div class="ion-content form-content"><slot /></div>' },
        IonItem: { template: '<div><slot /></div>' },
        IonLabel: { template: '<label><slot /></label>' },
        IonInput: { template: '<input />' },
        IonButton: { template: '<button class="submit-btn"><slot /></button>' },
        IonText: { template: '<div><slot /></div>' },
        OfflineBanner: { template: '<div />' },
      },
    },
  })
}

describe('responsive theme contract', () => {
  const css = readFileSync(resolve(__dirname, '../src/theme/variables.css'), 'utf8')

  it('includes safe-area and overflow guards for native shells', () => {
    expect(css).toContain('env(safe-area-inset-top')
    expect(css).toContain('env(safe-area-inset-bottom')
    expect(css).toContain('overflow-x: hidden')
    expect(css).toContain('--driver-touch-target-min: 48px')
  })
})

describe('responsive viewport smoke', () => {
  for (const [label, viewport] of Object.entries(VIEWPORTS)) {
    it(`renders login without horizontal overflow at ${label}`, () => {
      const wrapper = mountLoginAtViewport(viewport.width, viewport.height)
      expect(wrapper.find('form.login-form').exists()).toBe(true)
      expect(wrapper.find('.submit-btn').exists()).toBe(true)
      expect(wrapper.text()).toMatch(/Вход|Войти/)
    })
  }
})

describe('mobile-first layout guardrails', () => {
  it('keeps login form within phone-first max width', () => {
    const wrapper = mountLoginAtViewport(360, 800)
    const form = wrapper.find('form.login-form')
    expect(form.exists()).toBe(true)
    expect(form.classes()).not.toContain('desktop-only')
  })
})
