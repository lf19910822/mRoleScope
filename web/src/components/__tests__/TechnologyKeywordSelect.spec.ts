import { mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { nextTick } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'

import TechnologyKeywordSelect from '../TechnologyKeywordSelect.vue'

const options = [
  {
    value: 'go',
    label: 'Go',
    category: { value: 'programming-languages', label: '编程语言' },
  },
  {
    value: 'python',
    label: 'Python',
    category: { value: 'programming-languages', label: '编程语言' },
  },
  {
    value: 'pytorch',
    label: 'PyTorch',
    category: { value: 'machine-learning-frameworks', label: '机器学习框架' },
  },
  {
    value: 'rag',
    label: 'RAG',
    category: { value: 'ai-methods-and-mechanisms', label: 'AI 方法与机制' },
  },
]

describe('TechnologyKeywordSelect', () => {
  afterEach(() => {
    document.body
      .querySelectorAll(
        '[data-testid="technology-keyword-menu"], [data-testid="technology-keyword-backdrop"]',
      )
      .forEach((element) => element.remove())
  })

  it('renders an independent categorized panel with a bounded viewport width', () => {
    const source = readFileSync(
      resolve(process.cwd(), 'src/components/TechnologyKeywordSelect.vue'),
      'utf8',
    )
    const menuRule = source.match(
      /\.technology-keyword-select__menu \{(?<declarations>[\s\S]*?)\n\}/u,
    )?.groups?.declarations
    const browserRule = source.match(
      /\.technology-keyword-select__browser \{(?<declarations>[\s\S]*?)\n\}/u,
    )?.groups?.declarations

    expect(source).toMatch(/<Teleport\s+to="body">/u)
    expect(menuRule).toMatch(/position:\s*fixed;/u)
    expect(menuRule).toMatch(/max-height:/u)
    expect(menuRule).toMatch(/min-width:\s*0;/u)
    expect(menuRule).toMatch(/width:\s*min\(640px, calc\(100vw - 32px\)\);/u)
    expect(source).toMatch(
      /\.technology-keyword-select \{[\s\S]*?min-width:\s*0;[\s\S]*?width:\s*100%;/u,
    )
    expect(browserRule).toMatch(/grid-template-columns:\s*minmax\(164px, 43%\) minmax\(0, 1fr\);/u)
    expect(source).toMatch(
      /\.technology-keyword-select__categories \{[\s\S]*?overscroll-behavior:\s*contain;/u,
    )
    expect(source).toMatch(
      /\.technology-keyword-select__terms \{[\s\S]*?overscroll-behavior:\s*contain;/u,
    )
    expect(source).toMatch(
      /\.technology-keyword-select__category \{[\s\S]*?white-space:\s*nowrap;/u,
    )
  })

  it('browses a selected category in dictionary order', async () => {
    const wrapper = mount(TechnologyKeywordSelect, {
      props: { modelValue: [], options },
    })

    await wrapper.get('[data-testid="technology-keyword-trigger"]').trigger('click')
    const menu = () =>
      document.body.querySelector<HTMLElement>('[data-testid="technology-keyword-menu"]')!
    const menuQuery = (selector: string) => menu().querySelector<HTMLElement>(selector)!
    await menuQuery('[data-testid="technology-category-programming-languages"]').click()
    await nextTick()
    const programmingTerms = menuQuery('[data-testid="technology-keyword-terms"]').textContent ?? ''
    expect(programmingTerms.indexOf('Go')).toBeLessThan(programmingTerms.indexOf('Python'))

    menuQuery('[data-testid="technology-category-ai-methods-and-mechanisms"]').click()
    await nextTick()

    expect(menuQuery('[data-testid="technology-keyword-terms"]').textContent).toContain('RAG')
    expect(menuQuery('[data-testid="technology-keyword-terms"]').textContent).not.toContain('Go')
  })

  it('searches all categories without dropping selections from the browse category', async () => {
    const wrapper = mount(TechnologyKeywordSelect, {
      props: { modelValue: ['go'], options },
    })

    await wrapper.get('[data-testid="technology-keyword-trigger"]').trigger('click')
    const menu = () =>
      document.body.querySelector<HTMLElement>('[data-testid="technology-keyword-menu"]')!
    const menuQuery = (selector: string) => menu().querySelector<HTMLElement>(selector)!
    menuQuery('[data-testid="technology-category-ai-methods-and-mechanisms"]').click()
    await nextTick()
    const searchInput = menuQuery('[data-testid="technology-keyword-search"]') as HTMLInputElement
    searchInput.value = 'py'
    searchInput.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()

    const terms = menuQuery('[data-testid="technology-keyword-terms"]').textContent ?? ''
    expect(terms).toContain('Python')
    expect(terms).toContain('PyTorch')
    expect(menuQuery('[data-testid="technology-keyword-selected"]').textContent).toContain('Go')
  })

  it('emits the same selected keyword keys when a term is toggled', async () => {
    const wrapper = mount(TechnologyKeywordSelect, {
      props: { modelValue: ['go'], options },
    })

    await wrapper.get('[data-testid="technology-keyword-trigger"]').trigger('click')
    const menu = () =>
      document.body.querySelector<HTMLElement>('[data-testid="technology-keyword-menu"]')!
    const menuQuery = (selector: string) => menu().querySelector<HTMLElement>(selector)!
    menuQuery('[data-testid="technology-category-ai-methods-and-mechanisms"]').click()
    await nextTick()
    menuQuery('[data-testid="technology-keyword-rag"]').click()

    expect(wrapper.emitted('update:modelValue')).toEqual([[['go', 'rag']]])
  })

  it('opens as an independent panel and closes with Escape or an outside click', async () => {
    const wrapper = mount(TechnologyKeywordSelect, {
      props: { modelValue: [], options },
    })

    await wrapper.get('[data-testid="technology-keyword-trigger"]').trigger('click')
    expect(document.body.querySelector('[data-testid="technology-keyword-menu"]')).not.toBeNull()
    expect(
      wrapper.get('[data-testid="technology-keyword-trigger"]').attributes('aria-controls'),
    ).toBe(document.body.querySelector('[data-testid="technology-keyword-menu"]')?.id)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(document.body.querySelector('[data-testid="technology-keyword-menu"]')).toBeNull()

    await wrapper.get('[data-testid="technology-keyword-trigger"]').trigger('click')
    document.dispatchEvent(new Event('pointerdown', { bubbles: true }))
    await nextTick()
    expect(document.body.querySelector('[data-testid="technology-keyword-menu"]')).toBeNull()

    wrapper.unmount()
  })
})
