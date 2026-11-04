<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import type { FilterOption, TechnologyKeywordFilterOption } from '@/api/jd-api'

const props = withDefaults(
  defineProps<{
    modelValue: string[]
    options: TechnologyKeywordFilterOption[]
    placeholder?: string
  }>(),
  { placeholder: '搜索或选择技术关键词' },
)

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const root = ref<HTMLElement | null>(null)
const searchInput = ref<HTMLInputElement | null>(null)
const isOpen = ref(false)
const query = ref('')
const activeCategoryValue = ref('all')
const uncategorizedTechnology = { value: 'other', label: '其他技术' }

function categoryOf(option: TechnologyKeywordFilterOption): FilterOption {
  return option.category ?? uncategorizedTechnology
}

const categories = computed<Array<FilterOption & { count: number }>>(() => {
  const counts = new Map<string, FilterOption & { count: number }>()
  for (const option of props.options) {
    const category = categoryOf(option)
    const existing = counts.get(category.value)
    if (existing) {
      existing.count += 1
      continue
    }
    counts.set(category.value, { ...category, count: 1 })
  }
  return [{ value: 'all', label: '全部技术词', count: props.options.length }, ...counts.values()]
})

const activeCategory = computed(() =>
  categories.value.find((category) => category.value === activeCategoryValue.value),
)

const normalizedQuery = computed(() => query.value.trim().toLocaleLowerCase())
const isSearching = computed(() => normalizedQuery.value.length > 0)
const visibleOptions = computed(() => {
  if (isSearching.value) {
    return props.options.filter((option) =>
      option.label.toLocaleLowerCase().includes(normalizedQuery.value),
    )
  }
  if (activeCategoryValue.value === 'all') return props.options
  return props.options.filter((option) => categoryOf(option).value === activeCategoryValue.value)
})

const selectedOptions = computed(() => {
  const labelsByValue = new Map(props.options.map((option) => [option.value, option]))
  return props.modelValue.flatMap((value) => {
    const option = labelsByValue.get(value)
    return option === undefined ? [] : [option]
  })
})

const triggerText = computed(() => {
  if (selectedOptions.value.length === 0) return props.placeholder
  const labels = selectedOptions.value.slice(0, 2).map((option) => option.label)
  const remaining = selectedOptions.value.length - labels.length
  return remaining > 0 ? `${labels.join('、')} +${remaining}` : labels.join('、')
})

function isSelected(value: string): boolean {
  return props.modelValue.includes(value)
}

function toggleSelection(value: string): void {
  if (isSelected(value)) {
    emit(
      'update:modelValue',
      props.modelValue.filter((selectedValue) => selectedValue !== value),
    )
    return
  }
  emit('update:modelValue', [...props.modelValue, value])
}

function clearSelection(): void {
  emit('update:modelValue', [])
}

function selectCategory(value: string): void {
  activeCategoryValue.value = value
}

function openMenu(): void {
  isOpen.value = true
}

function closeMenu(): void {
  isOpen.value = false
  query.value = ''
}

function toggleMenu(): void {
  if (isOpen.value) {
    closeMenu()
    return
  }
  openMenu()
}

function onTriggerKeydown(event: KeyboardEvent): void {
  if (event.key === 'ArrowDown' || event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    openMenu()
  }
}

function onDocumentPointerDown(event: PointerEvent): void {
  if (!isOpen.value || root.value === null || !(event.target instanceof Node)) return
  if (!root.value.contains(event.target)) closeMenu()
}

watch(isOpen, async (open) => {
  if (!open) return
  await nextTick()
  searchInput.value?.focus()
})

watch(categories, (availableCategories) => {
  if (!availableCategories.some((category) => category.value === activeCategoryValue.value)) {
    activeCategoryValue.value = 'all'
  }
})

onMounted(() => document.addEventListener('pointerdown', onDocumentPointerDown))
onBeforeUnmount(() => document.removeEventListener('pointerdown', onDocumentPointerDown))
</script>

<template>
  <div ref="root" class="technology-keyword-select">
    <button
      type="button"
      class="technology-keyword-select__trigger"
      :class="{ 'technology-keyword-select__trigger--open': isOpen }"
      aria-haspopup="dialog"
      :aria-expanded="isOpen"
      data-testid="technology-keyword-trigger"
      @click="toggleMenu"
      @keydown="onTriggerKeydown"
    >
      <span :class="{ 'technology-keyword-select__placeholder': selectedOptions.length === 0 }">{{
        triggerText
      }}</span>
      <span class="technology-keyword-select__chevron" aria-hidden="true">⌄</span>
    </button>

    <section
      v-if="isOpen"
      class="technology-keyword-select__menu"
      role="dialog"
      aria-label="选择技术关键词"
    >
      <label class="technology-keyword-select__search">
        <span class="technology-keyword-select__search-icon" aria-hidden="true">⌕</span>
        <span class="visually-hidden">搜索全部技术关键词</span>
        <input
          ref="searchInput"
          v-model="query"
          type="search"
          autocomplete="off"
          placeholder="搜索技术关键词"
          data-testid="technology-keyword-search"
        />
      </label>

      <div
        v-if="selectedOptions.length"
        class="technology-keyword-select__selected"
        data-testid="technology-keyword-selected"
      >
        <span>已选择</span>
        <div class="technology-keyword-select__chips">
          <button
            v-for="option in selectedOptions"
            :key="option.value"
            type="button"
            class="technology-keyword-select__chip"
            :aria-label="`移除 ${option.label}`"
            @click="toggleSelection(option.value)"
          >
            {{ option.label }} <span aria-hidden="true">×</span>
          </button>
        </div>
        <button type="button" class="technology-keyword-select__clear" @click="clearSelection">
          清空
        </button>
      </div>

      <div class="technology-keyword-select__browser">
        <nav class="technology-keyword-select__categories" aria-label="技术关键词分类">
          <button
            v-for="category in categories"
            :key="category.value"
            type="button"
            class="technology-keyword-select__category"
            :class="{
              'technology-keyword-select__category--active':
                !isSearching && category.value === activeCategoryValue,
            }"
            :aria-current="
              !isSearching && category.value === activeCategoryValue ? 'true' : undefined
            "
            :data-testid="`technology-category-${category.value}`"
            @click="selectCategory(category.value)"
          >
            <span>{{ category.label }}</span
            ><small>{{ category.count }}</small>
          </button>
        </nav>

        <section class="technology-keyword-select__term-panel" aria-live="polite">
          <header>
            <strong>{{ isSearching ? '搜索结果' : activeCategory?.label }}</strong>
            <span>{{ visibleOptions.length }} 项</span>
          </header>
          <div class="technology-keyword-select__terms" data-testid="technology-keyword-terms">
            <button
              v-for="option in visibleOptions"
              :key="option.value"
              type="button"
              class="technology-keyword-select__term"
              :class="{ 'technology-keyword-select__term--selected': isSelected(option.value) }"
              :aria-pressed="isSelected(option.value)"
              :data-testid="`technology-keyword-${option.value}`"
              @click="toggleSelection(option.value)"
            >
              <span class="technology-keyword-select__checkbox" aria-hidden="true">{{
                isSelected(option.value) ? '✓' : ''
              }}</span>
              <span>{{ option.label }}</span>
              <small v-if="isSearching">{{ categoryOf(option).label }}</small>
            </button>
            <p v-if="visibleOptions.length === 0" class="technology-keyword-select__empty">
              未找到匹配的技术关键词。
            </p>
          </div>
        </section>
      </div>
    </section>
  </div>
</template>

<style scoped>
.technology-keyword-select {
  position: relative;
}

.technology-keyword-select__trigger {
  align-items: center;
  background: #fff;
  border: 1px solid #d1dbe1;
  border-radius: 10px;
  color: #2e373c;
  cursor: pointer;
  display: flex;
  font: inherit;
  gap: 10px;
  min-height: 40px;
  padding: 0 13px;
  text-align: left;
  transition:
    border-color 140ms ease,
    box-shadow 140ms ease;
  width: 100%;
}

.technology-keyword-select__trigger:hover,
.technology-keyword-select__trigger:focus-visible,
.technology-keyword-select__trigger--open {
  border-color: #007bab;
  box-shadow: 0 0 0 2px rgba(0, 123, 171, 0.2);
  outline: 0;
}

.technology-keyword-select__placeholder {
  color: #868d92;
}

.technology-keyword-select__chevron {
  color: #849197;
  font-size: 1.15rem;
  line-height: 1;
  margin-left: auto;
}

.technology-keyword-select__menu {
  background: #fff;
  border: 1px solid #e3ebf0;
  border-radius: 14px;
  box-shadow:
    0 9px 28px 0 rgba(11, 37, 50, 0.1),
    0 12px 48px 16px rgba(11, 37, 50, 0.04);
  left: auto;
  margin-top: 7px;
  min-width: 0;
  overflow: hidden;
  position: absolute;
  right: 0;
  top: 100%;
  width: min(640px, 100%);
  z-index: 20;
}

.technology-keyword-select__search {
  align-items: center;
  background: #f0f8fd;
  border-radius: 10px;
  display: flex;
  gap: 8px;
  margin: 12px 12px 8px;
  min-height: 40px;
  padding: 0 12px;
}

.technology-keyword-select__search-icon {
  color: #406980;
  font-size: 1.15rem;
}

.technology-keyword-select__search input {
  background: transparent;
  border: 0;
  color: #121c22;
  font: inherit;
  min-width: 0;
  outline: 0;
  width: 100%;
}

.technology-keyword-select__search input::placeholder {
  color: #868d92;
}

.technology-keyword-select__selected {
  align-items: flex-start;
  color: #626a6f;
  display: flex;
  font-size: 0.78rem;
  gap: 8px;
  line-height: 26px;
  padding: 0 14px 10px;
}

.technology-keyword-select__chips {
  display: flex;
  flex: 1;
  flex-wrap: wrap;
  gap: 5px;
}

.technology-keyword-select__chip {
  background: #e5f5f7;
  border: 0;
  border-radius: 7px;
  color: #006b70;
  cursor: pointer;
  font: inherit;
  font-weight: 650;
  line-height: 22px;
  padding: 0 7px;
}

.technology-keyword-select__clear {
  background: transparent;
  border: 0;
  color: #007bab;
  cursor: pointer;
  font: inherit;
  font-size: 0.78rem;
  font-weight: 650;
  padding: 0;
}

.technology-keyword-select__browser {
  border-top: 1px solid #e3ebf0;
  display: grid;
  grid-template-columns: minmax(164px, 43%) minmax(0, 1fr);
  min-height: 332px;
}

.technology-keyword-select__categories {
  background: #fbfdfe;
  border-right: 1px solid #e3ebf0;
  max-height: 372px;
  overscroll-behavior: contain;
  overflow-y: auto;
  padding: 8px;
}

.technology-keyword-select__category {
  align-items: center;
  background: transparent;
  border: 0;
  border-radius: 8px;
  color: #48555a;
  cursor: pointer;
  display: flex;
  font: inherit;
  font-size: 0.8rem;
  justify-content: space-between;
  min-height: 34px;
  padding: 0 8px;
  text-align: left;
  white-space: nowrap;
  width: 100%;
}

.technology-keyword-select__category:hover,
.technology-keyword-select__category:focus-visible {
  background: #eaf1f6;
  outline: 0;
}

.technology-keyword-select__category--active {
  background: #e3f4fa;
  color: #00658e;
  font-weight: 700;
}

.technology-keyword-select__category small {
  color: #7d8b90;
  font-size: 0.73rem;
  font-variant-numeric: tabular-nums;
}

.technology-keyword-select__category--active small {
  color: #007bab;
}

.technology-keyword-select__term-panel {
  max-height: 372px;
  min-width: 0;
  padding: 10px;
}

.technology-keyword-select__term-panel header {
  align-items: center;
  color: #406980;
  display: flex;
  font-size: 0.8rem;
  justify-content: space-between;
  padding: 0 6px 8px;
}

.technology-keyword-select__term-panel header span {
  color: #868d92;
  font-size: 0.75rem;
}

.technology-keyword-select__terms {
  max-height: 326px;
  overscroll-behavior: contain;
  overflow-y: auto;
}

.technology-keyword-select__term {
  align-items: center;
  background: transparent;
  border: 0;
  border-radius: 8px;
  color: #202c32;
  cursor: pointer;
  display: flex;
  font: inherit;
  font-size: 0.88rem;
  gap: 8px;
  min-height: 34px;
  padding: 0 8px;
  text-align: left;
  width: 100%;
}

.technology-keyword-select__term:hover,
.technology-keyword-select__term:focus-visible {
  background: #eaf1f6;
  outline: 0;
}

.technology-keyword-select__term--selected {
  background: #eaf6fa;
  color: #00658e;
  font-weight: 650;
}

.technology-keyword-select__checkbox {
  align-items: center;
  border: 1px solid #b9c9cf;
  border-radius: 5px;
  color: #fff;
  display: inline-flex;
  flex: 0 0 auto;
  font-size: 0.72rem;
  font-weight: 800;
  height: 16px;
  justify-content: center;
  width: 16px;
}

.technology-keyword-select__term--selected .technology-keyword-select__checkbox {
  background: #007bab;
  border-color: #007bab;
}

.technology-keyword-select__term small {
  color: #72818a;
  font-size: 0.73rem;
  margin-left: auto;
}

.technology-keyword-select__empty {
  color: #72818a;
  font-size: 0.82rem;
  margin: 12px 8px;
}

.visually-hidden {
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
  height: 1px;
  overflow: hidden;
  position: absolute;
  white-space: nowrap;
  width: 1px;
}

@media (max-width: 620px) {
  .technology-keyword-select__menu {
    min-width: min(100%, calc(100vw - 32px));
  }

  .technology-keyword-select__categories {
    padding: 6px;
  }

  .technology-keyword-select__category {
    font-size: 0.74rem;
    padding: 0 6px;
  }
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__trigger) {
  background: #19323c;
  border-color: #31505c;
  color: #ecf6f7;
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__trigger:hover),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__trigger:focus-visible),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__trigger--open) {
  border-color: #39b5a3;
  box-shadow: 0 0 0 2px rgba(57, 181, 163, 0.24);
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__placeholder),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__chevron),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__selected),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__term-panel header span),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__term small),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__empty) {
  color: #9bb8c0;
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__menu) {
  background: #17303a;
  border-color: #31505c;
  box-shadow: 0 18px 48px -18px rgba(0, 0, 0, 0.7);
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__search),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__categories) {
  background: #142a33;
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__browser),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__categories) {
  border-color: #294752;
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__search-icon),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__term-panel header) {
  color: #b5d7df;
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__search input),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__category),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__term) {
  color: #dceff1;
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__chip),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__category--active),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__term--selected) {
  background: #1d4a55;
  color: #a9e5d7;
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__category:hover),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__category:focus-visible),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__term:hover),
:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__term:focus-visible) {
  background: #203f4a;
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__clear) {
  color: #73c9dd;
}

:global(.app-shell[data-color-scheme='dark'] .technology-keyword-select__checkbox) {
  border-color: #5c8390;
}

:global(
  .app-shell[data-color-scheme='dark']
    .technology-keyword-select__term--selected
    .technology-keyword-select__checkbox
) {
  background: #39b5a3;
  border-color: #39b5a3;
}
</style>
