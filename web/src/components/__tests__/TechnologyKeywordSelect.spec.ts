import { mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

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
  it('contains its category and term browser within the trigger width', () => {
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

    expect(menuRule).toMatch(/left:\s*auto;/u)
    expect(menuRule).toMatch(/right:\s*0;/u)
    expect(menuRule).toMatch(/min-width:\s*0;/u)
    expect(menuRule).toMatch(/width:\s*min\(640px, 100%\);/u)
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
    await wrapper.get('[data-testid="technology-category-programming-languages"]').trigger('click')
    const programmingTerms = wrapper.get('[data-testid="technology-keyword-terms"]').text()
    expect(programmingTerms.indexOf('Go')).toBeLessThan(programmingTerms.indexOf('Python'))

    await wrapper
      .get('[data-testid="technology-category-ai-methods-and-mechanisms"]')
      .trigger('click')

    expect(wrapper.get('[data-testid="technology-keyword-terms"]').text()).toContain('RAG')
    expect(wrapper.get('[data-testid="technology-keyword-terms"]').text()).not.toContain('Go')
  })

  it('searches all categories without dropping selections from the browse category', async () => {
    const wrapper = mount(TechnologyKeywordSelect, {
      props: { modelValue: ['go'], options },
    })

    await wrapper.get('[data-testid="technology-keyword-trigger"]').trigger('click')
    await wrapper
      .get('[data-testid="technology-category-ai-methods-and-mechanisms"]')
      .trigger('click')
    await wrapper.get('[data-testid="technology-keyword-search"]').setValue('py')

    const terms = wrapper.get('[data-testid="technology-keyword-terms"]').text()
    expect(terms).toContain('Python')
    expect(terms).toContain('PyTorch')
    expect(wrapper.get('[data-testid="technology-keyword-selected"]').text()).toContain('Go')
  })

  it('emits the same selected keyword keys when a term is toggled', async () => {
    const wrapper = mount(TechnologyKeywordSelect, {
      props: { modelValue: ['go'], options },
    })

    await wrapper.get('[data-testid="technology-keyword-trigger"]').trigger('click')
    await wrapper
      .get('[data-testid="technology-category-ai-methods-and-mechanisms"]')
      .trigger('click')
    await wrapper.get('[data-testid="technology-keyword-rag"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([[['go', 'rag']]])
  })
})
