import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick } from 'vue'

import FilterDatePicker from '../FilterDatePicker.vue'

const PopoverStub = defineComponent({
  name: 'Popover',
  props: { show: Boolean },
  emits: ['update:show'],
  template: '<div class="popover-stub"><slot name="trigger" /><slot /></div>',
})

const DatePickerStub = defineComponent({
  name: 'DatePicker',
  props: {
    formattedValue: { type: [String, Array], default: null },
    valueFormat: { type: String, default: '' },
  },
  emits: ['update:formattedValue'],
  template: '<button data-testid="date-picker-panel" type="button"></button>',
})

function mountPicker(modelValue = '2026-10-02') {
  return mount(FilterDatePicker, {
    global: {
      stubs: { DatePicker: DatePickerStub, Popover: PopoverStub },
    },
    props: {
      ariaLabel: '选择首次发现日期',
      modelValue,
    },
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('FilterDatePicker', () => {
  it('uses a calendar trigger and preserves the ISO value contract', () => {
    const wrapper = mountPicker()

    const calendar = wrapper.findComponent(DatePickerStub)
    expect(wrapper.get('[data-testid="filter-date-picker-trigger"]').element.tagName).toBe('BUTTON')
    expect(wrapper.text()).toContain('2026.10.02')
    expect(wrapper.find('input[type="date"]').exists()).toBe(false)
    expect(calendar.props('formattedValue')).toBe('2026-10-02')
    expect(calendar.props('valueFormat')).toBe('yyyy-MM-dd')
  })

  it('emits an ISO selected date and clears only its own value', async () => {
    const wrapper = mountPicker()

    await wrapper.findComponent(DatePickerStub).vm.$emit('update:formattedValue', '2026-10-31')
    expect(wrapper.emitted('update:modelValue')).toEqual([['2026-10-31']])

    await wrapper.get('[data-testid="filter-date-picker-clear"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([['2026-10-31'], ['']])
  })

  it('opens the installed calendar panel from its trigger', async () => {
    const wrapper = mount(FilterDatePicker, {
      attachTo: document.body,
      props: {
        ariaLabel: '选择首次发现日期',
        modelValue: '',
      },
    })

    await wrapper.get('[data-testid="filter-date-picker-trigger"]').trigger('click')
    await nextTick()
    await flushPromises()

    const selectableDay = document.body.querySelector<HTMLElement>(
      '.n-date-panel-date:not(.n-date-panel-date--excluded)',
    )
    const popover = document.body.querySelector<HTMLElement>('.n-popover')
    expect(document.body.querySelector('.n-date-panel')).not.toBeNull()
    expect(document.body.querySelector('input[type="date"]')).toBeNull()
    if (popover === null) {
      throw new Error('the calendar positioning container did not render')
    }
    expect(popover.classList).toContain('n-popover--raw')
    expect(popover.style.getPropertyValue('--n-box-shadow')).toBe('none')
    if (selectableDay === null) {
      throw new Error('the calendar did not render a selectable day')
    }
    selectableDay.click()
    await nextTick()
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toMatch(/^\d{4}-\d{2}-\d{2}$/u)
    expect(wrapper.get('[data-testid="filter-date-picker-trigger"]').classes()).not.toContain(
      'filter-date-picker__trigger--open',
    )
    wrapper.unmount()
  })

  it('clears a selected date from the installed popover', async () => {
    const wrapper = mount(FilterDatePicker, {
      attachTo: document.body,
      props: {
        ariaLabel: '选择首次发现日期',
        modelValue: '2026-10-02',
      },
    })

    await wrapper.get('[data-testid="filter-date-picker-trigger"]').trigger('click')
    await nextTick()
    await flushPromises()
    const clearButtons = Array.from(
      document.body.querySelectorAll<HTMLButtonElement>('[data-testid="filter-date-picker-clear"]'),
    )
    const clearButton = clearButtons[clearButtons.length - 1]
    if (clearButton === undefined) {
      throw new Error('the calendar clear action did not render')
    }
    clearButton.click()
    await nextTick()

    expect(wrapper.emitted('update:modelValue')).toEqual([['']])
    expect(wrapper.get('[data-testid="filter-date-picker-trigger"]').classes()).not.toContain(
      'filter-date-picker__trigger--open',
    )
    wrapper.unmount()
  })

  it('keeps the month and year selector opaque above the date grid', async () => {
    Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
      configurable: true,
      value: () => undefined,
      writable: true,
    })
    vi.stubGlobal(
      'matchMedia',
      vi.fn(() => ({
        addEventListener: () => undefined,
        addListener: () => undefined,
        dispatchEvent: () => false,
        matches: false,
        media: '',
        onchange: null,
        removeEventListener: () => undefined,
        removeListener: () => undefined,
      })),
    )
    const wrapper = mount(FilterDatePicker, {
      attachTo: document.body,
      props: {
        ariaLabel: '选择首次发现日期',
        modelValue: '',
      },
    })

    try {
      await wrapper.get('[data-testid="filter-date-picker-trigger"]').trigger('click')
      await nextTick()
      await flushPromises()
      const monthYearHeading = document.body.querySelector<HTMLElement>('.n-date-panel-month__text')
      if (monthYearHeading === null) {
        throw new Error('the calendar month and year heading did not render')
      }
      monthYearHeading.click()
      await nextTick()
      await flushPromises()

      const quickSelector = document.body.querySelector<HTMLElement>('.n-date-panel--month')
      if (quickSelector === null) {
        throw new Error('the calendar month and year selector did not render')
      }
      const source = readFileSync(
        resolve(process.cwd(), 'src/components/FilterDatePicker.vue'),
        'utf8',
      )
      const basePanelRule = source.match(
        /:global\(\.filter-date-picker__calendar > \.n-date-panel\) \{(?<declarations>[\s\S]*?)\n\}/u,
      )?.groups?.declarations
      const quickSelectorRule = source.match(
        /:global\(\.filter-date-picker__popover \.n-date-panel--month\) \{(?<declarations>[\s\S]*?)\n\}/u,
      )?.groups?.declarations

      expect(basePanelRule).toMatch(/background:\s*transparent/u)
      expect(quickSelectorRule).toMatch(/background:\s*#ffffff/u)
      expect(quickSelectorRule).toMatch(/position:\s*relative/u)
      expect(quickSelectorRule).toMatch(/z-index:\s*2/u)
    } finally {
      wrapper.unmount()
    }
  })
})
