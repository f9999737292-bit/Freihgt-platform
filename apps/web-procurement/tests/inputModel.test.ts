import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { applyInputModel } from '~/utils/inputModel'

describe('Input string and number model', () => {
  it('keeps the ordinary string model unchanged', () => {
    expect(applyInputModel('Carrier A', false)).toBe('Carrier A')
    expect(applyInputModel('', false)).toBe('')
    expect(applyInputModel('15000', false)).toBe('15000')
  })

  it('applies v-model.number without coercing empty or garbage to 0', () => {
    expect(applyInputModel('15000', true)).toBe(15000)
    expect(applyInputModel(' 15000 ', true)).toBe(15000)
    expect(applyInputModel('', true)).toBeNull()
    expect(applyInputModel('   ', true)).toBeNull()
    expect(applyInputModel('abc', true)).toBeNull()
  })
})

describe('Input/Select Vue model propagation', () => {
  it('binds the native input through defineModel so Playwright fill updates v-model', () => {
    const source = readFileSync(resolve(import.meta.dirname, '../components/ui/Input.vue'), 'utf8')
    expect(source).toContain('defineModel<string | number | null>')
    expect(source).toContain('const [model, modifiers] = defineModel')
    expect(source).toContain(':value="displayValue()"')
    expect(source).toContain('@input="onInput"')
    expect(source).toContain('inheritAttrs: false')
    expect(source).toContain('v-bind="attrs"')
    expect(source).toContain('applyInputModel')
    expect(source).toContain('Boolean(modifiers.number)')
    expect(source).not.toContain(':value="modelValue"')
    expect(source).not.toContain("$emit('update:modelValue'")
  })

  it('binds the native select through defineModel so selectOption updates v-model', () => {
    const source = readFileSync(resolve(import.meta.dirname, '../components/ui/Select.vue'), 'utf8')
    expect(source).toContain('defineModel<string>')
    expect(source).toContain('v-model="model"')
    expect(source).not.toContain(':value="modelValue"')
    expect(source).not.toContain("$emit('update:modelValue'")
  })

  it('retries carrier load after auth and renders a native participant select', () => {
    const source = readFileSync(resolve(import.meta.dirname, '../pages/tenders/new.vue'), 'utf8')
    expect(source).toContain('void loadCarriers()')
    expect(source).toContain('data-testid="wizard-participant-company"')
    expect(source).toContain('data-testid="wizard-carriers-error"')
    expect(source).toContain("company_type: 'CARRIER'")
    expect(source).not.toMatch(/<Select[^>]*data-testid="wizard-participant-company"/)
  })

  it('wizard title and lot name use native v-model controls and surface saveGeneral errors', () => {
    const source = readFileSync(resolve(import.meta.dirname, '../pages/tenders/new.vue'), 'utf8')
    expect(source).toContain('v-model="form.title"')
    expect(source).toContain('data-testid="wizard-title"')
    expect(source).toContain('v-model="lotForm.name"')
    expect(source).toContain('data-testid="wizard-lot-name"')
    expect(source).toContain('data-testid="wizard-general-error"')
    expect(source).toContain('data-model-title')
    expect(source).toContain('data-model-owner')
    expect(source).not.toMatch(/<Input v-model="form.title"/)
    expect(source).not.toMatch(/<Input v-model="lotForm.name"/)
  })
})
